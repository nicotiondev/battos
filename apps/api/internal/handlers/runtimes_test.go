package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeRuntimeStore struct {
	providers      []store.Provider
	updatedStatus  map[string]string
	runtimeUpdates []store.UpdateAgentRuntimeDetectionParams
	cliUpserts     []store.UpsertCLIToolDetectionParams
	cliTools       map[string]store.CliTool
	installs       map[string]store.CliToolInstall
	installSeq     int
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
		RequiresAuth: 1,
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

func (f *fakeRuntimeStore) GetCLITool(_ context.Context, id string) (store.CliTool, error) {
	tool, ok := f.cliTools[id]
	if !ok {
		return store.CliTool{}, sql.ErrNoRows
	}
	return tool, nil
}

func (f *fakeRuntimeStore) CreateCLIToolInstall(_ context.Context, arg store.CreateCLIToolInstallParams) (store.CliToolInstall, error) {
	if f.installs == nil {
		f.installs = map[string]store.CliToolInstall{}
	}
	f.installSeq++
	item := store.CliToolInstall{
		ID:             fmt.Sprintf("install-%d", f.installSeq),
		CliToolID:      arg.CliToolID,
		InstallCommand: arg.InstallCommand,
		Status:         "pending_approval",
		RequestedAt:    time.Now(),
	}
	f.installs[item.ID] = item
	return item, nil
}

func (f *fakeRuntimeStore) GetCLIToolInstall(_ context.Context, id string) (store.CliToolInstall, error) {
	item, ok := f.installs[id]
	if !ok {
		return store.CliToolInstall{}, sql.ErrNoRows
	}
	return item, nil
}

func (f *fakeRuntimeStore) ListCLIToolInstalls(_ context.Context, toolID string) ([]store.CliToolInstall, error) {
	var out []store.CliToolInstall
	for _, item := range f.installs {
		if item.CliToolID == toolID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRuntimeStore) DecideCLIToolInstall(_ context.Context, arg store.DecideCLIToolInstallParams) (store.CliToolInstall, error) {
	item, ok := f.installs[arg.ID]
	if !ok || item.Status != "pending_approval" {
		return store.CliToolInstall{}, sql.ErrNoRows
	}
	item.Status = arg.Status
	item.Reason = arg.Reason
	item.DecidedAt = sql.NullTime{Time: time.Now(), Valid: true}
	f.installs[arg.ID] = item
	return item, nil
}

func (f *fakeRuntimeStore) FinishCLIToolInstall(_ context.Context, arg store.FinishCLIToolInstallParams) (store.CliToolInstall, error) {
	item, ok := f.installs[arg.ID]
	if !ok || item.Status != "running" {
		return store.CliToolInstall{}, sql.ErrNoRows
	}
	item.Status = arg.Status
	item.Output = arg.Output
	item.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
	f.installs[arg.ID] = item
	return item, nil
}

func TestDetectProvidersUsesEnvPresenceWithoutExposingSecret(t *testing.T) {
	t.Setenv("BATTOS_TEST_PROVIDER_KEY", "super-secret")
	q := &fakeRuntimeStore{providers: []store.Provider{{
		ID:          "test",
		Name:        "Test Provider",
		Kind:        "api",
		EnvKey:      "BATTOS_TEST_PROVIDER_KEY",
		Status:      "not_configured",
		LastCheckAt: sql.NullTime{Time: time.Now(), Valid: true},
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
		RequiresAuth: 1,
		BinaryPath:   sql.NullString{String: "C:/tools/claude.exe", Valid: true},
	})
	if !dto.ApprovalRequired {
		t.Fatalf("approval_required = false, want true")
	}
	if dto.ApprovedForExecution {
		t.Fatalf("approved_for_execution = true, want false after detection")
	}
}

func newInstallTestRouter(h *RuntimeHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/cli-tools/{id}/install", h.RequestCLIToolInstall)
	r.Get("/cli-tools/{id}/installs", h.ListCLIToolInstallHistory)
	r.Post("/cli-tools/installs/{installId}/approve", h.ApproveCLIToolInstall)
	return r
}

func installToolFixture() store.CliTool {
	return store.CliTool{
		ID:             "gemini",
		Name:           "Gemini CLI",
		Command:        "gemini",
		Kind:           "coding_agent",
		Status:         "not_detected",
		RiskLevel:      "medium",
		InstallCommand: sql.NullString{String: "npm install -g @google/gemini-cli", Valid: true},
		InstallUrl:     sql.NullString{String: "https://github.com/google-gemini/gemini-cli", Valid: true},
	}
}

func TestRequestCLIToolInstallCreatesPendingApproval(t *testing.T) {
	q := &fakeRuntimeStore{cliTools: map[string]store.CliTool{"gemini": installToolFixture()}}
	h := NewRuntimeHandler(q)
	router := newInstallTestRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/gemini/install", strings.NewReader("{}")))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Install cliToolInstallResponse `json:"install"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Install.Status != "pending_approval" {
		t.Fatalf("install status = %q, want pending_approval", resp.Install.Status)
	}
	if resp.Install.InstallCommand != "npm install -g @google/gemini-cli" {
		t.Fatalf("install command = %q", resp.Install.InstallCommand)
	}
}

func TestRequestCLIToolInstallRejectsToolWithoutCommandOrUnknown(t *testing.T) {
	tool := installToolFixture()
	tool.InstallCommand = sql.NullString{}
	q := &fakeRuntimeStore{cliTools: map[string]store.CliTool{"gemini": tool}}
	h := NewRuntimeHandler(q)
	router := newInstallTestRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/gemini/install", strings.NewReader("{}")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin install_command: status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/no-existe/install", strings.NewReader("{}")))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("tool desconocida: status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestApproveCLIToolInstallRunsCommandAndRedetects(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "supersecreto12345")
	q := &fakeRuntimeStore{cliTools: map[string]store.CliTool{"gemini": installToolFixture()}}
	h := NewRuntimeHandler(q)
	h.spawn = func(f func()) { f() } // síncrono para el test
	var ranCommand string
	h.installRunner = func(_ context.Context, command string) (string, error) {
		ranCommand = command
		return "added 1 package usando token supersecreto12345", nil
	}
	h.lookPath = func(string) (string, error) { return "C:/tools/gemini.exe", nil }
	h.commandVersion = func(context.Context, string) (string, error) { return "gemini 1.0.0", nil }
	h.getenv = func(string) string { return "" }
	router := newInstallTestRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/gemini/install", strings.NewReader("{}")))
	var created struct {
		Install cliToolInstallResponse `json:"install"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/cli-tools/installs/"+created.Install.ID+"/approve",
		strings.NewReader(`{"decision":"approved"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ranCommand != "npm install -g @google/gemini-cli" {
		t.Fatalf("ranCommand = %q", ranCommand)
	}

	final, err := q.GetCLIToolInstall(context.Background(), created.Install.ID)
	if err != nil {
		t.Fatalf("get final install: %v", err)
	}
	if final.Status != "succeeded" {
		t.Fatalf("final status = %q, want succeeded; output=%s", final.Status, final.Output.String)
	}
	if strings.Contains(final.Output.String, "supersecreto12345") {
		t.Fatalf("output no redactado: %s", final.Output.String)
	}
	if !strings.Contains(final.Output.String, "[redacted:GITHUB_TOKEN]") {
		t.Fatalf("falta marca de redaccion: %s", final.Output.String)
	}
	if len(q.cliUpserts) != 1 || len(q.runtimeUpdates) != 1 {
		t.Fatalf("re-deteccion no corrio: upserts=%d runtimeUpdates=%d", len(q.cliUpserts), len(q.runtimeUpdates))
	}
	if q.cliUpserts[0].Status != "detected" {
		t.Fatalf("status post-deteccion = %q, want detected", q.cliUpserts[0].Status)
	}
}

func TestApproveCLIToolInstallRejectedDoesNotRun(t *testing.T) {
	q := &fakeRuntimeStore{cliTools: map[string]store.CliTool{"gemini": installToolFixture()}}
	h := NewRuntimeHandler(q)
	h.spawn = func(f func()) { f() }
	ran := false
	h.installRunner = func(context.Context, string) (string, error) { ran = true; return "", nil }
	router := newInstallTestRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/gemini/install", strings.NewReader("{}")))
	var created struct {
		Install cliToolInstallResponse `json:"install"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/cli-tools/installs/"+created.Install.ID+"/approve",
		strings.NewReader(`{"decision":"rejected","reason":"no quiero"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ran {
		t.Fatal("installRunner corrio con decision rejected")
	}

	// Re-decidir una solicitud ya decidida → 400.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/cli-tools/installs/"+created.Install.ID+"/approve",
		strings.NewReader(`{"decision":"approved"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("re-decision status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestApproveCLIToolInstallFailureKeepsOutput(t *testing.T) {
	q := &fakeRuntimeStore{cliTools: map[string]store.CliTool{"gemini": installToolFixture()}}
	h := NewRuntimeHandler(q)
	h.spawn = func(f func()) { f() }
	h.installRunner = func(context.Context, string) (string, error) {
		return "npm ERR! network timeout", errors.New("exit status 1")
	}
	router := newInstallTestRouter(h)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cli-tools/gemini/install", strings.NewReader("{}")))
	var created struct {
		Install cliToolInstallResponse `json:"install"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/cli-tools/installs/"+created.Install.ID+"/approve",
		strings.NewReader(`{"decision":"approved"}`)))

	final, err := q.GetCLIToolInstall(context.Background(), created.Install.ID)
	if err != nil {
		t.Fatalf("get final install: %v", err)
	}
	if final.Status != "failed" {
		t.Fatalf("final status = %q, want failed", final.Status)
	}
	if !strings.Contains(final.Output.String, "npm ERR! network timeout") || !strings.Contains(final.Output.String, "exit status 1") {
		t.Fatalf("output incompleto: %s", final.Output.String)
	}
	if len(q.cliUpserts) != 0 {
		t.Fatal("no debe re-detectar tras una instalacion fallida")
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
		Capabilities: `["code_editing"]`,
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
