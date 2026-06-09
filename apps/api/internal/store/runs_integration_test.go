package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// openTestDB opens a fresh temp-file SQLite DB for integration tests.
// The returned cleanup func must be deferred by the caller.
func openTestDB(t *testing.T) (*Queries, func()) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "battos_test.db")
	db, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("openTestDB: OpenDB: %v", err)
	}
	q := New(db)
	return q, func() { db.Close() }
}

// seedRunFixtures inserts the minimum parent rows (project, agent, task) so
// that a Run can be created without FK violations. Returns (projectID,
// agentID, taskID). Uses unique IDs per test via a discriminator suffix.
func seedRunFixtures(t *testing.T, ctx context.Context, q *Queries, suffix string) (projectID, agentID, taskID string) {
	t.Helper()

	proj, err := q.CreateProject(ctx, CreateProjectParams{
		ID:       "proj-runs-" + suffix,
		Slug:     "proj-runs-" + suffix,
		Name:     "Runs Test Project " + suffix,
		Status:   "active",
		Metadata: "{}",
	})
	if err != nil {
		t.Fatalf("seedRunFixtures: CreateProject: %v", err)
	}

	// "sandbox-smoke" runtime is seeded by the schema; safe to reference.
	agent, err := q.CreateAgent(ctx, CreateAgentParams{
		ID:              "agent-runs-" + suffix,
		Slug:            "agent-runs-" + suffix,
		Name:            "Runs Test Agent " + suffix,
		RuntimeID:       sql.NullString{String: "sandbox-smoke", Valid: true},
		RuntimeConfig:   "{}",
		AllowedTools:    "[]",
		AllowedProjects: "[]",
		RiskLevel:       "low",
		Status:          "active",
	})
	if err != nil {
		t.Fatalf("seedRunFixtures: CreateAgent: %v", err)
	}

	task, err := q.CreateTask(ctx, CreateTaskParams{
		ProjectID:     proj.ID,
		Title:         "Runs Test Task " + suffix,
		Status:        "backlog",
		BoardPosition: 0,
		Metadata:      "{}",
	})
	if err != nil {
		t.Fatalf("seedRunFixtures: CreateTask: %v", err)
	}

	return proj.ID, agent.ID, task.ID
}

func TestCreateRunAndGetRun(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "a")

	created, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Do the thing",
		RequestedNetwork: 1,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if created.ID == "" {
		t.Fatal("CreateRun: got empty ID")
	}
	if created.Status != "awaiting_approval" {
		t.Errorf("CreateRun: status = %q, want awaiting_approval", created.Status)
	}
	if created.ProjectID != projectID {
		t.Errorf("CreateRun: project_id = %q, want %q", created.ProjectID, projectID)
	}
	if created.AgentID != agentID {
		t.Errorf("CreateRun: agent_id = %q, want %q", created.AgentID, agentID)
	}
	if created.RuntimeAdapterID != "sandbox-smoke" {
		t.Errorf("CreateRun: runtime_adapter_id = %q, want sandbox-smoke", created.RuntimeAdapterID)
	}
	if created.RequestedNetwork != 1 {
		t.Errorf("CreateRun: requested_network = %d, want 1", created.RequestedNetwork)
	}
	if created.NetworkEnabled != 0 {
		t.Errorf("CreateRun: network_enabled = %d, want 0", created.NetworkEnabled)
	}
	if created.HostSessionEnabled != 0 {
		t.Errorf("CreateRun: host_session_enabled = %d, want 0", created.HostSessionEnabled)
	}
	if created.Prompt != "Do the thing" {
		t.Errorf("CreateRun: prompt = %q, want 'Do the thing'", created.Prompt)
	}

	got, err := q.GetRun(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("GetRun: id = %q, want %q", got.ID, created.ID)
	}
	if got.ProjectID != projectID {
		t.Errorf("GetRun: project_id = %q, want %q", got.ProjectID, projectID)
	}
	if got.AgentID != agentID {
		t.Errorf("GetRun: agent_id = %q, want %q", got.AgentID, agentID)
	}
	if got.RuntimeAdapterID != "sandbox-smoke" {
		t.Errorf("GetRun: runtime_adapter_id = %q, want sandbox-smoke", got.RuntimeAdapterID)
	}
	if got.RequestedNetwork != 1 {
		t.Errorf("GetRun: requested_network = %d, want 1", got.RequestedNetwork)
	}
	if got.NetworkEnabled != 0 {
		t.Errorf("GetRun: network_enabled = %d, want 0", got.NetworkEnabled)
	}
	if got.HostSessionEnabled != 0 {
		t.Errorf("GetRun: host_session_enabled = %d, want 0", got.HostSessionEnabled)
	}
}

func TestEnableRunNetworkFlipsFlag(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "b")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Enable network test",
		RequestedNetwork: 1,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.NetworkEnabled != 0 {
		t.Fatalf("pre-condition: network_enabled = %d, want 0", run.NetworkEnabled)
	}

	updated, err := q.EnableRunNetwork(ctx, run.ID)
	if err != nil {
		t.Fatalf("EnableRunNetwork: %v", err)
	}
	if updated.NetworkEnabled != 1 {
		t.Errorf("EnableRunNetwork: network_enabled = %d, want 1", updated.NetworkEnabled)
	}
	if updated.HostSessionEnabled != 0 {
		t.Errorf("EnableRunNetwork: host_session_enabled should remain 0, got %d", updated.HostSessionEnabled)
	}

	got, err := q.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun after EnableRunNetwork: %v", err)
	}
	if got.NetworkEnabled != 1 {
		t.Errorf("GetRun after EnableRunNetwork: network_enabled = %d, want 1", got.NetworkEnabled)
	}
}

func TestEnableRunHostSessionFlipsFlag(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "c")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Enable host session test",
		RequestedNetwork: 0,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.HostSessionEnabled != 0 {
		t.Fatalf("pre-condition: host_session_enabled = %d, want 0", run.HostSessionEnabled)
	}

	updated, err := q.EnableRunHostSession(ctx, run.ID)
	if err != nil {
		t.Fatalf("EnableRunHostSession: %v", err)
	}
	if updated.HostSessionEnabled != 1 {
		t.Errorf("EnableRunHostSession: host_session_enabled = %d, want 1", updated.HostSessionEnabled)
	}
	if updated.NetworkEnabled != 0 {
		t.Errorf("EnableRunHostSession: network_enabled should remain 0, got %d", updated.NetworkEnabled)
	}

	got, err := q.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun after EnableRunHostSession: %v", err)
	}
	if got.HostSessionEnabled != 1 {
		t.Errorf("GetRun after EnableRunHostSession: host_session_enabled = %d, want 1", got.HostSessionEnabled)
	}
}

func TestUpdateRunStatus(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "d")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Status update test",
		RequestedNetwork: 0,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	updated, err := q.UpdateRunStatus(ctx, UpdateRunStatusParams{
		Status: "queued",
		ID:     run.ID,
	})
	if err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}
	if updated.Status != "queued" {
		t.Errorf("UpdateRunStatus: status = %q, want queued", updated.Status)
	}

	got, err := q.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun after UpdateRunStatus: %v", err)
	}
	if got.Status != "queued" {
		t.Errorf("GetRun after UpdateRunStatus: status = %q, want queued", got.Status)
	}
}

// TestClaimNextQueuedRunAndDoubleClaimGuard validates the AND status='queued'
// guard: after a successful claim the same run cannot be claimed again.
func TestClaimNextQueuedRunAndDoubleClaimGuard(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "e")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Claim test run",
		RequestedNetwork: 0,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if _, err := q.UpdateRunStatus(ctx, UpdateRunStatusParams{Status: "queued", ID: run.ID}); err != nil {
		t.Fatalf("UpdateRunStatus to queued: %v", err)
	}

	// First claim should succeed and transition to 'running'.
	claimed, err := q.ClaimNextQueuedRun(ctx)
	if err != nil {
		t.Fatalf("ClaimNextQueuedRun (first): %v", err)
	}
	if claimed.ID != run.ID {
		t.Errorf("ClaimNextQueuedRun: claimed.ID = %q, want %q", claimed.ID, run.ID)
	}
	if claimed.Status != "running" {
		t.Errorf("ClaimNextQueuedRun: status = %q, want running", claimed.Status)
	}
	if !claimed.StartedAt.Valid {
		t.Error("ClaimNextQueuedRun: started_at should be set after claim")
	}

	// Second claim must return sql.ErrNoRows (AND status='queued' guard).
	_, err = q.ClaimNextQueuedRun(ctx)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("ClaimNextQueuedRun (double-claim guard): want sql.ErrNoRows, got %v", err)
	}
}

func TestCreateRunApprovalAndAppendAndListRunLogs(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	projectID, agentID, taskID := seedRunFixtures(t, ctx, q, "f")

	run, err := q.CreateRun(ctx, CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          agentID,
		RuntimeAdapterID: "sandbox-smoke",
		Prompt:           "Approval and log test",
		RequestedNetwork: 0,
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// CreateRunApproval — round-trip all fields
	approval, err := q.CreateRunApproval(ctx, CreateRunApprovalParams{
		RunID:    run.ID,
		Kind:     "execute",
		Decision: "approved",
		Reason:   sql.NullString{String: "looks safe", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateRunApproval: %v", err)
	}
	if approval.ID == "" {
		t.Error("CreateRunApproval: got empty ID")
	}
	if approval.RunID != run.ID {
		t.Errorf("CreateRunApproval: run_id = %q, want %q", approval.RunID, run.ID)
	}
	if approval.Kind != "execute" {
		t.Errorf("CreateRunApproval: kind = %q, want execute", approval.Kind)
	}
	if approval.Decision != "approved" {
		t.Errorf("CreateRunApproval: decision = %q, want approved", approval.Decision)
	}
	if !approval.Reason.Valid || approval.Reason.String != "looks safe" {
		t.Errorf("CreateRunApproval: reason = %v, want 'looks safe'", approval.Reason)
	}

	// AppendRunLog — two entries
	log1, err := q.AppendRunLog(ctx, AppendRunLogParams{
		RunID:   run.ID,
		Stream:  "stdout",
		Message: "hello from run",
	})
	if err != nil {
		t.Fatalf("AppendRunLog (stdout): %v", err)
	}
	if log1.RunID != run.ID {
		t.Errorf("AppendRunLog: run_id = %q, want %q", log1.RunID, run.ID)
	}
	if log1.Stream != "stdout" {
		t.Errorf("AppendRunLog: stream = %q, want stdout", log1.Stream)
	}
	if log1.Message != "hello from run" {
		t.Errorf("AppendRunLog: message = %q, want 'hello from run'", log1.Message)
	}

	log2, err := q.AppendRunLog(ctx, AppendRunLogParams{
		RunID:   run.ID,
		Stream:  "stderr",
		Message: "warning: something",
	})
	if err != nil {
		t.Fatalf("AppendRunLog (stderr): %v", err)
	}
	if log2.Stream != "stderr" {
		t.Errorf("AppendRunLog (2): stream = %q, want stderr", log2.Stream)
	}

	// ListRunLogs — must return both in insertion order (ASC)
	logs, err := q.ListRunLogs(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("ListRunLogs: got %d logs, want 2", len(logs))
	}
	if logs[0].Message != "hello from run" {
		t.Errorf("ListRunLogs[0].Message = %q, want 'hello from run'", logs[0].Message)
	}
	if logs[1].Message != "warning: something" {
		t.Errorf("ListRunLogs[1].Message = %q, want 'warning: something'", logs[1].Message)
	}
}
