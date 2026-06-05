package worker

import (
	"context"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

const defaultRunTimeout = 30 * time.Minute

type CommandAdapter struct {
	RuntimeID   string
	Command     string
	BaseArgs    []string
	ProviderEnv string
	Timeout     time.Duration
}

func (a CommandAdapter) Plan(_ context.Context, run store.Run) (ExecutionPlan, error) {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	args := append([]string{}, a.BaseArgs...)
	return ExecutionPlan{
		RuntimeID:      a.RuntimeID,
		Command:        a.Command,
		Args:           args,
		EnvKeys:        envKeys(a.ProviderEnv),
		Prompt:         run.Prompt,
		NetworkEnabled: run.NetworkEnabled,
		Timeout:        timeout,
	}, nil
}

func ApprovedDryRunAdapters() map[string]Adapter {
	return map[string]Adapter{
		"codex": CommandAdapter{
			RuntimeID:   "codex",
			Command:     "sh",
			BaseArgs:    []string{"-c", `codex exec --sandbox workspace-write --ask-for-approval never --skip-git-repo-check --ephemeral --json - < "$BATTOS_PROMPT_FILE"`},
			ProviderEnv: "OPENAI_API_KEY",
		},
		"claude-code": CommandAdapter{
			RuntimeID:   "claude-code",
			Command:     "sh",
			BaseArgs:    []string{"-c", `claude --bare --print --input-format text --output-format stream-json --no-session-persistence --dangerously-skip-permissions "$(cat "$BATTOS_PROMPT_FILE")"`},
			ProviderEnv: "ANTHROPIC_API_KEY",
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
}

func envKeys(key string) []string {
	if key == "" {
		return nil
	}
	return []string{key}
}
