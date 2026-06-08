package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeRegistriesStore struct {
	createAgentArg store.CreateAgentParams
}

func (f *fakeRegistriesStore) CreateAgent(_ context.Context, arg store.CreateAgentParams) (store.Agent, error) {
	f.createAgentArg = arg
	return store.Agent{
		ID:              arg.ID,
		Slug:            arg.Slug,
		Name:            arg.Name,
		Role:            arg.Role,
		Description:     arg.Description,
		RuntimeID:       arg.RuntimeID,
		RuntimeConfig:   arg.RuntimeConfig,
		SystemPrompt:    arg.SystemPrompt,
		AllowedTools:    arg.AllowedTools,
		AllowedProjects: arg.AllowedProjects,
		RiskLevel:       arg.RiskLevel,
		IsLead:          0,
		IsMeta:          0,
		Status:          arg.Status,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil
}

func (f *fakeRegistriesStore) ListAgents(context.Context) ([]store.Agent, error) {
	return nil, nil
}

func (f *fakeRegistriesStore) ListSkills(context.Context) ([]store.Skill, error) {
	return nil, nil
}

func TestCreateAgentUsesSafeDefaults(t *testing.T) {
	q := &fakeRegistriesStore{}
	h := NewRegistriesHandler(q)

	req := httptest.NewRequest(http.MethodPost, "/agents", strings.NewReader(`{
		"slug":"builder-web",
		"name":"Builder Web",
		"role":"web_builder",
		"runtime_id":"codex",
		"system_prompt":"Construye y valida codigo."
	}`))
	res := httptest.NewRecorder()

	h.CreateAgent(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if q.createAgentArg.ID != "builder-web" {
		t.Fatalf("id = %q, want builder-web", q.createAgentArg.ID)
	}
	if q.createAgentArg.RuntimeID.String != "codex" || !q.createAgentArg.RuntimeID.Valid {
		t.Fatalf("runtime_id = %#v, want codex", q.createAgentArg.RuntimeID)
	}
	if q.createAgentArg.RuntimeConfig != "{}" {
		t.Fatalf("runtime_config = %s, want {}", q.createAgentArg.RuntimeConfig)
	}
	if q.createAgentArg.AllowedTools != "[]" || q.createAgentArg.AllowedProjects != "[]" {
		t.Fatalf("allowed defaults = %s / %s, want [] / []", q.createAgentArg.AllowedTools, q.createAgentArg.AllowedProjects)
	}
	if q.createAgentArg.RiskLevel != "medium" || q.createAgentArg.Status != "active" {
		t.Fatalf("defaults = risk %q status %q, want medium/active", q.createAgentArg.RiskLevel, q.createAgentArg.Status)
	}

	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["runtime_id"] != "codex" {
		t.Fatalf("response runtime_id = %v, want codex", out["runtime_id"])
	}
	if out["is_lead"] != false || out["is_meta"] != false {
		t.Fatalf("agent meta flags = %v / %v, want false / false", out["is_lead"], out["is_meta"])
	}
}

func TestCreateAgentRequiresRuntime(t *testing.T) {
	h := NewRegistriesHandler(&fakeRegistriesStore{})
	req := httptest.NewRequest(http.MethodPost, "/agents", strings.NewReader(`{"slug":"builder-web","name":"Builder Web"}`))
	res := httptest.NewRecorder()

	h.CreateAgent(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}
