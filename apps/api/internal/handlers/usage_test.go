package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeUsageStore struct {
	overview []store.GetUsageOverviewRow
}

func (s fakeUsageStore) GetUsageOverview(context.Context) ([]store.GetUsageOverviewRow, error) {
	return s.overview, nil
}

func (s fakeUsageStore) GetUsageByRun(context.Context, pgtype.UUID) ([]store.UsageEvent, error) {
	return nil, nil
}

func (s fakeUsageStore) GetRun(context.Context, pgtype.UUID) (store.Run, error) {
	return store.Run{}, nil
}

func numericValue(t *testing.T, value string) pgtype.Numeric {
	t.Helper()
	var out pgtype.Numeric
	if err := out.Scan(value); err != nil {
		t.Fatalf("scan numeric %q: %v", value, err)
	}
	return out
}

func TestUsageOverviewIncludesBudgetAndPrecision(t *testing.T) {
	handler := NewUsageHandler(fakeUsageStore{overview: []store.GetUsageOverviewRow{
		{
			ProjectID:               pgtype.Text{String: "landing-acme", Valid: true},
			ProjectName:             "Landing Acme",
			ProjectMonthlyBudgetUsd: numericValue(t, "25.50"),
			AgentID:                 pgtype.Text{String: "nova", Valid: true},
			ModelID:                 pgtype.Text{String: "gpt-4o", Valid: true},
			ProviderID:              pgtype.Text{String: "openai", Valid: true},
			TotalInputTokens:        1000,
			TotalOutputTokens:       250,
			TotalCachedTokens:       100,
			TotalRequests:           3,
			TotalCostUsd:            numericValue(t, "1.234567"),
		},
		{
			ProjectID:               pgtype.Text{String: "research", Valid: true},
			ProjectName:             "Research",
			ProjectMonthlyBudgetUsd: numericValue(t, "0"),
			AgentID:                 pgtype.Text{String: "analyst", Valid: true},
			ModelID:                 pgtype.Text{String: "unknown", Valid: true},
			ProviderID:              pgtype.Text{String: "unknown", Valid: true},
			TotalInputTokens:        10,
			TotalOutputTokens:       5,
			TotalCachedTokens:       0,
			TotalRequests:           1,
			TotalCostUsd:            numericValue(t, "0"),
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
