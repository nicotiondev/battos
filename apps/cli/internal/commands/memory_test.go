package commands

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDecodeOrErrorUsesStructuredAPIMessage(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"Work Board no disponible"}}`)),
	}

	err := decodeOrError(resp, nil)
	if err == nil {
		t.Fatal("decodeOrError returned nil, want error")
	}
	if got, want := err.Error(), "HTTP 503: Work Board no disponible"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestRenderMemoryContextMarkdown(t *testing.T) {
	generated := time.Date(2026, 6, 1, 20, 0, 0, 0, time.UTC)
	got := renderMemoryContext([]memoryResult{{
		memoryItem: memoryItem{
			Type:      "decision",
			Title:     "Trabajar por fases",
			Content:   "Nico prefiere avanzar por roadmap, con docs vivos y smoke tests.",
			TopicKey:  "nico/work-style",
			ProjectID: "battos",
			Scope:     "project",
		},
	}}, memoryContextOptions{ProjectID: "battos", Scope: "project", Generated: generated})

	for _, want := range []string{
		"# BattOS Memory Context",
		"- Project: battos",
		"- Scope: project",
		"## [decision] Trabajar por fases",
		"topic=nico/work-style",
		"Nico prefiere avanzar por roadmap",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context markdown missing %q:\n%s", want, got)
		}
	}
}
