package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type UsageStore interface {
	GetUsageOverview(context.Context) ([]store.GetUsageOverviewRow, error)
	GetUsageByRun(context.Context, pgtype.UUID) ([]store.UsageEvent, error)
	GetRun(context.Context, pgtype.UUID) (store.Run, error)
}

type UsageHandler struct {
	store UsageStore
}

func NewUsageHandler(s UsageStore) *UsageHandler {
	return &UsageHandler{store: s}
}

type usageOverviewResponse struct {
	ProjectID               string  `json:"project_id"`
	ProjectName             string  `json:"project_name"`
	ProjectMonthlyBudgetUSD float64 `json:"project_monthly_budget_usd"`
	AgentID                 string  `json:"agent_id"`
	ModelID                 string  `json:"model_id"`
	ProviderID              string  `json:"provider_id"`
	TotalInputTokens        int64   `json:"total_input_tokens"`
	TotalOutputTokens       int64   `json:"total_output_tokens"`
	TotalCachedTokens       int64   `json:"total_cached_tokens"`
	TotalRequests           int64   `json:"total_requests"`
	TotalCostUSD            float64 `json:"total_cost_usd"`
	CostPrecision           string  `json:"cost_precision"`
}

type usageEventResponse struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	ProviderID       string    `json:"provider_id"`
	ModelID          string    `json:"model_id"`
	ProjectID        string    `json:"project_id"`
	AgentID          string    `json:"agent_id"`
	SkillID          string    `json:"skill_id"`
	InputTokens      int32     `json:"input_tokens"`
	OutputTokens     int32     `json:"output_tokens"`
	CachedTokens     int32     `json:"cached_tokens"`
	RequestCount     int32     `json:"request_count"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	CreatedAt        time.Time `json:"created_at"`
}

func (h *UsageHandler) Overview(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.GetUsageOverview(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}

	out := make([]usageOverviewResponse, 0, len(items))
	for _, item := range items {
		cost, _ := item.TotalCostUsd.Float64Value()
		budget, _ := item.ProjectMonthlyBudgetUsd.Float64Value()
		costPrecision := "not_reported"
		if cost.Float64 > 0 {
			costPrecision = "estimated"
		}
		out = append(out, usageOverviewResponse{
			ProjectID:               textValue(item.ProjectID),
			ProjectName:             item.ProjectName,
			ProjectMonthlyBudgetUSD: budget.Float64,
			AgentID:                 textValue(item.AgentID),
			ModelID:                 textValue(item.ModelID),
			ProviderID:              textValue(item.ProviderID),
			TotalInputTokens:        item.TotalInputTokens,
			TotalOutputTokens:       item.TotalOutputTokens,
			TotalCachedTokens:       item.TotalCachedTokens,
			TotalRequests:           item.TotalRequests,
			TotalCostUSD:            cost.Float64,
			CostPrecision:           costPrecision,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *UsageHandler) GetUsageByRun(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "id")
	runID, ok := parseUUIDInput(w, runIDStr, "id")
	if !ok {
		return
	}

	_, errRun := h.store.GetRun(r.Context(), runID)
	if errRun != nil {
		if errors.Is(errRun, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "run no encontrada", "code": 404}})
			return
		}
		writeWorkError(w, errRun)
		return
	}

	events, err := h.store.GetUsageByRun(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}

	out := make([]usageEventResponse, 0, len(events))
	for _, item := range events {
		cost, _ := item.EstimatedCostUsd.Float64Value()
		out = append(out, usageEventResponse{
			ID:               uuidValue(item.ID),
			RunID:            uuidValue(item.RunID),
			ProviderID:       textValue(item.ProviderID),
			ModelID:          textValue(item.ModelID),
			ProjectID:        textValue(item.ProjectID),
			AgentID:          textValue(item.AgentID),
			SkillID:          textValue(item.SkillID),
			InputTokens:      item.InputTokens,
			OutputTokens:     item.OutputTokens,
			CachedTokens:     item.CachedTokens,
			RequestCount:     item.RequestCount,
			EstimatedCostUSD: cost.Float64,
			CreatedAt:        item.CreatedAt.Time,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
