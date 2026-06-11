package worker

import (
	"context"
	"fmt"
	"log/slog"
)

// ConnectedSandbox is a stub for execution_mode="connected" (forward to an
// always-on service such as Hermes or OpenClaw). The actual implementation —
// HTTP forwarding, authentication, result polling — is deferred to a later
// phase. For now it fails explicitly so callers receive a clear error.
type ConnectedSandbox struct{}

func (ConnectedSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	slog.WarnContext(ctx, "connected sandbox stub invoked; not yet implemented",
		"runtime_id", plan.RuntimeID,
		"command", plan.Command,
	)
	if err := log("system", "execution_mode 'connected': not implemented yet (stub)"); err != nil {
		return Result{}, err
	}
	return Result{}, fmt.Errorf("execution_mode 'connected' not implemented yet")
}
