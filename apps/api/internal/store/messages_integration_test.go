package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestAgentMessageMailboxRoundTrip(t *testing.T) {
	ctx := context.Background()
	q, done := openTestDB(t)
	defer done()

	// seedRunFixtures gives us a project + an agent (the recipient) + a task.
	projectID, leadID, _ := seedRunFixtures(t, ctx, q, "msg")

	// A second agent to be the recipient's counterpart (sender is the lead above;
	// recipient is a worker agent we create here).
	worker, err := q.CreateAgent(ctx, CreateAgentParams{
		ID: "msg-worker", Slug: "msg-worker", Name: "Worker", Status: "active",
		RuntimeConfig: "{}", AllowedTools: "[]", AllowedProjects: "[]", RiskLevel: "medium",
	})
	if err != nil {
		t.Fatalf("CreateAgent worker: %v", err)
	}

	// Lead sends two messages to the worker.
	for _, body := range []string{"primer encargo", "segundo encargo"} {
		if _, err := q.CreateAgentMessage(ctx, CreateAgentMessageParams{
			ProjectID:   sql.NullString{String: projectID, Valid: true},
			FromAgentID: sql.NullString{String: leadID, Valid: true},
			ToAgentID:   worker.ID,
			Subject:     sql.NullString{String: "tarea", Valid: true},
			Body:        body,
		}); err != nil {
			t.Fatalf("CreateAgentMessage: %v", err)
		}
	}

	// Unread count should be 2.
	unread, err := q.CountUnreadForAgent(ctx, worker.ID)
	if err != nil {
		t.Fatalf("CountUnreadForAgent: %v", err)
	}
	if unread != 2 {
		t.Fatalf("unread=%d, want 2", unread)
	}

	// Inbox lists both (newest first).
	inbox, err := q.ListInboxForAgent(ctx, ListInboxForAgentParams{ToAgentID: worker.ID, Limit: 10})
	if err != nil {
		t.Fatalf("ListInboxForAgent: %v", err)
	}
	if len(inbox) != 2 {
		t.Fatalf("inbox len=%d, want 2", len(inbox))
	}

	// Unread-only list, oldest first, then mark the first read.
	pending, err := q.ListUnreadInboxForAgent(ctx, worker.ID)
	if err != nil {
		t.Fatalf("ListUnreadInboxForAgent: %v", err)
	}
	if len(pending) != 2 || pending[0].Body != "primer encargo" {
		t.Fatalf("unread inbox = %+v, want oldest-first with 'primer encargo'", pending)
	}

	marked, err := q.MarkAgentMessageRead(ctx, pending[0].ID)
	if err != nil {
		t.Fatalf("MarkAgentMessageRead: %v", err)
	}
	if !marked.ReadAt.Valid {
		t.Error("read_at should be set after marking read")
	}

	// Unread count drops to 1.
	unread, err = q.CountUnreadForAgent(ctx, worker.ID)
	if err != nil {
		t.Fatalf("CountUnreadForAgent (post): %v", err)
	}
	if unread != 1 {
		t.Fatalf("unread=%d after marking one read, want 1", unread)
	}

	// The other agent (lead) has an empty inbox — messages are addressed, not broadcast.
	leadUnread, err := q.CountUnreadForAgent(ctx, leadID)
	if err != nil {
		t.Fatalf("CountUnreadForAgent lead: %v", err)
	}
	if leadUnread != 0 {
		t.Fatalf("lead unread=%d, want 0 (messages are directed)", leadUnread)
	}
}
