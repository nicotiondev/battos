package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type RegistriesStore interface {
	CreateAgent(context.Context, store.CreateAgentParams) (store.Agent, error)
	ListAgents(context.Context) ([]store.Agent, error)
	ListSkills(context.Context) ([]store.Skill, error)
}

type RegistriesHandler struct {
	store RegistriesStore
}

func NewRegistriesHandler(q RegistriesStore) *RegistriesHandler {
	return &RegistriesHandler{store: q}
}

type agentResponse struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Role         string    `json:"role,omitempty"`
	Description  string    `json:"description,omitempty"`
	RuntimeID    string    `json:"runtime_id,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	RiskLevel    string    `json:"risk_level"`
	IsLead       bool      `json:"is_lead"`
	IsMeta       bool      `json:"is_meta"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type agentInput struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	RuntimeID    string `json:"runtime_id"`
	SystemPrompt string `json:"system_prompt"`
	RiskLevel    string `json:"risk_level"`
	Status       string `json:"status"`
}

type skillResponse struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category,omitempty"`
	RiskLevel      string    `json:"risk_level"`
	Version        string    `json:"version,omitempty"`
	Status         string    `json:"status"`
	Lifecycle      string    `json:"lifecycle"`
	PromptTemplate string    `json:"prompt_template,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (h *RegistriesHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAgents(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]agentResponse, 0, len(items))
	for _, item := range items {
		out = append(out, agentDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RegistriesHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var in agentInput
	if !decodeWorkInput(w, r, &in) ||
		!required(w, in.Slug, "slug") ||
		!required(w, in.Name, "name") ||
		!required(w, in.RuntimeID, "runtime_id") {
		return
	}
	item, err := h.store.CreateAgent(r.Context(), store.CreateAgentParams{
		ID:              in.Slug,
		Slug:            in.Slug,
		Name:            in.Name,
		Role:            nullableText(in.Role),
		Description:     nullableText(in.Description),
		RuntimeID:       nullableText(in.RuntimeID),
		RuntimeConfig:   []byte("{}"),
		SystemPrompt:    nullableText(in.SystemPrompt),
		AllowedTools:    []byte("[]"),
		AllowedProjects: []byte("[]"),
		RiskLevel:       defaultString(in.RiskLevel, "medium"),
		Status:          defaultString(in.Status, "active"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agentDTO(item))
}

func (h *RegistriesHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListSkills(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]skillResponse, 0, len(items))
	for _, item := range items {
		out = append(out, skillDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func agentDTO(item store.Agent) agentResponse {
	return agentResponse{
		ID:           item.ID,
		Slug:         item.Slug,
		Name:         item.Name,
		Role:         textValue(item.Role),
		Description:  textValue(item.Description),
		RuntimeID:    textValue(item.RuntimeID),
		SystemPrompt: textValue(item.SystemPrompt),
		RiskLevel:    item.RiskLevel,
		IsLead:       item.IsLead,
		IsMeta:       item.IsMeta,
		Status:       item.Status,
		CreatedAt:    timeValue(item.CreatedAt),
		UpdatedAt:    timeValue(item.UpdatedAt),
	}
}

func skillDTO(item store.Skill) skillResponse {
	return skillResponse{
		ID:             item.ID,
		Slug:           item.Slug,
		Name:           item.Name,
		Description:    textValue(item.Description),
		Category:       textValue(item.Category),
		RiskLevel:      item.RiskLevel,
		Version:        textValue(item.Version),
		Status:         item.Status,
		Lifecycle:      item.Lifecycle,
		PromptTemplate: textValue(item.PromptTemplate),
		CreatedAt:      timeValue(item.CreatedAt),
		UpdatedAt:      timeValue(item.UpdatedAt),
	}
}
