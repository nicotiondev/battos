package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeRuntimeStore struct {
	providers      []store.Provider
	updatedStatus  map[string]string
	runtimeUpdates []store.UpdateAgentRuntimeDetectionParams
	cliUpserts     []store.UpsertCLIToolDetectionParams
}

func (f *fakeRuntimeStore) ListAgentRuntimes(context.Context) ([]store.AgentRuntime, error) {
	return nil, nil
}

func (f *fakeRuntimeStore) UpdateAgentRuntimeDetection(_ context.Context, arg store.UpdateAgentRuntimeDetectionParams) (store.AgentRuntime, error) {
	f.runtimeUpdates = append(f.runtimeUpdates, arg)
	return store.AgentRuntime{
		ID:           arg.ID,
		Name:         arg.ID,
		Status:       arg.Status,
		BinaryPath:   arg.BinaryPath,
		Version:      arg.Version,
		RequiresAuth: true,
	}, nil
}

func (f *fakeRuntimeStore) ListCLITools(context.Context) ([]store.CliTool, error) {
	return nil, nil
}

func (f *fakeRuntimeStore) UpsertCLIToolDetection(_ context.Context, arg store.UpsertCLIToolDetectionParams) (store.CliTool, error) {
	f.cliUpserts = append(f.cliUpserts, arg)
	return store.CliTool{}, nil
}

func (f *fakeRuntimeStore) ListProviders(context.Context) ([]store.Provider, error) {
	return f.providers, nil
}

func (f *fakeRuntimeStore) UpdateProviderStatus(_ context.Context, arg store.UpdateProviderStatusParams) error {
	if f.updatedStatus == nil {
		f.updatedStatus = map[string]string{}
	}
	f.updatedStatus[arg.ID] = arg.Status
	return nil
}

func TestDetectProvidersUsesEnvPresenceWithoutExposingSecret(t *testing.T) {
	t.Setenv("BATTOS_TEST_PROVIDER_KEY", "super-secret")
	q := &fakeRuntimeStore{providers: []store.Provider{{
		ID:          "test",
		Name:        "Test Provider",
		Kind:        "api",
		EnvKey:      "BATTOS_TEST_PROVIDER_KEY",
		Status:      "not_configured",
		LastCheckAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}}
	h := NewRuntimeHandler(q)
	req := httptest.NewRequest(http.MethodPost, "/providers/detect", nil)
	rec := httptest.NewRecorder()

	h.DetectProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if q.updatedStatus["test"] != "configured" {
		t.Fatalf("provider status = %q, want configured", q.updatedStatus["test"])
	}
	if strings.Contains(rec.Body.String(), os.Getenv("BATTOS_TEST_PROVIDER_KEY")) {
		t.Fatalf("provider detection response leaked secret: %s", rec.Body.String())
	}
}

func TestRuntimeAdapterDTORequiresApprovalAndDoesNotApproveExecution(t *testing.T) {
	dto := runtimeAdapterDTO(store.AgentRuntime{
		ID:           "claude-code",
		Name:         "Claude Code CLI",
		Status:       "detected",
		RequiresAuth: true,
		BinaryPath:   pgtype.Text{String: "C:/tools/claude.exe", Valid: true},
	})
	if !dto.ApprovalRequired {
		t.Fatalf("approval_required = false, want true")
	}
	if dto.ApprovedForExecution {
		t.Fatalf("approved_for_execution = true, want false after detection")
	}
}

func TestDetectRuntimeClassifiesAndPersistsStates(t *testing.T) {
	spec := runtimeToolSpec{
		ID:           "claude-code",
		Name:         "Claude Code",
		Command:      "claude",
		Kind:         "coding_agent",
		RuntimeID:    "claude-code",
		RiskLevel:    "high",
		RequiresAuth: true,
		ProviderEnv:  "ANTHROPIC_API_KEY",
		Capabilities: []byte(`["code_editing"]`),
	}
	tests := []struct {
		name          string
		path          string
		lookErr       error
		version       string
		versionErr    error
		env           string
		wantRuntime   string
		wantCLI       string
		wantVersion   string
		wantPathValid bool
	}{
		{
			name:          "configured when binary and provider env exist",
			path:          "C:/tools/claude.exe",
			version:       "claude 1.2.3",
			env:           "secret",
			wantRuntime:   "configured",
			wantCLI:       "detected",
			wantVersion:   "claude 1.2.3",
			wantPathValid: true,
		},
		{
			name:          "detected when binary exists without provider env",
			path:          "C:/tools/claude.exe",
			version:       "claude 1.2.3",
			wantRuntime:   "detected",
			wantCLI:       "detected",
			wantVersion:   "claude 1.2.3",
			wantPathValid: true,
		},
		{
			name:          "blocked when version probe times out",
			path:          "C:/tools/claude.exe",
			versionErr:    context.DeadlineExceeded,
			wantRuntime:   "blocked",
			wantCLI:       "broken",
			wantPathValid: true,
		},
		{
			name:        "unavailable when binary is missing",
			lookErr:     errors.New("not found"),
			wantRuntime: "unavailable",
			wantCLI:     "not_detected",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &fakeRuntimeStore{}
			h := NewRuntimeHandler(q)
			h.lookPath = func(command string) (string, error) {
				if command != "claude" {
					t.Fatalf("lookPath command = %q, want claude", command)
				}
				return tt.path, tt.lookErr
			}
			h.commandVersion = func(_ context.Context, path string) (string, error) {
				if path != tt.path {
					t.Fatalf("commandVersion path = %q, want %q", path, tt.path)
				}
				return tt.version, tt.versionErr
			}
			h.getenv = func(key string) string {
				if key != "ANTHROPIC_API_KEY" {
					t.Fatalf("getenv key = %q, want ANTHROPIC_API_KEY", key)
				}
				return tt.env
			}

			runtime, err := h.detectRuntime(context.Background(), spec)
			if err != nil {
				t.Fatalf("detectRuntime returned error: %v", err)
			}
			if runtime.Status != tt.wantRuntime {
				t.Fatalf("runtime status = %q, want %q", runtime.Status, tt.wantRuntime)
			}
			if len(q.runtimeUpdates) != 1 {
				t.Fatalf("runtimeUpdates len = %d, want 1", len(q.runtimeUpdates))
			}
			if q.runtimeUpdates[0].Status != tt.wantRuntime {
				t.Fatalf("persisted runtime status = %q, want %q", q.runtimeUpdates[0].Status, tt.wantRuntime)
			}
			if len(q.cliUpserts) != 1 {
				t.Fatalf("cliUpserts len = %d, want 1", len(q.cliUpserts))
			}
			if q.cliUpserts[0].Status != tt.wantCLI {
				t.Fatalf("persisted CLI status = %q, want %q", q.cliUpserts[0].Status, tt.wantCLI)
			}
			if q.cliUpserts[0].DetectedPath.Valid != tt.wantPathValid {
				t.Fatalf("detected path valid = %v, want %v", q.cliUpserts[0].DetectedPath.Valid, tt.wantPathValid)
			}
			if q.cliUpserts[0].Version.String != tt.wantVersion {
				t.Fatalf("version = %q, want %q", q.cliUpserts[0].Version.String, tt.wantVersion)
			}
		})
	}
}
