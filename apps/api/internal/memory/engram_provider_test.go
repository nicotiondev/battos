package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// newTestEngramProvider crea un EngramProvider apuntando al servidor mock dado.
// El Core de fallback usa SQLite en memoria (TempDir).
func newTestEngramProvider(t *testing.T, srv *httptest.Server) *EngramProvider {
	t.Helper()
	core := openTestCore(t)
	return &EngramProvider{
		BaseURL:  srv.URL,
		Fallback: core,
		Client:   srv.Client(),
	}
}

// --- Stats: Engram disponible ---

func TestEngramProvider_Stats_FromEngram(t *testing.T) {
	// Mock que devuelve el JSON real de Engram.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		payload := engramStatsResponse{
			TotalSessions:     3,
			TotalObservations: 5,
			TotalPrompts:      12,
			Projects:          []string{"battos", "side-project"},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encoding stats response: %v", err)
		}
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ep := newTestEngramProvider(t, srv)
	stats, err := ep.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.TotalItems != 5 {
		t.Errorf("TotalItems = %d, want 5", stats.TotalItems)
	}
	if stats.UniqueProjects != 2 {
		t.Errorf("UniqueProjects = %d, want 2 (len(projects))", stats.UniqueProjects)
	}
}

// --- Stats: Engram devuelve 500 → fallback al Core ---

func TestEngramProvider_Stats_FallbackOn500(t *testing.T) {
	// Mock que siempre falla.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Poblar el Core de fallback con 2 observaciones para verificar que Stats
	// las ve.
	core, err := Open(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = core.Close() })

	ctx := context.Background()
	for _, title := range []string{"obs one", "obs two"} {
		if _, err := core.Save(ctx, Observation{
			Type:    TypeManual,
			Title:   title,
			Content: "content",
		}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	ep := &EngramProvider{
		BaseURL:  srv.URL,
		Fallback: core,
		Client:   srv.Client(),
	}

	stats, err := ep.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v (expected fallback to succeed)", err)
	}
	if stats.TotalItems != 2 {
		t.Errorf("TotalItems = %d, want 2 (from fallback Core)", stats.TotalItems)
	}
}

// --- Stats: Engram no disponible (red) → fallback al Core ---

func TestEngramProvider_Stats_FallbackOnNetworkError(t *testing.T) {
	// Servidor cerrado antes de llamar.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // cerrado inmediatamente

	core := openTestCore(t)
	ep := &EngramProvider{
		BaseURL:  srv.URL,
		Fallback: core,
		Client:   srv.Client(),
	}

	// Debe devolver stats del Core (vacío) sin error.
	stats, err := ep.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error = %v, want nil (fallback)", err)
	}
	if stats == nil {
		t.Fatal("Stats() returned nil")
	}
}

// --- Context: respuesta correcta ---

func TestEngramProvider_Context_OK(t *testing.T) {
	const want = "## Memory from Previous Sessions\n- some fact"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/context" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("project") != "battos" {
			http.Error(w, "missing project", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"context": want})
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	ep := newTestEngramProvider(t, srv)
	got, err := ep.Context(context.Background(), "battos")
	if err != nil {
		t.Fatalf("Context() error = %v", err)
	}
	if got != want {
		t.Errorf("Context() = %q, want %q", got, want)
	}
}

// --- Context: Engram devuelve error ---

func TestEngramProvider_Context_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ep := newTestEngramProvider(t, srv)
	_, err := ep.Context(context.Background(), "battos")
	if err == nil {
		t.Fatal("Context() expected error, got nil")
	}
}

// --- Ping: ok ---

func TestEngramProvider_Ping_OK(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"service": "engram",
			"status":  "ok",
			"version": "1.15.10",
		})
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	ep := newTestEngramProvider(t, srv)
	if err := ep.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

// --- Compile-time: EngramProvider satisface MemoryProvider ---

func TestEngramProvider_ImplementsInterface(t *testing.T) {
	// Si no compila, el test ni arranca. Ejercicio en tiempo de ejecución
	// para que sea visible en el reporte de tests.
	var _ MemoryProvider = (*EngramProvider)(nil)
}
