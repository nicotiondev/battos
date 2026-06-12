package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
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

	if err := log("system", "sandbox direct: running on host (no container)"); err != nil {
		return Result{}, err
	}
	// Direct mode inherits the host network: no egress proxy, no allowlist.
	// Intentional for the trusted tier (see struct comment).
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

	workspace, cleanup, promptPath, err := prepareWorkspace(s.WorkspacesDir, plan.WorkDir, plan.Prompt)
	if err != nil {
		return Result{}, fmt.Errorf("direct sandbox: %w", err)
	}
	defer cleanup()

	runCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	//nolint:gosec // plan.Command and plan.Args are validated by validatePlan before Execute is called.
	cmd := exec.CommandContext(runCtx, plan.Command, plan.Args...)
	cmd.Dir = workspace
	// Inherit the host environment so the agent has its own auth/login, then add
	// BATTOS_PROMPT_FILE. plan.EnvKeys are already present via os.Environ(); the
	// loop re-appends them last so explicit values win (mirrors DockerSandbox -e).
	cmd.Env = append(os.Environ(), "BATTOS_PROMPT_FILE="+promptPath)
	for _, key := range plan.EnvKeys {
		if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+val)
		}
	}

	if err := streamProcess(runCtx, cmd, plan.Timeout, log); err != nil {
		return Result{}, fmt.Errorf("direct sandbox %w", err)
	}

	return Result{Summary: "direct sandbox completed", Artifacts: collectArtifacts(workspace, log)}, nil
}
