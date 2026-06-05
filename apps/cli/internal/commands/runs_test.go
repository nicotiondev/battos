package commands

import (
	"strings"
	"testing"
)

func TestRenderRunMemorySummaryOmitsPromptByDefault(t *testing.T) {
	item := runItem{
		ID:               "12345678-aaaa-bbbb-cccc-123456789abc",
		ProjectID:        "battos",
		TaskID:           "task-1",
		AgentID:          "agent-1",
		RuntimeAdapterID: "sandbox-memory-smoke",
		Prompt:           "secret-ish prompt",
		Status:           "succeeded",
		ResultSummary:    "docker sandbox completed",
	}
	logs := []runLogItem{
		{Stream: "system", Message: "run claimed by worker"},
		{Stream: "stdout", Message: "battos-memory-context-ok"},
	}

	got := renderRunMemorySummary(item, logs, runMemorySummaryOptions{IncludeLogs: true, LogLimit: 10})

	for _, want := range []string{
		"# BattOS Run Summary",
		"- Project: battos",
		"- Runtime: sandbox-memory-smoke",
		"- Result: docker sandbox completed",
		"`stdout` battos-memory-context-ok",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret-ish prompt") {
		t.Fatalf("summary included prompt without IncludePrompt:\n%s", got)
	}
}

func TestTailRunLogs(t *testing.T) {
	logs := []runLogItem{
		{Message: "1"},
		{Message: "2"},
		{Message: "3"},
	}
	got := tailRunLogs(logs, 2)
	if len(got) != 2 || got[0].Message != "2" || got[1].Message != "3" {
		t.Fatalf("tail=%+v, want last two logs", got)
	}
}
