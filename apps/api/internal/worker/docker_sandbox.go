package worker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultDockerImage = "alpine:3.20"

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandOutput, error)
}

type CommandOutput struct {
	Stdout string
	Stderr string
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) (CommandOutput, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return CommandOutput{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

type DockerSandbox struct {
	Image         string
	WorkspacesDir string
	Runner        CommandRunner
}

func (s DockerSandbox) Execute(ctx context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	image := defaultString(s.Image, defaultDockerImage)
	root := defaultString(s.WorkspacesDir, "data/runs/workspaces")
	root, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve workspaces root: %w", err)
	}
	runner := s.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Result{}, fmt.Errorf("create workspaces root: %w", err)
	}
	workspace := plan.WorkDir
	isTempWorkspace := false
	if strings.TrimSpace(workspace) == "" {
		var errTemp error
		workspace, errTemp = os.MkdirTemp(root, "run-*")
		if errTemp != nil {
			return Result{}, fmt.Errorf("create run workspace: %w", errTemp)
		}
		isTempWorkspace = true
	}
	if isTempWorkspace {
		defer os.RemoveAll(workspace)
	}
	if err := os.Chmod(workspace, 0o777); err != nil {
		return Result{}, fmt.Errorf("make run workspace writable: %w", err)
	}

	if strings.TrimSpace(plan.Prompt) != "" {
		if err := os.WriteFile(filepath.Join(workspace, "BATTOS_PROMPT.md"), []byte(plan.Prompt), 0o600); err != nil {
			return Result{}, fmt.Errorf("write prompt file: %w", err)
		}
	}
	if err := validateMounts(plan.Mounts); err != nil {
		return Result{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	args := dockerArgs(image, workspace, plan)
	if err := log("system", "sandbox docker: starting ephemeral container"); err != nil {
		return Result{}, err
	}
	if err := log("system", "network: "+networkLabel(plan.NetworkEnabled)); err != nil {
		return Result{}, err
	}

	if _, isExec := runner.(ExecCommandRunner); isExec {
		cmd := exec.CommandContext(runCtx, "docker", args...)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return Result{}, fmt.Errorf("stdout pipe: %w", err)
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return Result{}, fmt.Errorf("stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return Result{}, fmt.Errorf("docker run start: %w", err)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		logStream := func(stream string, pipe io.Reader) {
			defer wg.Done()
			scanner := bufio.NewScanner(pipe)
			buf := make([]byte, 64*1024)
			scanner.Buffer(buf, 4*1024*1024) // Buffer inicial 64KB, max 4MB

			for scanner.Scan() {
				line := scanner.Text()
				line = redactKnownSecrets(line)
				_ = log(stream, line)
			}
		}

		go logStream("stdout", stdoutPipe)
		go logStream("stderr", stderrPipe)

		err = cmd.Wait()
		wg.Wait()

		if err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				return Result{}, fmt.Errorf("docker sandbox timeout after %s", plan.Timeout)
			}
			return Result{}, fmt.Errorf("docker sandbox command failed: %w", err)
		}
	} else {
		out, err := runner.Run(runCtx, "docker", args...)
		logCommandOutput(log, "stdout", out.Stdout)
		logCommandOutput(log, "stderr", out.Stderr)
		if err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				return Result{}, fmt.Errorf("docker sandbox timeout after %s", plan.Timeout)
			}
			return Result{}, fmt.Errorf("docker sandbox command failed: %w", err)
		}
	}

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
		_ = log("system", "error scanning workspace artifacts: "+errScan.Error())
	}

	return Result{Summary: "docker sandbox completed", Artifacts: artifacts}, nil
}

func dockerArgs(image, workspace string, plan ExecutionPlan) []string {
	network := "none"
	if plan.NetworkEnabled {
		network = "bridge"
	}
	name := "battos-run-" + safeContainerSuffix(plan.RuntimeID) + "-" + time.Now().UTC().Format("20060102150405")
	args := []string{
		"run",
		"--rm",
		"--name", name,
		"--network", network,
		"-v", filepath.Clean(workspace) + ":/workspace",
		"-w", "/workspace",
		"-e", "BATTOS_PROMPT_FILE=/workspace/BATTOS_PROMPT.md",
	}
	for _, mount := range plan.Mounts {
		source := filepath.Clean(mount.Source)
		target := mount.Target
		mode := "rw"
		if mount.ReadOnly {
			mode = "ro"
		}
		args = append(args, "-v", source+":"+target+":"+mode)
	}
	for _, key := range plan.EnvKeys {
		args = append(args, "-e", key)
	}
	args = append(args, image, plan.Command)
	args = append(args, plan.Args...)
	return args
}

func validateMounts(mounts []Mount) error {
	for _, mount := range mounts {
		source := strings.TrimSpace(mount.Source)
		target := strings.TrimSpace(mount.Target)
		if source == "" || target == "" {
			return fmt.Errorf("host_session mount source and target are required")
		}
		if !filepath.IsAbs(source) {
			return fmt.Errorf("host_session mount source must be absolute: %s", source)
		}
		if !strings.HasPrefix(target, "/") {
			return fmt.Errorf("host_session mount target must be absolute container path: %s", target)
		}
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("host_session mount source unavailable %s: %w", source, err)
		}
	}
	return nil
}

func logCommandOutput(log LogFunc, stream, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	value = redactKnownSecrets(value)
	const maxLogBytes = 64 * 1024
	if len(value) > maxLogBytes {
		value = value[:maxLogBytes] + "\n[truncated]"
	}
	_ = log(stream, value)
}

func redactKnownSecrets(value string) string {
	// 1. Redact known LLM API keys
	for _, key := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY",
		// 2. Redact BattOS and Git tokens
		"BATTOS_API_TOKEN", "GITHUB_TOKEN", "GH_TOKEN",
	} {
		secret := strings.TrimSpace(os.Getenv(key))
		if len(secret) >= 8 {
			value = strings.ReplaceAll(value, secret, "[redacted:"+key+"]")
		}
	}
	// 3. Redact any BATTOS_CREDENTIAL_* env vars (used by CredentialRef)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "BATTOS_CREDENTIAL_") {
			secret := strings.TrimSpace(parts[1])
			if len(secret) >= 8 {
				value = strings.ReplaceAll(value, secret, "[redacted:"+parts[0]+"]")
			}
		}
	}
	return value
}

func networkLabel(enabled bool) string {
	if enabled {
		return "enabled by approval"
	}
	return "disabled"
}

var unsafeContainerChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func safeContainerSuffix(value string) string {
	cleaned := strings.Trim(unsafeContainerChars.ReplaceAllString(value, "-"), ".-")
	if cleaned == "" {
		return "run"
	}
	return cleaned
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func detectArtifactKind(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md":
		return "markdown"
	case ".diff":
		return "diff"
	case ".png", ".jpg", ".jpeg", ".gif":
		return "image"
	default:
		return "markdown"
	}
}
