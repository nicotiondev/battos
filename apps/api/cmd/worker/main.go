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
	workers := flag.Int("workers", 0, "runs en paralelo cuando -once=false (0 = usar config worker_concurrency)")
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

	// selector picks the right sandbox per run execution_mode.
	// When cfg.Execution.SandboxMode == "dry_run" it acts as a master off-switch:
	// every run uses DryRunSandbox regardless of the requested mode.
	var selector func(executionMode string) runworker.Sandbox
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
				Kind:     rc.Kind,
				Endpoint: rc.Endpoint,
				Command:  rc.Command,
				Args:     rc.Args,
			}
		}
		connectedSandbox := runworker.ConnectedSandbox{
			Runtimes:      connectedRuntimes,
			WorkspacesDir: cfg.Execution.WorkspacesDir,
		}
		selector = func(executionMode string) runworker.Sandbox {
			switch executionMode {
			case "direct":
				return runworker.DirectSandbox{WorkspacesDir: cfg.Execution.WorkspacesDir}
			case "connected":
				return connectedSandbox
			default: // "sandbox" or any unrecognised value
				return dockerSandbox
			}
		}
	}
	connectedIDs := make([]string, 0, len(cfg.Execution.ConnectedRuntimes))
	for id := range cfg.Execution.ConnectedRuntimes {
		connectedIDs = append(connectedIDs, id)
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
	memProvider := runworker.MemoryCoreContextProvider{Core: memCore}
	w.Memory = memProvider
	w.MemoryPromote = memProvider
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
	concurrency := *workers
	if concurrency <= 0 {
		concurrency = cfg.Execution.WorkerConcurrency
	}
	fmt.Printf("worker pool started; sandbox=%s poll=%s concurrency=%d\n", cfg.Execution.SandboxMode, pollInterval, concurrency)
	return w.RunPool(ctx, concurrency, pollInterval)
}

func parseRunID(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("run-id vacío")
	}
	return value, nil
}
