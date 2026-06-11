package worker

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// collectLog returns a LogFunc that appends every line to a slice (thread-safe,
// since DirectSandbox streams stdout/stderr from two goroutines).
func collectLog() (LogFunc, func() []string) {
	var mu sync.Mutex
	var lines []string
	fn := func(stream, message string) error {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, stream+": "+message)
		return nil
	}
	get := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(lines))
		copy(out, lines)
		return out
	}
	return fn, get
}

func TestDirectSandboxEmptyCommand(t *testing.T) {
	logFn, _ := collectLog()
	_, err := DirectSandbox{}.Execute(context.Background(), ExecutionPlan{}, logFn)
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDirectSandboxRunsOnHost runs a real command on the host and asserts that
// stdout reaches the log and that a file written by the command is collected as
// an artifact. Portable across Windows (cmd) and Unix (sh).
func TestDirectSandboxRunsOnHost(t *testing.T) {
	workspaces := t.TempDir()

	var command string
	var args []string
	if runtime.GOOS == "windows" {
		command = "cmd"
		// Create outputs dir, write a file into it, print the marker.
		args = []string{"/c", "mkdir outputs & echo hi> outputs\\a.md & echo MARKER123"}
	} else {
		command = "sh"
		args = []string{"-c", "mkdir -p outputs && echo hi > outputs/a.md && echo MARKER123"}
	}

	logFn, getLines := collectLog()
	res, err := DirectSandbox{WorkspacesDir: workspaces}.Execute(
		context.Background(),
		ExecutionPlan{
			RuntimeID: "test-direct",
			Command:   command,
			Args:      args,
			Prompt:    "do the thing",
			Timeout:   30 * time.Second,
		},
		logFn,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// MARKER must have reached the log via stdout streaming.
	var sawMarker, sawHostLine bool
	for _, l := range getLines() {
		if strings.Contains(l, "MARKER123") {
			sawMarker = true
		}
		if strings.Contains(l, "running on host (no container)") {
			sawHostLine = true
		}
	}
	if !sawMarker {
		t.Errorf("expected MARKER123 in streamed logs, got: %v", getLines())
	}
	if !sawHostLine {
		t.Error("expected the 'running on host' system line")
	}

	// The file under outputs/ must be collected as an artifact (BATTOS_PROMPT.md excluded).
	var foundArtifact bool
	for _, a := range res.Artifacts {
		if a.Name == "outputs/a.md" {
			foundArtifact = true
		}
		if a.Name == "BATTOS_PROMPT.md" {
			t.Error("BATTOS_PROMPT.md should be excluded from artifacts")
		}
	}
	if !foundArtifact {
		t.Errorf("expected outputs/a.md artifact, got: %+v", res.Artifacts)
	}
}

// TestDirectSandboxTimeout asserts the process is killed and a timeout error
// returned when it exceeds plan.Timeout.
func TestDirectSandboxTimeout(t *testing.T) {
	// NOTE: not using t.TempDir() here. On Windows a timed-out process can spawn a
	// grandchild (e.g. cmd -> ping) that briefly holds the workspace directory open
	// after the parent is killed, so the immediate RemoveAll races and fails. We use
	// a best-effort cleanup that tolerates that transient lock instead of letting the
	// testing framework's strict TempDir cleanup fail the test.
	workspaces, err := os.MkdirTemp("", "direct-timeout-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspaces) })

	var command string
	var args []string
	if runtime.GOOS == "windows" {
		command = "cmd"
		// ping with delay is the portable Windows "sleep".
		args = []string{"/c", "ping -n 6 127.0.0.1 > nul"}
	} else {
		command = "sh"
		args = []string{"-c", "sleep 5"}
	}

	logFn, _ := collectLog()
	_, err = DirectSandbox{WorkspacesDir: workspaces}.Execute(
		context.Background(),
		ExecutionPlan{
			RuntimeID: "test-direct-timeout",
			Command:   command,
			Args:      args,
			Timeout:   500 * time.Millisecond,
		},
		logFn,
	)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}
