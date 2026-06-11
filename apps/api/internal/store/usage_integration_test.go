package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

func TestCreateUsageEventRoundTrip(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	// provider_id "anthropic" is seeded; model_id is left null to avoid
	// FK violation (no models are seeded by default).
	event, err := q.CreateUsageEvent(ctx, CreateUsageEventParams{
		ProviderID:       sql.NullString{String: "anthropic", Valid: true},
		InputTokens:      500,
		OutputTokens:     200,
		CachedTokens:     50,
		RequestCount:     1,
		EstimatedCostUsd: 0.0042,
	})
	if err != nil {
		t.Fatalf("CreateUsageEvent: %v", err)
	}
	if event.ID == "" {
		t.Fatal("CreateUsageEvent: got empty ID")
	}
	if !event.ProviderID.Valid || event.ProviderID.String != "anthropic" {
		t.Errorf("CreateUsageEvent: provider_id = %v, want anthropic", event.ProviderID)
	}
	if event.ModelID.Valid {
		t.Errorf("CreateUsageEvent: model_id should be null, got %v", event.ModelID)
	}
	if event.InputTokens != 500 {
		t.Errorf("CreateUsageEvent: input_tokens = %d, want 500", event.InputTokens)
	}
	if event.OutputTokens != 200 {
		t.Errorf("CreateUsageEvent: output_tokens = %d, want 200", event.OutputTokens)
	}
	if event.CachedTokens != 50 {
		t.Errorf("CreateUsageEvent: cached_tokens = %d, want 50", event.CachedTokens)
	}
	if event.RequestCount != 1 {
		t.Errorf("CreateUsageEvent: request_count = %d, want 1", event.RequestCount)
	}
	const wantCost = 0.0042
	const epsilon = 1e-9
	diff := event.EstimatedCostUsd - wantCost
	if diff < -epsilon || diff > epsilon {
		t.Errorf("CreateUsageEvent: estimated_cost_usd = %v, want %v", event.EstimatedCostUsd, wantCost)
	}
}

// TestCreateUsageEventWithSeededModel verifica que, con `models` ahora seedeado
// (gpt-4o, claude-3-5-sonnet), un usage_event con model_id ya NO viola la FK —
// que era el bug que el worker.recordUsage disparaba en cada run real.
func TestCreateUsageEventWithSeededModel(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	event, err := q.CreateUsageEvent(ctx, CreateUsageEventParams{
		ProviderID:       sql.NullString{String: "openai", Valid: true},
		ModelID:          sql.NullString{String: "gpt-4o", Valid: true},
		InputTokens:      120,
		OutputTokens:     40,
		RequestCount:     1,
		EstimatedCostUsd: 0.002,
	})
	if err != nil {
		t.Fatalf("CreateUsageEvent con model seeded: %v", err)
	}
	if !event.ModelID.Valid || event.ModelID.String != "gpt-4o" {
		t.Errorf("model_id = %v, want gpt-4o", event.ModelID)
	}
	if !event.ProviderID.Valid || event.ProviderID.String != "openai" {
		t.Errorf("provider_id = %v, want openai", event.ProviderID)
	}
}

func TestGetUsageOverviewAggregateSanity(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	// Seed a project so the JOIN in GetUsageOverview can resolve project_name.
	proj, err := q.CreateProject(ctx, CreateProjectParams{
		ID: "proj-usage-overview", Slug: "proj-usage-overview",
		Name: "Usage Overview Project", Status: "active", Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Insert three events with known totals.
	// provider_id "anthropic" is seeded; model_id left null (no seeded models).
	for i := 0; i < 3; i++ {
		if _, err := q.CreateUsageEvent(ctx, CreateUsageEventParams{
			ProjectID:        sql.NullString{String: proj.ID, Valid: true},
			ProviderID:       sql.NullString{String: "anthropic", Valid: true},
			InputTokens:      100,
			OutputTokens:     50,
			CachedTokens:     0,
			RequestCount:     1,
			EstimatedCostUsd: 0.001,
		}); err != nil {
			t.Fatalf("CreateUsageEvent %d: %v", i, err)
		}
	}

	rows, err := q.GetUsageOverview(ctx)
	if err != nil {
		t.Fatalf("GetUsageOverview: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("GetUsageOverview: returned 0 rows, expected at least 1")
	}

	// Find the row for our project.
	var found bool
	for _, row := range rows {
		if !row.ProjectID.Valid || row.ProjectID.String != proj.ID {
			continue
		}
		found = true

		// project_name must be non-empty.
		if row.ProjectName == "" {
			t.Errorf("GetUsageOverview: project_name empty for project %s", proj.ID)
		}

		// TotalInputTokens: 3 * 100 = 300
		if row.TotalInputTokens.Valid && row.TotalInputTokens.Float64 != 300 {
			t.Errorf("GetUsageOverview: total_input_tokens = %v, want 300", row.TotalInputTokens)
		}

		// TotalCostUsd is interface{}. Validate it is non-nil and represents ~0.003.
		// This is the "fragile" path flagged in the review.
		if row.TotalCostUsd == nil {
			t.Errorf("GetUsageOverview: total_cost_usd is nil")
		} else {
			// SQLite can return float64 or []byte depending on the driver.
			costVal := toFloat64(row.TotalCostUsd)
			const wantCost = 0.003
			const epsilon = 1e-6
			diff := costVal - wantCost
			if diff < -epsilon || diff > epsilon {
				t.Errorf("GetUsageOverview: total_cost_usd = %v (%T), want ~%v", row.TotalCostUsd, row.TotalCostUsd, wantCost)
			}
		}
	}
	if !found {
		t.Errorf("GetUsageOverview: no row found for project %s in %d rows", proj.ID, len(rows))
	}
}

// toFloat64 converts the interface{} value returned by GetUsageOverview.TotalCostUsd
// into a float64. SQLite + modernc driver returns float64 for COALESCE(SUM(...), 0).
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case []byte:
		var f float64
		fmt.Sscanf(string(val), "%f", &f)
		return f
	default:
		return 0
	}
}

func TestGetUsageByRun(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "usage-g")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "usage by run test",
		RequestedNetwork: 0,
		ExecutionMode:    "sandbox",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Two events linked to this run.
	for i := 0; i < 2; i++ {
		if _, err := q.CreateUsageEvent(ctx, CreateUsageEventParams{
			RunID:            sql.NullString{String: run.ID, Valid: true},
			InputTokens:      int64(100 + i*10),
			OutputTokens:     50,
			RequestCount:     1,
			EstimatedCostUsd: 0.001,
		}); err != nil {
			t.Fatalf("CreateUsageEvent %d: %v", i, err)
		}
	}

	events, err := q.GetUsageByRun(ctx, sql.NullString{String: run.ID, Valid: true})
	if err != nil {
		t.Fatalf("GetUsageByRun: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("GetUsageByRun: got %d events, want 2", len(events))
	}
	for _, ev := range events {
		if !ev.RunID.Valid || ev.RunID.String != run.ID {
			t.Errorf("GetUsageByRun: event run_id = %v, want %s", ev.RunID, run.ID)
		}
	}
}
