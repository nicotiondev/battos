package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/store"
	runworker "github.com/nicotion/battos/apps/api/internal/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	once := flag.Bool("once", true, "procesa un run queued y termina")
	runID := flag.String("run-id", "", "procesa solo el run queued indicado")
	poll := flag.Duration("poll", 0, "intervalo de polling cuando -once=false")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL es obligatorio para el worker")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("postgres pool: %w", err)
	}
	defer pool.Close()

	memCore, err := memory.Open(cfg.Memory.DBPath)
	if err != nil {
		return fmt.Errorf("memory core: %w", err)
	}
	defer memCore.Close()

	sandbox := runworker.Sandbox(runworker.DryRunSandbox{})
	if cfg.Execution.SandboxMode == "docker" {
		sandbox = runworker.DockerSandbox{
			Image:         cfg.Execution.DockerImage,
			WorkspacesDir: cfg.Execution.WorkspacesDir,
		}
	}
	w := runworker.New(store.New(pool), sandbox, runworker.ApprovedDryRunAdapters())
	w.ArtifactsDir = cfg.Knowledge.ArtifactsDir
	w.WorkspacesDir = cfg.Execution.WorkspacesDir
	w.RepositoriesDir = cfg.Execution.RepositoriesDir
	w.Memory = runworker.MemoryCoreContextProvider{Core: memCore}
	if *once {
		fmt.Printf("worker once started; sandbox=%s\n", cfg.Execution.SandboxMode)
		var processed bool
		var err error
		if *runID != "" {
			id, parseErr := parseRunID(*runID)
			if parseErr != nil {
				return parseErr
			}
			processed, err = w.ProcessRunID(ctx, id)
		} else {
			processed, err = w.ProcessOne(ctx)
		}
		if err != nil {
			return err
		}
		if processed {
			fmt.Println("processed one run")
		} else {
			fmt.Println("no queued runs")
		}
		return nil
	}

	pollInterval := *poll
	if pollInterval <= 0 {
		pollInterval = time.Duration(cfg.Execution.PollIntervalS) * time.Second
	}
	fmt.Printf("worker loop started; sandbox=%s poll=%s\n", cfg.Execution.SandboxMode, pollInterval)
	return w.RunLoop(ctx, pollInterval)
}

func parseRunID(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("run-id invalido: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
