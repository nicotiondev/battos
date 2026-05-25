// Package main es el entrypoint del API HTTP de BattOS.
//
// Responsabilidades:
//   1. Cargar config (viper).
//   2. Inicializar logger estructurado (slog).
//   3. Lanzar sampler de métricas en background.
//   4. Construir el router chi con sus dependencias.
//   5. Levantar el HTTP server.
//   6. Manejar shutdown graceful en SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/handlers"
	"github.com/nicotion/battos/apps/api/internal/server"
	"github.com/nicotion/battos/apps/api/internal/sysmetrics"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	// --- 1. Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// --- 2. Logger ---
	logger := newLogger(cfg.Logs.Level, cfg.Logs.Format)
	slog.SetDefault(logger)
	logger.Info("battos.api starting",
		"version", handlers.Version,
		"commit", handlers.Commit,
	)

	// Context que se cancela en SIGINT/SIGTERM — propaga a sampler y server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- 3. Sampler de métricas en background ---
	sampleInterval := time.Duration(cfg.Sysmetrics.SampleIntervalS) * time.Second
	if sampleInterval == 0 {
		sampleInterval = time.Second
	}
	sampler := sysmetrics.New(sampleInterval)
	sampler.Start(ctx)
	logger.Info("sysmetrics.sampler started", "interval_s", int(sampleInterval.Seconds()))

	// --- 4. Router ---
	systemHandler := handlers.NewSystemHandler(sampler)
	router := server.NewRouter(server.Deps{
		Config: cfg,
		Logger: logger,
		System: systemHandler,
	})

	// --- 5. HTTP server ---
	addr := fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0, // 0 para no cortar SSE en Fase 5
		IdleTimeout:       120 * time.Second,
	}

	// Lanzar server en goroutine — para poder esperar señales en main.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http.server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// --- 6. Esperar shutdown o crash ---
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	}

	// Graceful shutdown: 10s para que terminen requests en vuelo.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("battos.api stopped cleanly")
	return nil
}

// newLogger construye un slog.Logger según level/format del config.
//
//	format=json   → JSONHandler  (production)
//	format=text   → TextHandler  (development, más legible)
func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
