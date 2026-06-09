package store

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// seedKnowledgeProject creates a project for knowledge tests.
func seedKnowledgeProject(t *testing.T, ctx context.Context, q *Queries, id string) string {
	t.Helper()
	proj, err := q.CreateProject(ctx, CreateProjectParams{
		ID: id, Slug: id, Name: "Knowledge Project " + id,
		Status: "active", Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("seedKnowledgeProject: CreateProject %s: %v", id, err)
	}
	return proj.ID
}

func TestCreateKnowledgeWorkspaceAndGetAndList(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projID := seedKnowledgeProject(t, ctx, q, "proj-kws-1")

	ws, err := q.CreateKnowledgeWorkspace(ctx, CreateKnowledgeWorkspaceParams{
		ProjectID: projID,
		Name:      "Wiki 1",
		Layout:    "raw_wiki_outputs",
		Status:    "active",
		Metadata:  "{}",
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeWorkspace: %v", err)
	}
	if ws.ID == "" {
		t.Fatal("CreateKnowledgeWorkspace: got empty ID")
	}
	if ws.ProjectID != projID {
		t.Errorf("CreateKnowledgeWorkspace: project_id = %q, want %q", ws.ProjectID, projID)
	}
	if ws.Name != "Wiki 1" {
		t.Errorf("CreateKnowledgeWorkspace: name = %q, want 'Wiki 1'", ws.Name)
	}
	if ws.Layout != "raw_wiki_outputs" {
		t.Errorf("CreateKnowledgeWorkspace: layout = %q, want raw_wiki_outputs", ws.Layout)
	}
	if ws.Status != "active" {
		t.Errorf("CreateKnowledgeWorkspace: status = %q, want active", ws.Status)
	}

	got, err := q.GetKnowledgeWorkspace(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeWorkspace: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("GetKnowledgeWorkspace: id = %q, want %q", got.ID, ws.ID)
	}
	if got.Name != "Wiki 1" {
		t.Errorf("GetKnowledgeWorkspace: name = %q", got.Name)
	}
	if got.ProjectID != projID {
		t.Errorf("GetKnowledgeWorkspace: project_id = %q, want %q", got.ProjectID, projID)
	}

	// ListKnowledgeWorkspaces — must include our workspace.
	list, err := q.ListKnowledgeWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListKnowledgeWorkspaces: %v", err)
	}
	var found bool
	for _, w := range list {
		if w.ID == ws.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListKnowledgeWorkspaces: workspace %s not in list of %d", ws.ID, len(list))
	}
}

func TestCreateJournalAndGetAndListByProject(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projID := seedKnowledgeProject(t, ctx, q, "proj-journal-1")

	// A workspace is required (FK: journals.workspace_id → knowledge_workspaces.id)
	ws, err := q.CreateKnowledgeWorkspace(ctx, CreateKnowledgeWorkspaceParams{
		ProjectID: projID,
		Name:      "Journal Workspace",
		Layout:    "raw_wiki_outputs",
		Status:    "active",
		Metadata:  "{}",
	})
	if err != nil {
		t.Fatalf("CreateKnowledgeWorkspace: %v", err)
	}

	journalDate := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	created, err := q.CreateJournal(ctx, CreateJournalParams{
		WorkspaceID: ws.ID,
		ProjectID:   projID,
		Title:       "Day 1 notes",
		Content:     "We did a lot today.",
		JournalDate: journalDate,
	})
	if err != nil {
		t.Fatalf("CreateJournal: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateJournal: got empty ID")
	}
	if created.WorkspaceID != ws.ID {
		t.Errorf("CreateJournal: workspace_id = %q, want %q", created.WorkspaceID, ws.ID)
	}
	if created.ProjectID != projID {
		t.Errorf("CreateJournal: project_id = %q, want %q", created.ProjectID, projID)
	}
	if created.Title != "Day 1 notes" {
		t.Errorf("CreateJournal: title = %q, want 'Day 1 notes'", created.Title)
	}
	if created.Content != "We did a lot today." {
		t.Errorf("CreateJournal: content = %q", created.Content)
	}

	got, err := q.GetJournal(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJournal: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetJournal: id = %q, want %q", got.ID, created.ID)
	}
	if got.Title != "Day 1 notes" {
		t.Errorf("GetJournal: title = %q", got.Title)
	}
	if got.Content != "We did a lot today." {
		t.Errorf("GetJournal: content = %q", got.Content)
	}
	// journal_date round-trip
	if !got.JournalDate.Truncate(24 * time.Hour).Equal(journalDate.Truncate(24 * time.Hour)) {
		t.Errorf("GetJournal: journal_date = %v, want %v", got.JournalDate, journalDate)
	}

	// ListJournalsByProject
	journals, err := q.ListJournalsByProject(ctx, projID)
	if err != nil {
		t.Fatalf("ListJournalsByProject: %v", err)
	}
	if len(journals) != 1 {
		t.Fatalf("ListJournalsByProject: got %d, want 1", len(journals))
	}
	if journals[0].ID != created.ID {
		t.Errorf("ListJournalsByProject[0].ID = %q, want %q", journals[0].ID, created.ID)
	}
}

func TestCreateArtifactAndGetAndListByProject(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projID := seedKnowledgeProject(t, ctx, q, "proj-artifact-1")

	created, err := q.CreateArtifact(ctx, CreateArtifactParams{
		ProjectID: projID,
		Name:      "design-doc.md",
		Kind:      "markdown",
		Content:   sql.NullString{String: "# Design\nContent here.", Valid: true},
		Metadata:  "{}",
	})
	if err != nil {
		t.Fatalf("CreateArtifact: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateArtifact: got empty ID")
	}
	if created.ProjectID != projID {
		t.Errorf("CreateArtifact: project_id = %q, want %q", created.ProjectID, projID)
	}
	if created.Name != "design-doc.md" {
		t.Errorf("CreateArtifact: name = %q, want design-doc.md", created.Name)
	}
	if created.Kind != "markdown" {
		t.Errorf("CreateArtifact: kind = %q, want markdown", created.Kind)
	}
	if !created.Content.Valid || created.Content.String != "# Design\nContent here." {
		t.Errorf("CreateArtifact: content = %v", created.Content)
	}

	got, err := q.GetArtifact(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetArtifact: id = %q, want %q", got.ID, created.ID)
	}
	if got.Name != "design-doc.md" {
		t.Errorf("GetArtifact: name = %q", got.Name)
	}
	if !got.Content.Valid || got.Content.String != "# Design\nContent here." {
		t.Errorf("GetArtifact: content = %v", got.Content)
	}

	// ListArtifactsByProject
	artifacts, err := q.ListArtifactsByProject(ctx, projID)
	if err != nil {
		t.Fatalf("ListArtifactsByProject: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("ListArtifactsByProject: got %d, want 1", len(artifacts))
	}
	if artifacts[0].ID != created.ID {
		t.Errorf("ListArtifactsByProject[0].ID = %q, want %q", artifacts[0].ID, created.ID)
	}
}

func TestListArtifactsByRun(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projID := seedKnowledgeProject(t, ctx, q, "proj-artifact-run-1")

	runID := "fake-run-id-for-artifact-test"

	// Artifact linked to a specific run_id (run_id in artifacts is just TEXT, no FK to runs)
	art, err := q.CreateArtifact(ctx, CreateArtifactParams{
		ProjectID: projID,
		RunID:     sql.NullString{String: runID, Valid: true},
		Name:      "build-report.md",
		Kind:      "build_report",
		Content:   sql.NullString{String: "Build OK", Valid: true},
		Metadata:  "{}",
	})
	if err != nil {
		t.Fatalf("CreateArtifact with run_id: %v", err)
	}

	artifacts, err := q.ListArtifactsByRun(ctx, sql.NullString{String: runID, Valid: true})
	if err != nil {
		t.Fatalf("ListArtifactsByRun: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("ListArtifactsByRun: got %d, want 1", len(artifacts))
	}
	if artifacts[0].ID != art.ID {
		t.Errorf("ListArtifactsByRun[0].ID = %q, want %q", artifacts[0].ID, art.ID)
	}
	if !artifacts[0].RunID.Valid || artifacts[0].RunID.String != runID {
		t.Errorf("ListArtifactsByRun[0].RunID = %v, want %s", artifacts[0].RunID, runID)
	}
}
