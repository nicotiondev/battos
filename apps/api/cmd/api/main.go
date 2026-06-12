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
	"strings"
	"syscall"
	"time"

	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/handlers"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/server"
	"github.com/nicotion/battos/apps/api/internal/store"
	"github.com/nicotion/battos/apps/api/internal/sysmetrics"
	runworker "github.com/nicotion/battos/apps/api/internal/worker"
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

	// --- 4. SQLite unificado ---
	db, err := store.OpenDB(ctx, cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()
	logger.Info("sqlite database ready", "path", cfg.Database.Path)

	// --- 5. Memory Core sobre la misma base SQLite ---
	memCore, err := memory.OpenWithDB(db)
	if err != nil {
		return fmt.Errorf("memory core: %w", err)
	}
	defer memCore.Close()
	logger.Info("memory core ready", "db_path", cfg.Database.Path)

	// --- 5b. Selección del MemoryProvider activo ---
	var memProvider memory.MemoryProvider
	if cfg.Memory.Provider == "engram" {
		engramURL := cfg.Memory.EngramURL
		if engramURL == "" {
			engramURL = "http://localhost:7437"
		}
		memProvider = memory.NewEngramProvider(engramURL, memCore)
		logger.Info("memory provider: engram", "url", engramURL)
	} else {
		memProvider = memCore
		logger.Info("memory provider: builtin")
	}

	// Closures de healthcheck que el SystemHandler usa para reportar /status.
	pingDB := db.PingContext
	pingMem := memCore.Ping

	// --- 6. Router ---
	systemHandler := handlers.NewSystemHandler(sampler, pingDB, pingMem)
	memoryHandler := handlers.NewMemoryHandler(memProvider)
	queries := store.New(db)
	workHandler := handlers.NewWorkHandler(queries)
	knowledgeHandler := handlers.NewKnowledgeHandler(queries, cfg.Knowledge.ArtifactsDir)
	registriesHandler := handlers.NewRegistriesHandler(queries)
	runtimeHandler := handlers.NewRuntimeHandler(queries)
	runHandler := handlers.NewRunHandler(queries, memProvider)
	messagesHandler := handlers.NewMessagesHandler(queries)
	repositoriesHandler := handlers.NewRepositoriesHandler(queries, cfg.Execution.RepositoriesDir)
	novaCoreHandler := handlers.NewNovaCoreHandler(queries, memProvider, cfg)
	usageHandler := handlers.NewUsageHandler(queries)
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
		Messages:     messagesHandler,
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

	// --- 7. Worker integrado (cuando worker_enabled: true en config) ---
	// Corre el mismo pool de workers dentro del proceso de la API, de modo que
	// `battos serve` (o el binario compilado) arranque todo en un solo comando.
	if cfg.Execution.WorkerEnabled {
		connectedIDs := make([]string, 0, len(cfg.Execution.ConnectedRuntimes))
		for id := range cfg.Execution.ConnectedRuntimes {
			if s := strings.TrimSpace(id); s != "" {
				connectedIDs = append(connectedIDs, s)
			}
		}
		var selector func(string) runworker.Sandbox
		if cfg.Execution.SandboxMode == "dry_run" {
			selector = func(_ string) runworker.Sandbox { return runworker.DryRunSandbox{} }
		} else {
			dockerSandbox := runworker.DockerSandbox{
				Image:           cfg.Execution.DockerImage,
				WorkspacesDir:   cfg.Execution.WorkspacesDir,
				EgressNetwork:   cfg.Execution.EgressNetwork,
				EgressProxyAddr: cfg.Execution.EgressProxyAddr,
			}
			connectedRuntimes := make(map[string]runworker.ConnectedRuntimeConfig, len(cfg.Execution.ConnectedRuntimes))
			for id, rc := range cfg.Execution.ConnectedRuntimes {
				connectedRuntimes[id] = runworker.ConnectedRuntimeConfig{
					Kind: rc.Kind, Endpoint: rc.Endpoint,
					Command: rc.Command, Args: rc.Args,
				}
			}
			connSandbox := runworker.ConnectedSandbox{
				Runtimes: connectedRuntimes, WorkspacesDir: cfg.Execution.WorkspacesDir,
			}
			selector = func(mode string) runworker.Sandbox {
				switch mode {
				case "direct":
					return runworker.DirectSandbox{WorkspacesDir: cfg.Execution.WorkspacesDir}
				case "connected":
					return connSandbox
				default:
					return dockerSandbox
				}
			}
		}
		w := runworker.NewWithSelector(store.New(db), selector, runworker.ApprovedAdapters(runworker.AdapterOptions{
			HostSessionEnabled:   cfg.Execution.HostSessionEnabled,
			CodexCredentialsDir:  cfg.Execution.CodexCredentialsDir,
			ClaudeCredentialsDir: cfg.Execution.ClaudeCredentialsDir,
			ConnectedRuntimeIDs:  connectedIDs,
		}))
		w.ArtifactsDir = cfg.Knowledge.ArtifactsDir
		w.WorkspacesDir = cfg.Execution.WorkspacesDir
		w.RepositoriesDir = cfg.Execution.RepositoriesDir
		memCtx := runworker.MemoryCoreContextProvider{Core: memProvider}
		w.Memory = memCtx
		w.MemoryPromote = memCtx
		pollInterval := time.Duration(cfg.Execution.PollIntervalS) * time.Second
		concurrency := cfg.Execution.WorkerConcurrency
		if concurrency <= 0 {
			concurrency = 1
		}
		go func() {
			logger.Info("worker.pool started", "concurrency", concurrency, "sandbox", cfg.Execution.SandboxMode)
			if err := w.RunPool(ctx, concurrency, pollInterval); err != nil {
				logger.Error("worker.pool stopped", "err", err)
			}
		}()
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
