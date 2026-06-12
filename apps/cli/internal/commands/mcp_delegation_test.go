// mcp_delegation_test.go — tests para las tools MCP de delegación (Etapa 3, B1d):
// team_spawn_run, team_read_board y team_get_run_status.
//
// Misma estrategia que mcp_test.go: un httptest.Server simula el API de BattOS
// y se ejercitan los handlers directamente, sin levantar el servidor MCP.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicotion/battos/apps/cli/internal/client"
)

// sampleRun es un run de ejemplo para respuestas simuladas.
var sampleRun = client.Run{
	ID:               "run-child-1",
	ProjectID:        "battos",
	TaskID:           "task-1",
	AgentID:          "codex",
	RuntimeAdapterID: "codex",
	Prompt:           "implementar la feature X",
	ExecutionMode:    "sandbox",
	Status:           "awaiting_approval",
}

// --- team_spawn_run ---

func TestTeamSpawnRunToolHappyPath(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runs" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sampleRun)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	args := teamSpawnRunArgs{
		ProjectID:        "battos",
		TaskID:           "task-1",
		AgentID:          "codex",
		RuntimeAdapterID: "codex",
		Prompt:           "implementar la feature X",
		ParentRunID:      "run-lead-9",
		RequestedNetwork: true,
	}
	result, _, err := teamSpawnRunToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success result, got: %+v", result)
	}

	// Verificar el body enviado al API.
	if receivedBody == nil {
		t.Fatal("server did not receive body")
	}
	if pid, _ := receivedBody["project_id"].(string); pid != "battos" {
		t.Fatalf("want project_id='battos', got %q", pid)
	}
	if parent, _ := receivedBody["parent_run_id"].(string); parent != "run-lead-9" {
		t.Fatalf("want parent_run_id='run-lead-9', got %q", parent)
	}
	// Sin execution_mode en args → default "sandbox" client-side.
	if mode, _ := receivedBody["execution_mode"].(string); mode != "sandbox" {
		t.Fatalf("want execution_mode='sandbox' (default), got %q", mode)
	}
	if network, _ := receivedBody["requested_network"].(bool); !network {
		t.Fatal("want requested_network=true in body")
	}

	// La respuesta debe traer run_id + status + nota de aprobación humana.
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "run-child-1") {
		t.Fatalf("response missing run_id, got: %s", text)
	}
	if !strings.Contains(text, "awaiting_approval") {
		t.Fatalf("response missing status, got: %s", text)
	}
	if !strings.Contains(text, "aprobación humana") {
		t.Fatalf("response missing human-approval note, got: %s", text)
	}
	if !strings.Contains(text, "run-lead-9") {
		t.Fatalf("response should echo parent_run_id, got: %s", text)
	}
}

func TestTeamSpawnRunToolExplicitExecutionMode(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		run := sampleRun
		run.ExecutionMode = "direct"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(run)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	args := teamSpawnRunArgs{
		ProjectID:        "battos",
		TaskID:           "task-1",
		AgentID:          "codex",
		RuntimeAdapterID: "codex",
		Prompt:           "p",
		ExecutionMode:    "direct",
	}
	result, _, err := teamSpawnRunToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}
	if mode, _ := receivedBody["execution_mode"].(string); mode != "direct" {
		t.Fatalf("want execution_mode='direct', got %q", mode)
	}
	// Sin parent_run_id en args, la respuesta no lo incluye.
	text := extractText(t, result.Content[0])
	if strings.Contains(text, "parent_run_id") {
		t.Fatalf("response should not include parent_run_id when absent, got: %s", text)
	}
}

func TestTeamSpawnRunToolAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"execution_mode invalido; valores permitidos: sandbox, direct, connected"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	args := teamSpawnRunArgs{
		ProjectID:        "battos",
		TaskID:           "task-1",
		AgentID:          "codex",
		RuntimeAdapterID: "codex",
		Prompt:           "p",
		ExecutionMode:    "yolo",
	}
	result, _, err := teamSpawnRunToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("tool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true on 400")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "execution_mode invalido") {
		t.Fatalf("error should surface API message, got: %s", text)
	}
}

// --- team_read_board ---

func TestTeamReadBoardToolFiltersStatusClientSide(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("project_id"); got != "battos" {
			http.Error(w, fmt.Sprintf("want project_id=battos got %q", got), http.StatusBadRequest)
			return
		}
		tasks := []client.Task{
			{ID: "task-1", ProjectID: "battos", Title: "Diseñar contrato", Status: "ready", AssignedAgentID: "codex"},
			{ID: "task-2", ProjectID: "battos", Title: "Escribir docs", Status: "backlog"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	// El API no filtra por status: la tool debe filtrar client-side.
	result, _, err := teamReadBoardToolHandler(context.Background(), c, teamReadBoardArgs{ProjectID: "battos", Status: "ready"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "task-1") || !strings.Contains(text, "Diseñar contrato") {
		t.Fatalf("response missing matching task, got: %s", text)
	}
	if strings.Contains(text, "task-2") {
		t.Fatalf("response should exclude tasks with other status, got: %s", text)
	}
	if !strings.Contains(text, "codex") {
		t.Fatalf("response should include assigned_agent_id, got: %s", text)
	}
	if !strings.Contains(text, `"count": 1`) {
		t.Fatalf("response should report count=1, got: %s", text)
	}
}

func TestTeamReadBoardToolNoFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("project_id"); got != "" {
			http.Error(w, "no project_id filter expected", http.StatusBadRequest)
			return
		}
		tasks := []client.Task{
			{ID: "task-1", ProjectID: "battos", Title: "A", Status: "ready"},
			{ID: "task-2", ProjectID: "web", Title: "B", Status: "done"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := teamReadBoardToolHandler(context.Background(), c, teamReadBoardArgs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, `"count": 2`) {
		t.Fatalf("response should report count=2, got: %s", text)
	}
}

func TestTeamReadBoardToolAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := teamReadBoardToolHandler(context.Background(), c, teamReadBoardArgs{})
	if err != nil {
		t.Fatalf("tool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true on 401")
	}
}

// --- team_get_run_status ---

func TestTeamGetRunStatusToolHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runs/run-child-1" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		run := sampleRun
		run.Status = "succeeded"
		run.ResultSummary = "feature X implementada"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(run)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := teamGetRunStatusToolHandler(context.Background(), c, teamGetRunStatusArgs{RunID: "run-child-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result.Content)
	}
	text := extractText(t, result.Content[0])
	for _, want := range []string{"run-child-1", "succeeded", "feature X implementada", "sandbox", "codex"} {
		if !strings.Contains(text, want) {
			t.Fatalf("response missing %q, got: %s", want, text)
		}
	}
}

func TestTeamGetRunStatusToolNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"run not found"}}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := teamGetRunStatusToolHandler(context.Background(), c, teamGetRunStatusArgs{RunID: "ghost"})
	if err != nil {
		t.Fatalf("tool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true on 404")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "ghost") || !strings.Contains(text, "no existe") {
		t.Fatalf("error should clearly say the run does not exist, got: %s", text)
	}
}
