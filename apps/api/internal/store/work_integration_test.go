package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestCreateDomainAndGetDomain(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	created, err := q.CreateDomain(ctx, CreateDomainParams{
		ID:          "dom-test-1",
		Slug:        "dom-test-1",
		Name:        "Test Domain",
		Description: sql.NullString{String: "A test domain", Valid: true},
		Status:      "active",
		Metadata:    "{}",
	})
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if created.ID != "dom-test-1" {
		t.Errorf("CreateDomain: id = %q, want dom-test-1", created.ID)
	}
	if created.Slug != "dom-test-1" {
		t.Errorf("CreateDomain: slug = %q, want dom-test-1", created.Slug)
	}
	if created.Name != "Test Domain" {
		t.Errorf("CreateDomain: name = %q, want 'Test Domain'", created.Name)
	}
	if !created.Description.Valid || created.Description.String != "A test domain" {
		t.Errorf("CreateDomain: description = %v, want 'A test domain'", created.Description)
	}
	if created.Status != "active" {
		t.Errorf("CreateDomain: status = %q, want active", created.Status)
	}

	got, err := q.GetDomain(ctx, "dom-test-1")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetDomain: id = %q, want %q", got.ID, created.ID)
	}
	if got.Name != "Test Domain" {
		t.Errorf("GetDomain: name = %q, want 'Test Domain'", got.Name)
	}
}

func TestListDomains(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	if _, err := q.CreateDomain(ctx, CreateDomainParams{
		ID: "dom-list-1", Slug: "dom-list-1", Name: "List Domain 1",
		Status: "active", Metadata: "{}",
	}); err != nil {
		t.Fatalf("CreateDomain 1: %v", err)
	}
	if _, err := q.CreateDomain(ctx, CreateDomainParams{
		ID: "dom-list-2", Slug: "dom-list-2", Name: "List Domain 2",
		Status: "active", Metadata: "{}",
	}); err != nil {
		t.Fatalf("CreateDomain 2: %v", err)
	}

	domains, err := q.ListDomains(ctx)
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) < 2 {
		t.Errorf("ListDomains: got %d, want at least 2", len(domains))
	}
}

func TestCreateProjectAndGetProject(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	created, err := q.CreateProject(ctx, CreateProjectParams{
		ID:          "proj-get-test",
		Slug:        "proj-get-test",
		Name:        "Get Test Project",
		Description: sql.NullString{String: "desc", Valid: true},
		Status:      "active",
		Metadata:    `{"key":"value"}`,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.ID != "proj-get-test" {
		t.Errorf("CreateProject: id = %q, want proj-get-test", created.ID)
	}
	if created.Slug != "proj-get-test" {
		t.Errorf("CreateProject: slug = %q, want proj-get-test", created.Slug)
	}
	if created.Name != "Get Test Project" {
		t.Errorf("CreateProject: name = %q, want 'Get Test Project'", created.Name)
	}
	if !created.Description.Valid || created.Description.String != "desc" {
		t.Errorf("CreateProject: description = %v", created.Description)
	}
	if created.Status != "active" {
		t.Errorf("CreateProject: status = %q, want active", created.Status)
	}
	if created.Metadata != `{"key":"value"}` {
		t.Errorf("CreateProject: metadata = %q", created.Metadata)
	}

	got, err := q.GetProject(ctx, "proj-get-test")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.ID != "proj-get-test" {
		t.Errorf("GetProject: id = %q, want proj-get-test", got.ID)
	}
	if got.Name != "Get Test Project" {
		t.Errorf("GetProject: name = %q", got.Name)
	}
}

func TestListProjects(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	for _, id := range []string{"proj-list-a", "proj-list-b"} {
		if _, err := q.CreateProject(ctx, CreateProjectParams{
			ID: id, Slug: id, Name: id, Status: "active", Metadata: "{}",
		}); err != nil {
			t.Fatalf("CreateProject %s: %v", id, err)
		}
	}

	projects, err := q.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) < 2 {
		t.Errorf("ListProjects: got %d, want at least 2", len(projects))
	}
}

func TestCreateGoalAndGetGoalAndListGoals(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	proj, err := q.CreateProject(ctx, CreateProjectParams{
		ID: "proj-goal-test", Slug: "proj-goal-test", Name: "Goal Test Project",
		Status: "active", Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	created, err := q.CreateGoal(ctx, CreateGoalParams{
		ProjectID:   proj.ID,
		Title:       "Ship MVP",
		Description: sql.NullString{String: "The first milestone", Valid: true},
		Status:      "planned",
		Metadata:    "{}",
	})
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateGoal: got empty ID")
	}
	if created.ProjectID != proj.ID {
		t.Errorf("CreateGoal: project_id = %q, want %q", created.ProjectID, proj.ID)
	}
	if created.Title != "Ship MVP" {
		t.Errorf("CreateGoal: title = %q, want 'Ship MVP'", created.Title)
	}
	if !created.Description.Valid || created.Description.String != "The first milestone" {
		t.Errorf("CreateGoal: description = %v", created.Description)
	}
	if created.Status != "planned" {
		t.Errorf("CreateGoal: status = %q, want planned", created.Status)
	}

	got, err := q.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetGoal: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetGoal: id = %q, want %q", got.ID, created.ID)
	}
	if got.Title != "Ship MVP" {
		t.Errorf("GetGoal: title = %q, want 'Ship MVP'", got.Title)
	}

	// ListGoalsByProject
	goals, err := q.ListGoalsByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListGoalsByProject: %v", err)
	}
	if len(goals) != 1 {
		t.Errorf("ListGoalsByProject: got %d, want 1", len(goals))
	}
	if goals[0].ID != created.ID {
		t.Errorf("ListGoalsByProject[0].ID = %q, want %q", goals[0].ID, created.ID)
	}
}

func TestCreateTaskAndGetTaskAndListTasks(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	proj, err := q.CreateProject(ctx, CreateProjectParams{
		ID: "proj-task-test", Slug: "proj-task-test", Name: "Task Test Project",
		Status: "active", Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	created, err := q.CreateTask(ctx, CreateTaskParams{
		ProjectID:     proj.ID,
		Title:         "Implement feature X",
		Description:   sql.NullString{String: "details here", Valid: true},
		Status:        "backlog",
		BoardPosition: 10,
		Metadata:      "{}",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateTask: got empty ID")
	}
	if created.ProjectID != proj.ID {
		t.Errorf("CreateTask: project_id = %q, want %q", created.ProjectID, proj.ID)
	}
	if created.Title != "Implement feature X" {
		t.Errorf("CreateTask: title = %q, want 'Implement feature X'", created.Title)
	}
	if !created.Description.Valid || created.Description.String != "details here" {
		t.Errorf("CreateTask: description = %v", created.Description)
	}
	if created.Status != "backlog" {
		t.Errorf("CreateTask: status = %q, want backlog", created.Status)
	}
	if created.BoardPosition != 10 {
		t.Errorf("CreateTask: board_position = %d, want 10", created.BoardPosition)
	}

	got, err := q.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetTask: id = %q, want %q", got.ID, created.ID)
	}
	if got.Title != "Implement feature X" {
		t.Errorf("GetTask: title = %q", got.Title)
	}
	if got.BoardPosition != 10 {
		t.Errorf("GetTask: board_position = %d, want 10", got.BoardPosition)
	}

	// ListTasksByProject
	tasks, err := q.ListTasksByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListTasksByProject: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("ListTasksByProject: got %d, want 1", len(tasks))
	}
	if tasks[0].ID != created.ID {
		t.Errorf("ListTasksByProject[0].ID = %q, want %q", tasks[0].ID, created.ID)
	}
}

// TestEnsureInboxProjectUpsertIdempotency verifies that calling EnsureInboxProject
// twice returns the same row without error (ON CONFLICT DO UPDATE idempotency).
func TestEnsureInboxProjectUpsertIdempotency(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	first, err := q.EnsureInboxProject(ctx)
	if err != nil {
		t.Fatalf("EnsureInboxProject (first): %v", err)
	}
	if first.ID != "inbox" {
		t.Errorf("EnsureInboxProject: id = %q, want inbox", first.ID)
	}
	if first.Slug != "inbox" {
		t.Errorf("EnsureInboxProject: slug = %q, want inbox", first.Slug)
	}
	if first.Status != "active" {
		t.Errorf("EnsureInboxProject: status = %q, want active", first.Status)
	}

	// Second call must be a no-op upsert returning the same ID.
	second, err := q.EnsureInboxProject(ctx)
	if err != nil {
		t.Fatalf("EnsureInboxProject (second): %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("EnsureInboxProject idempotency: second.ID = %q, want %q", second.ID, first.ID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Errorf("EnsureInboxProject idempotency: created_at changed between calls")
	}
}
