package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeUsageStore struct {
	overview []store.GetUsageOverviewRow
}

func (s fakeUsageStore) GetUsageOverview(context.Context) ([]store.GetUsageOverviewRow, error) {
	return s.overview, nil
}

func (s fakeUsageStore) GetUsageByRun(context.Context, sql.NullString) ([]store.UsageEvent, error) {
	return nil, nil
}

func (s fakeUsageStore) GetRun(context.Context, string) (store.Run, error) {
	return store.Run{}, nil
}

func TestUsageOverviewIncludesBudgetAndPrecision(t *testing.T) {
	handler := NewUsageHandler(fakeUsageStore{overview: []store.GetUsageOverviewRow{
		{
			ProjectID:               sql.NullString{String: "landing-acme", Valid: true},
			ProjectName:             "Landing Acme",
			ProjectMonthlyBudgetUsd: sql.NullFloat64{Float64: 25.50, Valid: true},
			AgentID:                 sql.NullString{String: "nova", Valid: true},
			ModelID:                 sql.NullString{String: "gpt-4o", Valid: true},
			ProviderID:              sql.NullString{String: "openai", Valid: true},
			TotalInputTokens:        sql.NullFloat64{Float64: 1000, Valid: true},
			TotalOutputTokens:       sql.NullFloat64{Float64: 250, Valid: true},
			TotalCachedTokens:       sql.NullFloat64{Float64: 100, Valid: true},
			TotalRequests:           sql.NullFloat64{Float64: 3, Valid: true},
			TotalCostUsd:            1.234567,
		},
		{
			ProjectID:               sql.NullString{String: "research", Valid: true},
			ProjectName:             "Research",
			ProjectMonthlyBudgetUsd: sql.NullFloat64{Float64: 0, Valid: true},
			AgentID:                 sql.NullString{String: "analyst", Valid: true},
			ModelID:                 sql.NullString{String: "unknown", Valid: true},
			ProviderID:              sql.NullString{String: "unknown", Valid: true},
			TotalInputTokens:        sql.NullFloat64{Float64: 10, Valid: true},
			TotalOutputTokens:       sql.NullFloat64{Float64: 5, Valid: true},
			TotalCachedTokens:       sql.NullFloat64{Float64: 0, Valid: true},
			TotalRequests:           sql.NullFloat64{Float64: 1, Valid: true},
			TotalCostUsd:            0,
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/usage/overview", nil)
	rec := httptest.NewRecorder()

	handler.Overview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []usageOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body) != 2 {
		t.Fatalf("len(body) = %d, want 2", len(body))
	}
	if body[0].ProjectName != "Landing Acme" {
		t.Fatalf("project_name = %q", body[0].ProjectName)
	}
	if body[0].ProjectMonthlyBudgetUSD != 25.50 {
		t.Fatalf("project_monthly_budget_usd = %f", body[0].ProjectMonthlyBudgetUSD)
	}
	if body[0].CostPrecision != "estimated" {
		t.Fatalf("cost_precision = %q, want estimated", body[0].CostPrecision)
	}
	if body[1].CostPrecision != "not_reported" {
		t.Fatalf("cost_precision = %q, want not_reported", body[1].CostPrecision)
	}
}
