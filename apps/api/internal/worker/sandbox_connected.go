package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ConnectedRuntimeConfig describes how to reach one always-on service for the
// "connected" execution tier. Two kinds are supported:
//
//   - "local-cli": run a host command that talks to the service (e.g. `hermes -z`).
//     {{prompt}} / {{prompt_file}} placeholders in Args are substituted; the
//     prompt is also exposed via BATTOS_PROMPT_FILE. Output is streamed and the
//     workspace is scanned for artifacts, exactly like DirectSandbox.
//   - "http": POST a JSON body {prompt, runtime_id} to Endpoint and stream the
//     response back into the run log.
type ConnectedRuntimeConfig struct {
	Kind     string
	Endpoint string
	Command  string
	Args     []string
}

// ConnectedSandbox forwards a run to a configured always-on service (Hermes,
// OpenClaw, …) instead of containerising it. The service holds its own auth and
// session; BattOS relays the prompt and streams logs/artifacts back.
type ConnectedSandbox struct {
	// Runtimes maps a runtime adapter id (plan.RuntimeID) to its service config.
	Runtimes map[string]ConnectedRuntimeConfig
	// WorkspacesDir is the root for ephemeral workspaces (local-cli mode).
	WorkspacesDir string
	// HTTPClient is used for http mode; defaults to a 10-minute-timeout client.
	HTTPClient *http.Client
}

func (s ConnectedSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	cfg, ok := s.Runtimes[plan.RuntimeID]
	if !ok {
		_ = log("system", fmt.Sprintf("connected sandbox: no service configured for runtime %q", plan.RuntimeID))
		return Result{}, fmt.Errorf("connected sandbox: no service configured for runtime %q", plan.RuntimeID)
	}

	kind := strings.TrimSpace(cfg.Kind)
	if kind == "" {
		kind = "local-cli"
	}
	slog.InfoContext(ctx, "connected sandbox: forwarding to always-on service",
		"runtime_id", plan.RuntimeID, "kind", kind)
	if err := log("system", fmt.Sprintf("sandbox connected: forwarding to always-on service (%s)", kind)); err != nil {
		return Result{}, err
	}

	switch kind {
	case "http":
		return s.forwardHTTP(ctx, cfg, plan, log)
	case "local-cli":
		return s.forwardCLI(ctx, cfg, plan, log)
	default:
		return Result{}, fmt.Errorf("connected sandbox: unknown kind %q for runtime %q", kind, plan.RuntimeID)
	}
}

// forwardCLI runs the configured service command on the host and relays its output.
func (s ConnectedSandbox) forwardCLI(ctx context.Context, cfg ConnectedRuntimeConfig, plan ExecutionPlan, log LogFunc) (Result, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return Result{}, fmt.Errorf("connected sandbox: runtime %q has kind local-cli but no command configured", plan.RuntimeID)
	}

	workspace, cleanup, promptPath, err := prepareWorkspace(s.WorkspacesDir, plan.WorkDir, plan.Prompt)
	if err != nil {
		return Result{}, fmt.Errorf("connected sandbox: %w", err)
	}
	defer cleanup()

	// Substitute placeholders in the configured args.
	args := make([]string, len(cfg.Args))
	for i, a := range cfg.Args {
		a = strings.ReplaceAll(a, "{{prompt}}", plan.Prompt)
		a = strings.ReplaceAll(a, "{{prompt_file}}", promptPath)
		args[i] = a
	}

	runCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	//nolint:gosec // command/args come from operator config (config/*.yaml), not from run input.
	cmd := exec.CommandContext(runCtx, command, args...)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "BATTOS_PROMPT_FILE="+promptPath)
	for _, key := range plan.EnvKeys {
		// Prefer pre-resolved values (managed/inline_encrypted credentials).
		if val, ok := plan.ResolvedEnv[key]; ok && val != "" {
			cmd.Env = append(cmd.Env, key+"="+val)
		} else if val, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+val)
		}
	}

	if err := streamProcess(runCtx, cmd, plan.Timeout, log); err != nil {
		return Result{}, fmt.Errorf("connected sandbox (local-cli) %w", err)
	}
	return Result{Summary: "connected sandbox completed (local-cli)", Artifacts: collectArtifacts(workspace, log)}, nil
}

// forwardHTTP POSTs the prompt to the service endpoint and streams the response.
func (s ConnectedSandbox) forwardHTTP(ctx context.Context, cfg ConnectedRuntimeConfig, plan ExecutionPlan, log LogFunc) (Result, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return Result{}, fmt.Errorf("connected sandbox: runtime %q has kind http but no endpoint configured", plan.RuntimeID)
	}

	payload, err := json.Marshal(map[string]string{
		"prompt":     plan.Prompt,
		"runtime_id": plan.RuntimeID,
	})
	if err != nil {
		return Result{}, fmt.Errorf("connected sandbox: marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("connected sandbox: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}

	_ = log("system", "connected sandbox: POST "+endpoint)
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("connected sandbox: request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("connected sandbox: read response: %w", err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		_ = log("stdout", redactKnownSecrets(line))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("connected sandbox: service returned status %d", resp.StatusCode)
	}
	return Result{Summary: fmt.Sprintf("connected sandbox completed (http %d)", resp.StatusCode)}, nil
}
