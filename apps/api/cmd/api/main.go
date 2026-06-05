// Package main es el entrypoint del API HTTP de BattOS.
//
// Responsabilidades:
//  1. Cargar config (viper).
//  2. Inicializar logger estructurado (slog).
//  3. Lanzar sampler de métricas en background.
//  4. Construir el router chi con sus dependencias.
//  5. Levantar el HTTP server.
//  6. Manejar shutdown graceful en SIGINT/SIGTERM.
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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/handlers"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/server"
	"github.com/nicotion/battos/apps/api/internal/store"
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

	// --- 4. Postgres pool (opcional — el API arranca aunque falle, en estado degraded) ---
	var pgPool *pgxpool.Pool
	var pgInitErr error
	if cfg.DatabaseURL != "" {
		pgPool, err = store.OpenPool(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Warn("postgres pool init failed (subsystem 'database' será DOWN)", "error", err)
			pgInitErr = err
			pgPool = nil
		} else {
			logger.Info("postgres pool ready")
			defer pgPool.Close()
		}
	} else {
		logger.Warn("DATABASE_URL no seteado — subsistema 'database' quedará UNKNOWN")
	}

	// --- 5. Memory Core (siempre disponible, embebido) ---
	memCore, err := memory.Open(cfg.Memory.DBPath)
	if err != nil {
		return fmt.Errorf("memory core: %w", err)
	}
	defer memCore.Close()
	logger.Info("memory core ready", "db_path", cfg.Memory.DBPath)

	// Closures de healthcheck que el SystemHandler usa para reportar /status.
	var pingDB func(context.Context) error
	if pgPool != nil {
		pingDB = func(ctx context.Context) error { return pgPool.Ping(ctx) }
	} else if pgInitErr != nil {
		pingDB = func(context.Context) error { return pgInitErr }
	}
	pingMem := memCore.Ping

	// --- 6. Router ---
	systemHandler := handlers.NewSystemHandler(sampler, pingDB, pingMem)
	memoryHandler := handlers.NewMemoryHandler(memCore)
	var workHandler *handlers.WorkHandler
	var knowledgeHandler *handlers.KnowledgeHandler
	var registriesHandler *handlers.RegistriesHandler
	var runtimeHandler *handlers.RuntimeHandler
	var runHandler *handlers.RunHandler
	var repositoriesHandler *handlers.RepositoriesHandler
	var novaCoreHandler *handlers.NovaCoreHandler
	var usageHandler *handlers.UsageHandler
	if pgPool != nil {
		queries := store.New(pgPool)
		workHandler = handlers.NewWorkHandler(queries)
		knowledgeHandler = handlers.NewKnowledgeHandler(queries, cfg.Knowledge.ArtifactsDir)
		registriesHandler = handlers.NewRegistriesHandler(queries)
		runtimeHandler = handlers.NewRuntimeHandler(queries)
		runHandler = handlers.NewRunHandler(queries, memCore)
		repositoriesHandler = handlers.NewRepositoriesHandler(queries, cfg.Execution.RepositoriesDir)
		novaCoreHandler = handlers.NewNovaCoreHandler(queries, memCore, cfg)
		usageHandler = handlers.NewUsageHandler(queries)
	}
	router := server.NewRouter(server.Deps{
		Config:       cfg,
		Logger:       logger,
		System:       systemHandler,
		Memory:       memoryHandler,
		Work:         workHandler,
		Knowledge:    knowledgeHandler,
		Registries:   registriesHandler,
		Runtime:      runtimeHandler,
		Runs:         runHandler,
		Repositories: repositoriesHandler,
		NovaCore:     novaCoreHandler,
		Usage:        usageHandler,
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
