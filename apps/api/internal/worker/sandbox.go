package worker

import (
	"context"
	"fmt"
	"strings"
)

type DryRunSandbox struct{}

func (DryRunSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}
	if err := log("system", "sandbox dry-run: no host command executed"); err != nil {
		return Result{}, err
	}
	if err := log("system", fmt.Sprintf("would run %s %s", plan.Command, strings.Join(plan.Args, " "))); err != nil {
		return Result{}, err
	}
	if plan.NetworkEnabled {
		if err := log("system", "network: enabled by approval"); err != nil {
			return Result{}, err
		}
	} else if err := log("system", "network: disabled"); err != nil {
		return Result{}, err
	}
	return Result{Summary: "dry-run completed; execution sandbox boundary is ready"}, nil
}
