package worker

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConnectedAdapterPlan(t *testing.T) {
	run := testRunWithMode("hermes", "connected")
	run.Prompt = "do connected work"
	plan, err := ConnectedAdapter{RuntimeID: "hermes"}.Plan(context.Background(), run)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.RuntimeID != "hermes" {
		t.Errorf("RuntimeID=%q, want hermes", plan.RuntimeID)
	}
	if plan.Command != "" {
		t.Errorf("Command=%q, want empty (resolved by ConnectedSandbox)", plan.Command)
	}
	if plan.Prompt != "do connected work" {
		t.Errorf("Prompt=%q", plan.Prompt)
	}
	if plan.Timeout <= 0 {
		t.Error("Timeout should default to a positive value")
	}
	// The plan must pass validation precisely because the run is connected mode.
	if err := validatePlan(plan, run); err != nil {
		t.Errorf("validatePlan rejected a connected plan with empty command: %v", err)
	}
}

func TestValidatePlanRequiresCommandForNonConnected(t *testing.T) {
	run := testRunWithMode("hermes", "sandbox")
	plan := ExecutionPlan{RuntimeID: "hermes", Timeout: time.Second}
	if err := validatePlan(plan, run); err == nil {
		t.Error("expected validatePlan to require a command for non-connected runs")
	}
}

func TestConnectedSandboxNoServiceConfigured(t *testing.T) {
	logFn, _ := collectLog()
	_, err := ConnectedSandbox{Runtimes: map[string]ConnectedRuntimeConfig{}}.Execute(
		context.Background(),
		ExecutionPlan{RuntimeID: "hermes", Timeout: time.Second},
		logFn,
	)
	if err == nil {
		t.Fatal("expected error when no service is configured for the runtime")
	}
	if !strings.Contains(err.Error(), "no service configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectedSandboxHTTPForward(t *testing.T) {
	var gotPrompt, gotRuntime string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// crude extraction — avoids pulling json into the test for two fields
		s := string(body)
		if strings.Contains(s, `"prompt":"build the thing"`) {
			gotPrompt = "build the thing"
		}
		if strings.Contains(s, `"runtime_id":"openclaw"`) {
			gotRuntime = "openclaw"
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("service did the work\nline two"))
	}))
	defer srv.Close()

	sb := ConnectedSandbox{
		Runtimes: map[string]ConnectedRuntimeConfig{
			"openclaw": {Kind: "http", Endpoint: srv.URL},
		},
	}
	logFn, getLines := collectLog()
	res, err := sb.Execute(context.Background(), ExecutionPlan{
		RuntimeID: "openclaw",
		Prompt:    "build the thing",
		Timeout:   30 * time.Second,
	}, logFn)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPrompt != "build the thing" || gotRuntime != "openclaw" {
		t.Errorf("service received prompt=%q runtime=%q", gotPrompt, gotRuntime)
	}
	joined := strings.Join(getLines(), "\n")
	if !strings.Contains(joined, "service did the work") || !strings.Contains(joined, "line two") {
		t.Errorf("response body not streamed to log: %v", getLines())
	}
	if !strings.Contains(res.Summary, "http 200") {
		t.Errorf("summary=%q, want http 200", res.Summary)
	}
}

func TestConnectedSandboxHTTPNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	sb := ConnectedSandbox{Runtimes: map[string]ConnectedRuntimeConfig{
		"openclaw": {Kind: "http", Endpoint: srv.URL},
	}}
	logFn, _ := collectLog()
	_, err := sb.Execute(context.Background(), ExecutionPlan{
		RuntimeID: "openclaw", Prompt: "x", Timeout: 30 * time.Second,
	}, logFn)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status 500 error, got %v", err)
	}
}

func TestConnectedSandboxLocalCLIForward(t *testing.T) {
	workspaces := t.TempDir()

	var command string
	var args []string
	if runtime.GOOS == "windows" {
		command = "cmd"
		// echo the substituted prompt, and write an artifact into outputs/.
		args = []string{"/c", "echo PROMPT={{prompt}} & mkdir outputs & echo done> outputs\\r.md"}
	} else {
		command = "sh"
		args = []string{"-c", "echo PROMPT={{prompt}}; mkdir -p outputs; echo done > outputs/r.md"}
	}

	sb := ConnectedSandbox{
		WorkspacesDir: workspaces,
		Runtimes: map[string]ConnectedRuntimeConfig{
			"hermes": {Kind: "local-cli", Command: command, Args: args},
		},
	}
	logFn, getLines := collectLog()
	res, err := sb.Execute(context.Background(), ExecutionPlan{
		RuntimeID: "hermes",
		Prompt:    "summarize",
		Timeout:   30 * time.Second,
	}, logFn)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(strings.Join(getLines(), "\n"), "PROMPT=summarize") {
		t.Errorf("expected substituted prompt in output, got: %v", getLines())
	}
	var foundArtifact bool
	for _, a := range res.Artifacts {
		if a.Name == "outputs/r.md" {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Errorf("expected outputs/r.md artifact, got: %+v", res.Artifacts)
	}
}

func TestConnectedSandboxLocalCLIMissingCommand(t *testing.T) {
	sb := ConnectedSandbox{Runtimes: map[string]ConnectedRuntimeConfig{
		"hermes": {Kind: "local-cli"},
	}}
	logFn, _ := collectLog()
	_, err := sb.Execute(context.Background(), ExecutionPlan{
		RuntimeID: "hermes", Timeout: time.Second,
	}, logFn)
	if err == nil || !strings.Contains(err.Error(), "no command configured") {
		t.Fatalf("expected missing-command error, got %v", err)
	}
}
