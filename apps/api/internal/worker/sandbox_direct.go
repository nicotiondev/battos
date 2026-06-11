package worker

import (
	"context"
	"fmt"
	"log/slog"
)

// DirectSandbox is a stub for execution_mode="direct" (host exec, no container).
// The actual implementation — spawning the CLI directly on the host process tree,
// with per-run credential injection and a process-level timeout — is deferred to
// a later phase. For now it fails explicitly so that any run proposing direct
// execution receives a clear, intentional error rather than a silent no-op.
type DirectSandbox struct{}

func (DirectSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	slog.WarnContext(ctx, "direct sandbox stub invoked; not yet implemented",
		"runtime_id", plan.RuntimeID,
		"command", plan.Command,
	)
	if err := log("system", "execution_mode 'direct': not implemented yet (stub)"); err != nil {
		return Result{}, err
	}
	return Result{}, fmt.Errorf("execution_mode 'direct' not implemented yet")
}
