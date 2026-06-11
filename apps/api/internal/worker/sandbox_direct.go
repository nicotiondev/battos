package worker

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// DirectSandbox executes the agent CLI directly on the host process tree,
// without any Docker container. This is the "direct" execution tier: trusted,
// fast, and warm — the agent uses its own host credentials and environment.
//
// Security tradeoff: direct mode has NO egress control. The agent process
// inherits the host network stack and can reach any endpoint. This is the
// conscious, intentional tradeoff of the trusted tier. Operators must only
// grant "direct" execution mode to runs/agents they fully trust (gated by the
// per-run execution_mode + its approval).
type DirectSandbox struct {
	// WorkspacesDir is the root directory for ephemeral run workspaces.
	// Defaults to "data/runs/workspaces" when empty.
	WorkspacesDir string
}

func (s DirectSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	if strings.TrimSpace(plan.Command) == "" {
		return Result{}, fmt.Errorf("direct sandbox: command is required")
	}

	slog.InfoContext(ctx, "direct sandbox: executing agent on host",
		"runtime_id", plan.RuntimeID,
		"command", plan.Command,
	)

	// Informational system log lines — visible in the run log stream.
	if err := log("system", "sandbox direct: running on host (no container)"); err != nil {
		return Result{}, err
	}
	// Direct mode inherits the host network: no egress proxy, no allowlist.
	// This is expected and intentional for the trusted tier (see struct comment above).
	if err := log("system", "network: host (unfiltered)"); err != nil {
		return Result{}, err
	}

	// Mounts are a Docker concept — irrelevant in direct mode. The agent already
	// has its own host credentials natively; no volume injection is needed.
	if len(plan.Mounts) > 0 {
		if err := log("system", "direct sandbox: mounts ignored (not applicable on host)"); err != nil {
			return Result{}, err
		}
	}

	// --- Workspace ---
	root := defaultString(s.WorkspacesDir, "data/runs/workspaces")
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("direct sandbox: resolve workspaces root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("direct sandbox: create workspaces root: %w", err)
	}

	workspace := plan.WorkDir
	isTempWorkspace := false
	if strings.TrimSpace(workspace) == "" {
		workspace, err = os.MkdirTemp(absRoot, "run-*")
		if err != nil {
			return Result{}, fmt.Errorf("direct sandbox: create run workspace: %w", err)
		}
		isTempWorkspace = true
	}
	if isTempWorkspace {
		defer os.RemoveAll(workspace)
	}
	if err := os.Chmod(workspace, 0o755); err != nil {
		return Result{}, fmt.Errorf("direct sandbox: make workspace accessible: %w", err)
	}

	// --- Prompt file ---
	promptPath := filepath.Join(workspace, "BATTOS_PROMPT.md")
	if strings.TrimSpace(plan.Prompt) != "" {
		if err := os.WriteFile(promptPath, []byte(plan.Prompt), 0o600); err != nil {
			return Result{}, fmt.Errorf("direct sandbox: write prompt file: %w", err)
		}
	}

	// --- Timeout context ---
	runCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	// --- Build command ---
	//nolint:gosec // plan.Command and plan.Args are validated by validatePlan before Execute is called.
	cmd := exec.CommandContext(runCtx, plan.Command, plan.Args...)
	cmd.Dir = workspace

	// Env: start from the full host environment so the agent has its own auth/login,
	// then inject BATTOS_PROMPT_FILE.
	// plan.EnvKeys names vars the adapter needs; since we pass os.Environ() already
	// those vars are present — the loop documents intent and mirrors DockerSandbox's -e pattern.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "BATTOS_PROMPT_FILE="+promptPath)
	for _, key := range plan.EnvKeys {
		// Vars are already in the inherited env; this ensures they appear last
		// so explicit overrides (if any) win. This matches DockerSandbox's -e KEY behaviour.
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+val)
		}
	}

	// --- Stream stdout + stderr line by line ---
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("direct sandbox: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("direct sandbox: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("direct sandbox: start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	scan := func(stream string, scanner *bufio.Scanner) {
		defer wg.Done()
		for scanner.Scan() {
			line := redactKnownSecrets(scanner.Text())
			_ = log(stream, line)
		}
	}

	stdoutScanner := bufio.NewScanner(stdoutPipe)
	stderrScanner := bufio.NewScanner(stderrPipe)
	buf := make([]byte, 64*1024)
	stdoutScanner.Buffer(buf, 4*1024*1024)
	buf2 := make([]byte, 64*1024)
	stderrScanner.Buffer(buf2, 4*1024*1024)

	go scan("stdout", stdoutScanner)
	go scan("stderr", stderrScanner)

	cmdErr := cmd.Wait()
	wg.Wait()

	if cmdErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return Result{}, fmt.Errorf("direct sandbox timeout after %s", plan.Timeout)
		}
		return Result{}, fmt.Errorf("direct sandbox command failed: %w", cmdErr)
	}

	// --- Scan workspace for produced artifacts (mirrors DockerSandbox) ---
	var artifacts []ProducedArtifact
	errScan := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		if rel == "BATTOS_PROMPT.md" {
			return nil
		}
		rel = filepath.ToSlash(rel)
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		kind := detectArtifactKind(rel)
		artifacts = append(artifacts, ProducedArtifact{
			Name:    rel,
			Kind:    kind,
			Content: string(contentBytes),
		})
		return nil
	})
	if errScan != nil {
		_ = log("system", "direct sandbox: error scanning workspace artifacts: "+errScan.Error())
	}

	return Result{Summary: "direct sandbox completed", Artifacts: artifacts}, nil
}
