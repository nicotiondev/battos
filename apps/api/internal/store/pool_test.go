package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenDBInitializesUnifiedSQLiteSchema(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "battos.db")

	db, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `INSERT INTO memory_items (type, title, content) VALUES ('note', 'SQLite', 'FTS works')`); err != nil {
		t.Fatalf("insert memory item: %v", err)
	}

	var hits int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_items_fts WHERE memory_items_fts MATCH 'FTS'`).Scan(&hits); err != nil {
		t.Fatalf("query memory FTS: %v", err)
	}
	if hits != 1 {
		t.Fatalf("FTS hits = %d, want 1", hits)
	}

	var runtimes int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_runtimes`).Scan(&runtimes); err != nil {
		t.Fatalf("query seeded runtimes: %v", err)
	}
	if runtimes == 0 {
		t.Fatalf("seeded runtimes = 0, want at least one")
	}
}

func TestOpenDBSupportsGeneratedRegistryQueries(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "battos.db")

	db, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	q := New(db)
	runtimes, err := q.ListAgentRuntimes(ctx)
	if err != nil {
		t.Fatalf("ListAgentRuntimes: %v", err)
	}
	if len(runtimes) == 0 {
		t.Fatalf("ListAgentRuntimes returned no seeded runtimes")
	}

	if _, err := q.CountAvailableRuntimes(ctx); err != nil {
		t.Fatalf("CountAvailableRuntimes: %v", err)
	}

	agent, err := q.CreateAgent(ctx, CreateAgentParams{
		ID:              "sqlite-smoke-agent",
		Slug:            "sqlite-smoke-agent",
		Name:            "SQLite Smoke Agent",
		Role:            sql.NullString{String: "smoke", Valid: true},
		RuntimeID:       sql.NullString{String: "sandbox-smoke", Valid: true},
		RuntimeConfig:   "{}",
		AllowedTools:    "[]",
		AllowedProjects: "[]",
		RiskLevel:       "low",
		Status:          "active",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if agent.ID != "sqlite-smoke-agent" {
		t.Fatalf("agent ID = %q, want sqlite-smoke-agent", agent.ID)
	}

	agents, err := q.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) == 0 {
		t.Fatalf("ListAgents returned no agents after CreateAgent")
	}
}
