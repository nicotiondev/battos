package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// EngramProvider implementa MemoryProvider delegando a Engram HTTP cuando es
// posible, y cayendo al BuiltinCore (fallback offline) para operaciones no
// disponibles via HTTP.
//
// Engram v1.15.10 expone únicamente:
//   - GET /health   → ping
//   - GET /stats    → métricas de alto nivel
//   - GET /context  → contexto markdown por proyecto
//
// Save, Search, Recent, GetByID y FindConflictCandidates no tienen endpoints HTTP
// en esta versión de Engram, por lo que siempre delegan al Fallback (*Core).
type EngramProvider struct {
	BaseURL  string       // e.g. "http://localhost:7437"
	Fallback *Core        // BuiltinCore — operaciones de escritura/búsqueda
	Client   *http.Client // inyectable para tests
}

// NewEngramProvider crea un EngramProvider listo para usar.
// baseURL debe ser la raíz del servidor Engram (sin trailing slash).
// fallback no puede ser nil.
func NewEngramProvider(baseURL string, fallback *Core) *EngramProvider {
	return &EngramProvider{
		BaseURL:  baseURL,
		Fallback: fallback,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// --- MemoryProvider implementation ---

// Save delega al Core local.
func (e *EngramProvider) Save(ctx context.Context, o Observation) (*Observation, error) {
	return e.Fallback.Save(ctx, o)
}

// Search delega al Core local.
func (e *EngramProvider) Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error) {
	return e.Fallback.Search(ctx, query, filter)
}

// Recent delega al Core local.
func (e *EngramProvider) Recent(ctx context.Context, limit int) ([]Observation, error) {
	return e.Fallback.Recent(ctx, limit)
}

// GetByID delega al Core local.
func (e *EngramProvider) GetByID(ctx context.Context, id int64) (*Observation, error) {
	return e.Fallback.GetByID(ctx, id)
}

// FindConflictCandidates delega al Core local.
func (e *EngramProvider) FindConflictCandidates(ctx context.Context, obs Observation, limit int) ([]SearchResult, error) {
	return e.Fallback.FindConflictCandidates(ctx, obs, limit)
}

// Stats llama GET /stats a Engram. Si Engram no está disponible (error de red
// o status != 200), delega al fallback.
//
// Mapeo desde el JSON de Engram:
//   - total_observations → TotalItems
//   - len(projects)      → UniqueProjects
//   - el resto          → 0 o desde fallback en caso de error
func (e *EngramProvider) Stats(ctx context.Context) (*Stats, error) {
	engramStats, err := e.fetchEngramStats(ctx)
	if err != nil {
		// Engram no disponible → fallback silencioso.
		return e.Fallback.Stats(ctx)
	}

	return &Stats{
		TotalItems:     engramStats.TotalObservations,
		UniqueProjects: int64(len(engramStats.Projects)),
		// El resto de campos no está disponible via HTTP; se deja en cero
		// para no forzar una llamada extra al fallback.
	}, nil
}

// Context llama GET /context?project=PROJECT a Engram y devuelve el bloque
// markdown de contexto consolidado. Usado por el seam de contexto del worker.
//
// Si Engram no está disponible devuelve error — los callers deben decidir si
// quieren continuar sin contexto externo.
func (e *EngramProvider) Context(ctx context.Context, project string) (string, error) {
	endpoint := e.BaseURL + "/context"
	if project != "" {
		endpoint += "?project=" + url.QueryEscape(project)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("engram: building context request: %w", err)
	}

	resp, err := e.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("engram: context request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("engram: context returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("engram: reading context body: %w", err)
	}

	var payload struct {
		Context string `json:"context"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("engram: parsing context response: %w", err)
	}
	return payload.Context, nil
}

// Ping llama GET /health y verifica que status == "ok".
// Útil para health checks del servicio.
func (e *EngramProvider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.BaseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("engram: building ping request: %w", err)
	}

	resp, err := e.client().Do(req)
	if err != nil {
		return fmt.Errorf("engram: ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("engram: ping returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("engram: reading ping body: %w", err)
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("engram: parsing health response: %w", err)
	}
	if payload.Status != "ok" {
		return fmt.Errorf("engram: unexpected health status %q", payload.Status)
	}
	return nil
}

// --- compile-time checks ---

var _ MemoryProvider = (*EngramProvider)(nil)

// --- internal helpers ---

// engramStatsResponse es el shape del JSON que devuelve GET /stats de Engram.
type engramStatsResponse struct {
	TotalSessions      int64    `json:"total_sessions"`
	TotalObservations  int64    `json:"total_observations"`
	TotalPrompts       int64    `json:"total_prompts"`
	Projects           []string `json:"projects"`
}

// fetchEngramStats llama GET /stats y decodifica la respuesta.
func (e *EngramProvider) fetchEngramStats(ctx context.Context) (*engramStatsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.BaseURL+"/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("engram: building stats request: %w", err)
	}

	resp, err := e.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("engram: stats request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("engram: stats returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("engram: reading stats body: %w", err)
	}

	var s engramStatsResponse
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("engram: parsing stats response: %w", err)
	}
	return &s, nil
}

// client devuelve el http.Client configurado, cayendo al default si nil.
func (e *EngramProvider) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return http.DefaultClient
}
