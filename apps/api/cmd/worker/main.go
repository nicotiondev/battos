package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.OpenDB(ctx, cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()

	memCore, err := memory.OpenWithDB(db)
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
	w := runworker.New(store.New(db), sandbox, runworker.ApprovedAdapters(runworker.AdapterOptions{
		HostSessionEnabled:   cfg.Execution.HostSessionEnabled,
		CodexCredentialsDir:  cfg.Execution.CodexCredentialsDir,
		ClaudeCredentialsDir: cfg.Execution.ClaudeCredentialsDir,
	}))
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

func parseRunID(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("run-id vacío")
	}
	return value, nil
}
