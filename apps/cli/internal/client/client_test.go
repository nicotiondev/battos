package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizeAddsConfiguredBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	New("http://localhost:8000", "admin-token").Authorize(req)
	if got := req.Header.Get("Authorization"); got != "Bearer admin-token" {
		t.Fatalf("authorization = %q", got)
	}
}

func TestAuthorizeLeavesHeaderEmptyWithoutToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	New("http://localhost:8000", "").Authorize(req)
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("authorization = %q", got)
	}
}

func TestCreateRunPostsBodyWithParentRunID(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runs" || r.Method != http.MethodPost {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Run{ID: "run-1", Status: "awaiting_approval", ExecutionMode: "sandbox"})
	}))
	defer srv.Close()

	run, err := New(srv.URL, "").CreateRun(context.Background(), CreateRunRequest{
		ProjectID:        "battos",
		TaskID:           "task-1",
		AgentID:          "codex",
		RuntimeAdapterID: "codex",
		Prompt:           "hacer X",
		ExecutionMode:    "sandbox",
		ParentRunID:      "run-lead-9",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.ID != "run-1" || run.Status != "awaiting_approval" {
		t.Fatalf("run = %+v", run)
	}
	if parent, _ := receivedBody["parent_run_id"].(string); parent != "run-lead-9" {
		t.Fatalf("parent_run_id = %q, want run-lead-9", parent)
	}
	// parent_run_id vacío no debe viajar en el body (omitempty).
	if _, ok := receivedBody["skill_id"]; ok {
		t.Fatal("skill_id vacío no debería estar en el body")
	}
}

func TestGetRunEscapesIDAndPropagatesNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/runs/run-1" {
			json.NewEncoder(w).Encode(Run{ID: "run-1", Status: "queued"})
			return
		}
		http.Error(w, `{"error":{"message":"run not found"}}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	run, err := c.GetRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != "queued" {
		t.Fatalf("status = %q", run.Status)
	}

	if _, err := c.GetRun(context.Background(), "ghost"); err == nil {
		t.Fatal("expected error on 404")
	} else if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error should mention 404, got: %v", err)
	}
}

func TestListTasksAddsProjectFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("project_id"); got != "battos" {
			http.Error(w, "want project_id=battos", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode([]Task{{ID: "task-1", ProjectID: "battos", Title: "A", Status: "ready"}})
	}))
	defer srv.Close()

	tasks, err := New(srv.URL, "").ListTasks(context.Background(), "battos")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-1" {
		t.Fatalf("tasks = %+v", tasks)
	}
}
