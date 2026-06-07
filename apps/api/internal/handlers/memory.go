// memory.go — handlers HTTP del Memory Core.
//
// Endpoints:
//   GET  /memory/recent?limit=N           → últimas N observaciones
//   POST /memory/search    {query, ...}   → FTS5 search
//   POST /memory/save      {observation}  → guardar/upsert
//   GET  /memory/stats                    → contadores agregados
//   GET  /memory/{id}                     → una observación específica
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/memory"
)

// MemoryHandler agrupa los endpoints relacionados al Memory Core.
type MemoryHandler struct {
	core *memory.Core
}

// NewMemoryHandler construye el handler con el Core inyectado.
func NewMemoryHandler(core *memory.Core) *MemoryHandler {
	return &MemoryHandler{core: core}
}

// Recent — GET /memory/recent?limit=20
func (h *MemoryHandler) Recent(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 20, 200)
	items, err := h.core.Recent(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "code": 500},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

// Search — POST /memory/search
//
// Body:
//
//	{"query": "ficha tecnica", "filter": {"type": "decision", "project_id": "red-nbl"}, "limit": 20}
func (h *MemoryHandler) Search(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query  string `json:"query"`
		Filter struct {
			Type      memory.ObservationType `json:"type"`
			ProjectID string                 `json:"project_id"`
			AgentID   string                 `json:"agent_id"`
			Scope     memory.Scope           `json:"scope"`
		} `json:"filter"`
		Limit int `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON inválido: " + err.Error(), "code": 400},
		})
		return
	}

	filter := memory.SearchFilter{
		Type:      req.Filter.Type,
		ProjectID: req.Filter.ProjectID,
		AgentID:   req.Filter.AgentID,
		Scope:     req.Filter.Scope,
		Limit:     req.Limit,
	}
	results, err := h.core.Search(r.Context(), req.Query, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "code": 500},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
		"query":   req.Query,
	})
}

// saveResponse envuelve una Observation agregando conflict_candidates sin romper
// la compatibilidad con clientes que leen los campos de Observation directamente
// (embed en JSON produce los campos al nivel raíz).
type saveResponse struct {
	memory.Observation
	ConflictCandidates []memory.SearchResult `json:"conflict_candidates,omitempty"`
}

// Save — POST /memory/save
//
// Body: estructura completa de Observation. Si TopicKey está presente, upsertea.
// La respuesta devuelve los campos de la observación en la raíz (backward compat)
// más un campo conflict_candidates opcional con observaciones que podrían
// solaparse léxicamente con la recién guardada.
func (h *MemoryHandler) Save(w http.ResponseWriter, r *http.Request) {
	var obs memory.Observation
	if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "JSON inválido: " + err.Error(), "code": 400},
		})
		return
	}
	saved, err := h.core.Save(r.Context(), obs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "code": 500},
		})
		return
	}

	resp := saveResponse{Observation: *saved}

	// Detección de candidatos de conflicto — determinista, sin LLM.
	// Si falla, no rompemos el save (el save ya tuvo éxito); simplemente
	// omitimos conflict_candidates en la respuesta.
	candidates, err := h.core.FindConflictCandidates(r.Context(), *saved, 5)
	if err == nil && len(candidates) > 0 {
		resp.ConflictCandidates = candidates
	}
	// err != nil: ignorado deliberadamente — el save fue exitoso y no queremos
	// fallar la respuesta por una búsqueda de candidatos no crítica.

	writeJSON(w, http.StatusCreated, resp)
}

// Stats — GET /memory/stats
func (h *MemoryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	s, err := h.core.Stats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "code": 500},
		})
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// GetByID — GET /memory/{id}
func (h *MemoryHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]any{"message": "id inválido", "code": 400},
		})
		return
	}
	obs, err := h.core.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{"message": err.Error(), "code": 500},
		})
		return
	}
	if obs == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "observación no encontrada", "code": 404},
		})
		return
	}
	writeJSON(w, http.StatusOK, obs)
}

func parseLimit(r *http.Request, def, max int) int {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return def
	}
	n, err := strconv.Atoi(q)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
