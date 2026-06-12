package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

const defaultRunTimeout = 30 * time.Minute

type CommandAdapter struct {
	RuntimeID               string
	Command                 string
	BaseArgs                []string
	ProviderEnv             string
	AuthMode                string
	HostCredentialPath      string
	ContainerCredentialPath string
	Timeout                 time.Duration
}

func (a CommandAdapter) Plan(_ context.Context, run store.Run) (ExecutionPlan, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	args := append([]string{}, a.BaseArgs...)
	envKeys := envKeys(a.ProviderEnv)
	var mounts []Mount
	if a.AuthMode == "host_session" {
		envKeys = nil
		source := strings.TrimSpace(a.HostCredentialPath)
		if source == "" {
			return ExecutionPlan{}, errMissingHostCredentialPath(a.RuntimeID)
		}
		target := strings.TrimSpace(a.ContainerCredentialPath)
		if target == "" {
			target = "/mnt/battos-codex-host"
		}
		mounts = append(mounts, Mount{Source: source, Target: target, ReadOnly: true})
	}
	return ExecutionPlan{
		RuntimeID:      a.RuntimeID,
		Command:        a.Command,
		Args:           args,
		EnvKeys:        envKeys,
		Mounts:         mounts,
		Prompt:         run.Prompt,
		NetworkEnabled: run.NetworkEnabled != 0,
		Timeout:        timeout,
	}, nil
}

// ConnectedAdapter produces a minimal plan for a "connected" runtime: the actual
// command/endpoint lives in the ConnectedSandbox service config, so the plan only
// carries the runtime id and the prompt. Command is intentionally empty —
// validatePlan permits that for the connected tier.
type ConnectedAdapter struct {
	RuntimeID string
	Timeout   time.Duration
}

func (a ConnectedAdapter) Plan(_ context.Context, run store.Run) (ExecutionPlan, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	return ExecutionPlan{
		RuntimeID: a.RuntimeID,
		Prompt:    run.Prompt,
		Timeout:   timeout,
	}, nil
}

type AdapterOptions struct {
	HostSessionEnabled   bool
	CodexCredentialsDir  string
	ClaudeCredentialsDir string
	// ConnectedRuntimeIDs are the runtime adapter ids configured as connected
	// services (config.execution.connected_runtimes); each gets a ConnectedAdapter.
	ConnectedRuntimeIDs []string
}

func ApprovedDryRunAdapters() map[string]Adapter {
	return ApprovedAdapters(AdapterOptions{})
}

func ApprovedAdapters(options AdapterOptions) map[string]Adapter {
	codexCredentialsDir := strings.TrimSpace(options.CodexCredentialsDir)
	if codexCredentialsDir == "" {
		codexCredentialsDir = defaultCodexCredentialsDir()
	}
	claudeCredentialsDir := strings.TrimSpace(options.ClaudeCredentialsDir)
	if claudeCredentialsDir == "" {
		claudeCredentialsDir = defaultClaudeCredentialsDir()
	}
	adapters := map[string]Adapter{
		"codex": CommandAdapter{
			RuntimeID:   "codex",
			Command:     "sh",
			BaseArgs:    []string{"-c", `codex exec --sandbox workspace-write --skip-git-repo-check --ephemeral --json - < "$BATTOS_PROMPT_FILE"`},
			ProviderEnv: "OPENAI_API_KEY",
		},
		"claude-code": CommandAdapter{
			RuntimeID:   "claude-code",
			Command:     "sh",
			BaseArgs:    []string{"-c", `claude --print --verbose --input-format text --output-format stream-json --no-session-persistence --dangerously-skip-permissions "$(cat "$BATTOS_PROMPT_FILE")"`},
			ProviderEnv: "ANTHROPIC_API_KEY",
		},
		// gemini: Gemini CLI de Google en modo no-interactivo (-p). --yolo
		// auto-aprueba tools (seguro porque corre dentro del sandbox/tier).
		// Los flags exactos se confirman en el smoke (Etapa 1).
		"gemini": CommandAdapter{
			RuntimeID:   "gemini",
			Command:     "sh",
			BaseArgs:    []string{"-c", `gemini --yolo -p "$(cat "$BATTOS_PROMPT_FILE")"`},
			ProviderEnv: "GEMINI_API_KEY",
		},
		// pi: harness minimalista (earendil-works/pi) en print mode (-p). Maneja su
		// propia auth/login, por eso sin ProviderEnv — ideal en tier direct (host)
		// donde su login está disponible. Flags exactos a confirmar en el smoke.
		"pi": CommandAdapter{
			RuntimeID: "pi",
			Command:   "sh",
			BaseArgs:  []string{"-c", `pi -p "$(cat "$BATTOS_PROMPT_FILE")"`},
		},
		"sandbox-smoke": CommandAdapter{
			RuntimeID: "sandbox-smoke",
			Command:   "sh",
			BaseArgs:  []string{"-c", "mkdir -p outputs && echo '# BattOS smoke artifact' > outputs/smoke.md && echo battos-worker-docker-ok && test -f \"$BATTOS_PROMPT_FILE\""},
		},
		"sandbox-memory-smoke": CommandAdapter{
			RuntimeID: "sandbox-memory-smoke",
			Command:   "sh",
			BaseArgs:  []string{"-c", "grep -q 'BattOS Memory Context' \"$BATTOS_PROMPT_FILE\" && grep -q 'memory bridge smoke marker' \"$BATTOS_PROMPT_FILE\" && mkdir -p outputs && echo '# BattOS memory smoke artifact' > outputs/smoke.md && echo battos-memory-context-ok"},
		},
	}
	if options.HostSessionEnabled {
		adapters["codex-host-session"] = CommandAdapter{
			RuntimeID:               "codex-host-session",
			Command:                 "sh",
			BaseArgs:                []string{"-c", `rm -rf /home/battos/.battos-codex-home && mkdir -p /home/battos/.battos-codex-home && for f in auth.json config.toml version.json installation_id .codex-global-state.json; do if [ -f "/mnt/battos-codex-host/$f" ]; then cp "/mnt/battos-codex-host/$f" "/home/battos/.battos-codex-home/$f"; fi; done && chmod -R u+rwX /home/battos/.battos-codex-home && CODEX_HOME=/home/battos/.battos-codex-home codex exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --ephemeral --json - < "$BATTOS_PROMPT_FILE"; status=$?; rm -rf /home/battos/.battos-codex-home; exit $status`},
			AuthMode:                "host_session",
			HostCredentialPath:      codexCredentialsDir,
			ContainerCredentialPath: "/mnt/battos-codex-host",
		}
		adapters["claude-code-host-session"] = CommandAdapter{
			RuntimeID:               "claude-code-host-session",
			Command:                 "sh",
			BaseArgs:                []string{"-c", `rm -rf /home/battos/.claude && mkdir -p /home/battos/.claude && for f in .credentials.json settings.json settings.local.json CLAUDE.md; do if [ -f "/mnt/battos-claude-host/$f" ]; then cp "/mnt/battos-claude-host/$f" "/home/battos/.claude/$f"; fi; done && claude --print --verbose --input-format text --output-format stream-json --no-session-persistence --dangerously-skip-permissions "$(cat "$BATTOS_PROMPT_FILE")"; status=$?; rm -rf /home/battos/.claude; exit $status`},
			AuthMode:                "host_session",
			HostCredentialPath:      claudeCredentialsDir,
			ContainerCredentialPath: "/mnt/battos-claude-host",
		}
	}
	// Connected runtimes (Hermes, OpenClaw, …) are config-driven: register a
	// ConnectedAdapter for each id declared in connected_runtimes so a run can
	// reference it; the ConnectedSandbox resolves how to reach the service.
	for _, id := range options.ConnectedRuntimeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		adapters[id] = ConnectedAdapter{RuntimeID: id}
	}
	return adapters
}

func defaultCodexCredentialsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
}

func defaultClaudeCredentialsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".claude")
}

func errMissingHostCredentialPath(runtimeID string) error {
	return &missingHostCredentialPathError{RuntimeID: runtimeID}
}

type missingHostCredentialPathError struct {
	RuntimeID string
}

func (e *missingHostCredentialPathError) Error() string {
	return "host_session credentials path is not configured for " + e.RuntimeID
}

func envKeys(key string) []string {
	if key == "" {
		return nil
	}
	return []string{key}
}
