package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/nicotion/battos/apps/api/internal/memory"
)

func openTestMemoryCore(t *testing.T) *memory.Core {
	t.Helper()
	core, err := memory.Open(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("memory.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := core.Close(); err != nil {
			t.Errorf("core.Close: %v", err)
		}
	})
	return core
}

// TestSaveHandler_ResponseBackwardCompatible verifica que el handler de Save
// devuelve los campos de la observación en la raíz del JSON (sin anidar).
func TestSaveHandler_ResponseBackwardCompatible(t *testing.T) {
	core := openTestMemoryCore(t)
	h := NewMemoryHandler(core)

	body := `{"type":"decision","title":"Auth via JWT seleccionado","content":"Usamos JWT","project_id":"battos","scope":"project"}`
	req := httptest.NewRequest(http.MethodPost, "/memory/save", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Campos raíz de Observation deben estar presentes (backward compat).
	for _, field := range []string{"id", "type", "title", "content", "scope", "created_at", "updated_at"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("response missing field %q; got keys: %v", field, keys(resp))
		}
	}
}

// TestSaveHandler_ConflictCandidatesPresent verifica que cuando hay observaciones
// superpuestas, la respuesta incluye conflict_candidates con al menos un resultado.
func TestSaveHandler_ConflictCandidatesPresent(t *testing.T) {
	core := openTestMemoryCore(t)
	h := NewMemoryHandler(core)

	// Insertar observación existente directamente en el core.
	_, err := core.Save(t.Context(), memory.Observation{
		Type:      memory.TypeDecision,
		Title:     "Auth JWT decision tomada",
		Content:   "Usamos JWT para autenticación",
		ProjectID: "battos",
		Scope:     memory.ScopeProject,
	})
	if err != nil {
		t.Fatalf("core.Save existing: %v", err)
	}

	// Guardar una observación con título superpuesto via handler.
	body := `{"type":"decision","title":"Auth JWT alternativa revisada","content":"Revisión del approach","project_id":"battos","scope":"project"}`
	req := httptest.NewRequest(http.MethodPost, "/memory/save", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	candidates, ok := resp["conflict_candidates"]
	if !ok {
		t.Fatalf("response missing conflict_candidates; keys: %v", keys(resp))
	}
	list, ok := candidates.([]any)
	if !ok {
		t.Fatalf("conflict_candidates is not an array: %T", candidates)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one conflict_candidate, got none")
	}
}

// TestSaveHandler_ConflictCandidatesOmittedWhenNone verifica que cuando no hay
// candidatos de conflicto, el campo conflict_candidates se omite (omitempty).
func TestSaveHandler_ConflictCandidatesOmittedWhenNone(t *testing.T) {
	core := openTestMemoryCore(t)
	h := NewMemoryHandler(core)

	// Sin observaciones previas → no hay candidatos posibles.
	body := `{"type":"bugfix","title":"Corregido panic en startup goroutine","content":"nil pointer fix","project_id":"battos","scope":"project"}`
	req := httptest.NewRequest(http.MethodPost, "/memory/save", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Save(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if _, ok := resp["conflict_candidates"]; ok {
		t.Fatal("conflict_candidates should be omitted when there are no candidates")
	}
}

// keys es un helper de test para listar las claves de un map[string]any.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
