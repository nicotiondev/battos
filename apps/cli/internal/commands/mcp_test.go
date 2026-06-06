// mcp_test.go — tests para el servidor MCP de BattOS.
//
// Estrategia: levantamos un httptest.Server que simula el API de BattOS
// (/memory/*) y comprobamos que cada tool MCP arma la request correcta
// y parsea bien la respuesta.  Los handlers se ejercitan directamente,
// sin levantar el servidor MCP sobre stdio.
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
	"time"

	"github.com/nicotion/battos/apps/cli/internal/client"
)

// --- helpers ---

// newTestClient crea un client.Client apuntando al servidor de test dado.
func newTestClient(srv *httptest.Server, token string) *client.Client {
	return client.New(srv.URL, token)
}

// sampleItem es una observación de ejemplo para usar en respuestas simuladas.
var sampleItem = client.MemoryItem{
	ID:        1,
	Type:      "decision",
	Title:     "Trabajar por fases",
	Content:   "BattOS avanza fase a fase con docs vivos.",
	TopicKey:  "battos/work-style",
	ProjectID: "battos",
	Scope:     "project",
	CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
}

// mustMarshal serializa v o hace panic.
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// --- memory_recent ---

func TestMemoryRecentToolHappyPath(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/recent" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			http.Error(w, fmt.Sprintf("want limit=5 got %q", got), http.StatusBadRequest)
			return
		}
		called = true
		resp := client.MemoryRecentResponse{
			Count: 1,
			Items: []client.MemoryItem{sampleItem},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := memoryRecentToolHandler(context.Background(), c, memoryRecentArgs{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("server was not called")
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "Trabajar por fases") {
		t.Fatalf("response missing item title, got: %s", text)
	}
	if !strings.Contains(text, `"count"`) {
		t.Fatalf("response missing count field, got: %s", text)
	}
}

func TestMemoryRecentToolDefaultLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "20" {
			http.Error(w, fmt.Sprintf("want limit=20 got %q", got), http.StatusBadRequest)
			return
		}
		resp := client.MemoryRecentResponse{Count: 0, Items: []client.MemoryItem{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	// Limit 0 → default 20
	_, _, err := memoryRecentToolHandler(context.Background(), c, memoryRecentArgs{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryRecentToolAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verificar que el token se envía correctamente.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		resp := client.MemoryRecentResponse{Count: 0, Items: []client.MemoryItem{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Sin token → 401
	c := newTestClient(srv, "")
	result, _, err := memoryRecentToolHandler(context.Background(), c, memoryRecentArgs{Limit: 5})
	if err != nil {
		t.Fatalf("tool should not return protocol error on 401: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true on 401 response")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "401") && !strings.Contains(strings.ToLower(text), "unauthorized") {
		t.Fatalf("error message should mention 401 or unauthorized, got: %s", text)
	}
}

// --- memory_search ---

func TestMemorySearchToolHappyPath(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/search" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		resp := client.MemorySearchResponse{
			Results: []client.MemoryResult{{MemoryItem: sampleItem, Rank: 3.14}},
			Count:   1,
			Query:   "fases",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	args := memorySearchArgs{
		Query:     "fases",
		Type:      "decision",
		ProjectID: "battos",
		Limit:     10,
	}
	result, _, err := memorySearchToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected success result, got: %+v", result)
	}

	// Verificar que el body enviado al servidor es correcto.
	if receivedBody == nil {
		t.Fatal("server did not receive body")
	}
	if q, _ := receivedBody["query"].(string); q != "fases" {
		t.Fatalf("want query='fases', got %q", q)
	}
	filter, _ := receivedBody["filter"].(map[string]any)
	if filter == nil {
		t.Fatal("missing filter in body")
	}
	if tp, _ := filter["type"].(string); tp != "decision" {
		t.Fatalf("want filter.type='decision', got %q", tp)
	}
	if pid, _ := filter["project_id"].(string); pid != "battos" {
		t.Fatalf("want filter.project_id='battos', got %q", pid)
	}

	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "Trabajar por fases") {
		t.Fatalf("response missing item title, got: %s", text)
	}
	if !strings.Contains(text, "rank") {
		t.Fatalf("response should include rank, got: %s", text)
	}
}

func TestMemorySearchToolAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"service unavailable"}}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := memorySearchToolHandler(context.Background(), c, memorySearchArgs{Query: "test"})
	if err != nil {
		t.Fatalf("tool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "503") && !strings.Contains(strings.ToLower(text), "service unavailable") {
		t.Fatalf("error should mention 503 or service unavailable, got: %s", text)
	}
}

// --- memory_save ---

func TestMemorySaveToolHappyPath(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/save" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		saved := sampleItem
		saved.Title = "Nueva decisión"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(saved)
	}))
	defer srv.Close()

	c := newTestClient(srv, "tok-secret")
	args := memorySaveArgs{
		Title:     "Nueva decisión",
		Content:   "Usamos sqlc para queries tipadas.",
		Type:      "decision",
		ProjectID: "battos",
		Scope:     "project",
	}
	result, _, err := memorySaveToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result)
	}

	// Verificar body.
	if receivedBody == nil {
		t.Fatal("server did not receive body")
	}
	if title, _ := receivedBody["title"].(string); title != "Nueva decisión" {
		t.Fatalf("want title='Nueva decisión', got %q", title)
	}
	if tp, _ := receivedBody["type"].(string); tp != "decision" {
		t.Fatalf("want type='decision', got %q", tp)
	}
	if scope, _ := receivedBody["scope"].(string); scope != "project" {
		t.Fatalf("want scope='project', got %q", scope)
	}

	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "Nueva decisión") {
		t.Fatalf("response should contain saved title, got: %s", text)
	}
}

func TestMemorySaveToolDefaultType(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sampleItem)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	// Sin type ni scope → defaults "manual" y "project"
	args := memorySaveArgs{
		Title:   "Solo un título",
		Content: "contenido",
	}
	_, _, err := memorySaveToolHandler(context.Background(), c, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp, _ := receivedBody["type"].(string); tp != "manual" {
		t.Fatalf("default type should be 'manual', got %q", tp)
	}
	if sc, _ := receivedBody["scope"].(string); sc != "project" {
		t.Fatalf("default scope should be 'project', got %q", sc)
	}
}

// --- memory_stats ---

func TestMemoryStatsToolHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/stats" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		stats := client.MemoryStatsResponse{
			TotalItems:     42,
			ItemsLast24h:   3,
			UniqueProjects: 2,
			UniqueAgents:   1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := memoryStatsToolHandler(context.Background(), c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "42") {
		t.Fatalf("response should contain total_items=42, got: %s", text)
	}
	if !strings.Contains(text, "total_items") {
		t.Fatalf("response should contain total_items key, got: %s", text)
	}
}

func TestMemoryStatsToolHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"unauthorized"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(srv, "")
	result, _, err := memoryStatsToolHandler(context.Background(), c)
	if err != nil {
		t.Fatalf("tool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true on 401")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "401") && !strings.Contains(strings.ToLower(text), "unauthorized") {
		t.Fatalf("error should mention 401 or unauthorized, got: %s", text)
	}
}

// --- extractText helper ---

// extractText extrae el texto del primer Content de tipo TextContent.
// Soporta tanto *mcp.TextContent (struct con campo Text) como json.RawMessage.
func extractText(t *testing.T, content any) string {
	t.Helper()
	if content == nil {
		t.Fatal("nil content")
	}
	// El SDK serializa Content como []mcp.Content (interface).
	// En los tests ejercitamos los handlers directamente y recibimos
	// *mcp.TextContent a través de la interfaz mcp.Content.
	// Usamos reflexión mínima: marshal+unmarshal.
	b, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshalling content: %v", err)
	}
	var m struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshalling content: %v", err)
	}
	return m.Text
}
