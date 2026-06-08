package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type UsageStore interface {
	GetUsageOverview(context.Context) ([]store.GetUsageOverviewRow, error)
	GetUsageByRun(context.Context, sql.NullString) ([]store.UsageEvent, error)
	GetRun(context.Context, string) (store.Run, error)
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
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CachedTokens     int64     `json:"cached_tokens"`
	RequestCount     int64     `json:"request_count"`
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
		cost := interfaceFloat64(item.TotalCostUsd)
		budget := nullFloat64(item.ProjectMonthlyBudgetUsd)
		costPrecision := "not_reported"
		if cost > 0 {
			costPrecision = "estimated"
		}
		out = append(out, usageOverviewResponse{
			ProjectID:               textValue(item.ProjectID),
			ProjectName:             item.ProjectName,
			ProjectMonthlyBudgetUSD: budget,
			AgentID:                 textValue(item.AgentID),
			ModelID:                 textValue(item.ModelID),
			ProviderID:              textValue(item.ProviderID),
			TotalInputTokens:        int64(nullFloat64(item.TotalInputTokens)),
			TotalOutputTokens:       int64(nullFloat64(item.TotalOutputTokens)),
			TotalCachedTokens:       int64(nullFloat64(item.TotalCachedTokens)),
			TotalRequests:           int64(nullFloat64(item.TotalRequests)),
			TotalCostUSD:            cost,
			CostPrecision:           costPrecision,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *UsageHandler) GetUsageByRun(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "id")
	runID, ok := parseIDInput(w, runIDStr, "id")
	if !ok {
		return
	}

	_, errRun := h.store.GetRun(r.Context(), runID)
	if errRun != nil {
		if errors.Is(errRun, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "run no encontrada", "code": 404}})
			return
		}
		writeWorkError(w, errRun)
		return
	}

	events, err := h.store.GetUsageByRun(r.Context(), sql.NullString{String: runID, Valid: true})
	if err != nil {
		writeWorkError(w, err)
		return
	}

	out := make([]usageEventResponse, 0, len(events))
	for _, item := range events {
		out = append(out, usageEventResponse{
			ID:               item.ID,
			RunID:            textValue(item.RunID),
			ProviderID:       textValue(item.ProviderID),
			ModelID:          textValue(item.ModelID),
			ProjectID:        textValue(item.ProjectID),
			AgentID:          textValue(item.AgentID),
			SkillID:          textValue(item.SkillID),
			InputTokens:      item.InputTokens,
			OutputTokens:     item.OutputTokens,
			CachedTokens:     item.CachedTokens,
			RequestCount:     item.RequestCount,
			EstimatedCostUSD: item.EstimatedCostUsd,
			CreatedAt:        item.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func nullFloat64(value sql.NullFloat64) float64 {
	if value.Valid {
		return value.Float64
	}
	return 0
}

func interfaceFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case []byte:
		var parsed float64
		_, _ = fmt.Sscanf(string(v), "%f", &parsed)
		return parsed
	case string:
		var parsed float64
		_, _ = fmt.Sscanf(v, "%f", &parsed)
		return parsed
	default:
		return 0
	}
}
