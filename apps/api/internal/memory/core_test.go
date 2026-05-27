package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestCore(t *testing.T) *Core {
	t.Helper()

	core, err := Open(filepath.Join(t.TempDir(), "nested", "memory", "battos.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := core.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return core
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	core := openTestCore(t)
	if err := core.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestSaveUpsertSearchAndStats(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	first, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core inicial",
		Content:   "SQLite FTS5 inicial",
		TopicKey:  "battos/memory-core",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save(first) error = %v", err)
	}

	updated, err := core.Save(ctx, Observation{
		Type:      TypeArchitecture,
		Title:     "Memory Core consolidado",
		Content:   "SQLite FTS5 listo para fase dos",
		TopicKey:  "battos/memory-core",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save(updated) error = %v", err)
	}
	if updated.ID != first.ID {
		t.Fatalf("upsert generated id %d, want %d", updated.ID, first.ID)
	}

	results, err := core.Search(ctx, "consolidado", SearchFilter{ProjectID: "battos", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 || results[0].Title != "Memory Core consolidado" {
		t.Fatalf("Search() results = %#v, want updated observation", results)
	}

	stats, err := core.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalItems != 1 {
		t.Fatalf("Stats().TotalItems = %d, want 1", stats.TotalItems)
	}
}

func TestSearchWithoutTextAppliesFilters(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	for _, obs := range []Observation{
		{Type: TypeDecision, Title: "BattOS decision", Content: "keep", ProjectID: "battos", Scope: ScopeProject},
		{Type: TypePattern, Title: "BattOS pattern", Content: "skip type", ProjectID: "battos", Scope: ScopeProject},
		{Type: TypeDecision, Title: "Other decision", Content: "skip project", ProjectID: "other", Scope: ScopeProject},
	} {
		if _, err := core.Save(ctx, obs); err != nil {
			t.Fatalf("Save(%q) error = %v", obs.Title, err)
		}
	}

	results, err := core.Search(ctx, "", SearchFilter{
		Type:      TypeDecision,
		ProjectID: "battos",
		Scope:     ScopeProject,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 || results[0].Title != "BattOS decision" {
		t.Fatalf("Search() results = %#v, want only filtered decision", results)
	}
}
