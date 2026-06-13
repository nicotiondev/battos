package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// host_exec.go holds the shared host-process machinery used by the non-Docker
// sandboxes (DirectSandbox and ConnectedSandbox's local-cli mode): workspace
// preparation, line-streamed process execution with secret redaction, and
// artifact collection. DockerSandbox has its own container-based path.

// prepareWorkspace resolves (or creates) the run workspace under workspacesDir
// and writes the prompt to BATTOS_PROMPT.md. When planWorkDir is provided it is
// reused as-is and cleanup is a no-op; otherwise an ephemeral run-* dir is
// created and cleanup removes it. promptPath is the absolute path of the written
// prompt file (empty prompt → file is not written but the path is still returned).
func prepareWorkspace(workspacesDir, planWorkDir, prompt string) (workspace string, cleanup func(), promptPath string, err error) {
	cleanup = func() {}

	root := defaultString(workspacesDir, "data/runs/workspaces")
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", cleanup, "", fmt.Errorf("resolve workspaces root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return "", cleanup, "", fmt.Errorf("create workspaces root: %w", err)
	}

	workspace = strings.TrimSpace(planWorkDir)
	if workspace == "" {
		workspace, err = os.MkdirTemp(absRoot, "run-*")
		if err != nil {
			return "", cleanup, "", fmt.Errorf("create run workspace: %w", err)
		}
		ws := workspace
		cleanup = func() { _ = os.RemoveAll(ws) }
	}
	if err := os.Chmod(workspace, 0o755); err != nil {
		cleanup()
		return "", func() {}, "", fmt.Errorf("make workspace accessible: %w", err)
	}

	promptPath = filepath.Join(workspace, "BATTOS_PROMPT.md")
	if strings.TrimSpace(prompt) != "" {
		if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
			cleanup()
			return "", func() {}, "", fmt.Errorf("write prompt file: %w", err)
		}
	}
	return workspace, cleanup, promptPath, nil
}

// streamProcess starts cmd, streams its stdout and stderr line-by-line to log
// (each line passed through redactKnownSecrets), and waits for it to exit. If
// runCtx hit its deadline, a timeout error mentioning timeout is returned.
func streamProcess(runCtx context.Context, cmd *exec.Cmd, timeout time.Duration, log LogFunc) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	scan := func(stream string, r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			_ = log(stream, redactKnownSecrets(sc.Text()))
		}
	}
	go scan("stdout", stdoutPipe)
	go scan("stderr", stderrPipe)

	cmdErr := cmd.Wait()
	wg.Wait()

	if cmdErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timeout after %s", timeout)
		}
		return cmdErr
	}
	return nil
}

// teamMCPConfigFilename es el archivo MCP que el sandbox materializa en el
// workspace cuando el run lleva tools de equipo (ver ExecutionPlan.MCPConfigJSON).
const teamMCPConfigFilename = "battos-mcp.json"

// collectArtifacts walks workspace and returns every regular file (except the
// injected BATTOS_PROMPT.md and battos-mcp.json) as a ProducedArtifact, with
// relative slash paths.
func collectArtifacts(workspace string, log LogFunc) []ProducedArtifact {
	var artifacts []ProducedArtifact
	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
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
		if rel == "BATTOS_PROMPT.md" || rel == teamMCPConfigFilename {
			return nil
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, ProducedArtifact{
			Name:    rel,
			Kind:    detectArtifactKind(rel),
			Content: string(content),
		})
		return nil
	})
	if err != nil {
		_ = log("system", "error scanning workspace artifacts: "+err.Error())
	}
	return artifacts
}
