// Package client es el HTTP client tipado que el CLI usa para hablar con el API.
//
// Mantenemos los structs alineados con packages/core, pero el CLI tiene su
// propia copia para evitar acoplamiento accidental — esto cambiará cuando
// usemos oapi-codegen en Fase 3 (un cliente generado).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client habla con el API HTTP de BattOS.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New crea un client apuntando a la baseURL dada (ej: http://localhost:8000).
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// BaseURL expone la URL para que otros paquetes (commands) construyan requests propias.
// Mantenemos la API mínima — en Fase 3 oapi-codegen va a reemplazar todo esto.
func (c *Client) BaseURL() string { return c.baseURL }

// Authorize agrega el token administrativo si fue configurado en CLI o entorno.
func (c *Client) Authorize(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// --- Tipos espejo de packages/core ---
// (En Fase 3 se reemplazan por el cliente generado vía oapi-codegen.)

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

type SubsystemHealth struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	LatencyMs int    `json:"latency_ms,omitempty"`
}

type SystemMetrics struct {
	CPUPercent      float64 `json:"cpu_percent"`
	MemPercent      float64 `json:"mem_percent"`
	MemUsedMB       uint64  `json:"mem_used_mb"`
	MemTotalMB      uint64  `json:"mem_total_mb"`
	NetUploadKBps   float64 `json:"net_upload_kbps"`
	NetDownloadKBps float64 `json:"net_download_kbps"`
}

type StatusResponse struct {
	Version    VersionResponse   `json:"version"`
	Overall    string            `json:"overall"`
	Subsystems []SubsystemHealth `json:"subsystems"`
	Metrics    SystemMetrics     `json:"metrics"`
	Timestamp  time.Time         `json:"timestamp"`
}

// --- Métodos ---

// Status llama a GET /status y devuelve el estado completo del OS.
//
// Si el API no responde, devuelve un error envuelto con contexto útil
// (ej: "no se pudo contactar el API en http://localhost:8000 — ¿está corriendo?").
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	var s StatusResponse
	if err := c.getJSON(ctx, "/status", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Health llama a GET /health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var h HealthResponse
	if err := c.getJSON(ctx, "/health", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// Version llama a GET /version.
func (c *Client) Version(ctx context.Context) (*VersionResponse, error) {
	var v VersionResponse
	if err := c.getJSON(ctx, "/version", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// --- Tipos de Memory Core ---

// MemoryItem es una observación persistida en el Memory Core.
type MemoryItem struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	TopicKey  string    `json:"topic_key,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemoryResult es una observación con rank de búsqueda FTS5.
type MemoryResult struct {
	MemoryItem
	Rank float64 `json:"rank"`
}

// MemoryRecentResponse es la respuesta de GET /memory/recent.
type MemoryRecentResponse struct {
	Count int          `json:"count"`
	Items []MemoryItem `json:"items"`
}

// MemorySearchFilter son los filtros opcionales de búsqueda.
type MemorySearchFilter struct {
	Type      string `json:"type,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	Scope     string `json:"scope,omitempty"`
}

// MemorySearchRequest es el cuerpo de POST /memory/search.
type MemorySearchRequest struct {
	Query  string             `json:"query"`
	Filter MemorySearchFilter `json:"filter"`
	Limit  int                `json:"limit,omitempty"`
}

// MemorySearchResponse es la respuesta de POST /memory/search.
type MemorySearchResponse struct {
	Results []MemoryResult `json:"results"`
	Count   int            `json:"count"`
	Query   string         `json:"query"`
}

// MemorySaveRequest es el cuerpo de POST /memory/save.
type MemorySaveRequest struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	TopicKey  string `json:"topic_key,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	Scope     string `json:"scope"`
}

// MemoryStatsResponse es la respuesta de GET /memory/stats.
type MemoryStatsResponse struct {
	TotalItems     int64     `json:"total_items"`
	ItemsLast24h   int64     `json:"items_last_24h"`
	UniqueProjects int64     `json:"unique_projects"`
	UniqueAgents   int64     `json:"unique_agents"`
	OldestItem     time.Time `json:"oldest_item"`
	NewestItem     time.Time `json:"newest_item"`
}

// --- Métodos de Memory Core ---

// MemoryRecent llama a GET /memory/recent?limit=N.
func (c *Client) MemoryRecent(ctx context.Context, limit int) (*MemoryRecentResponse, error) {
	path := fmt.Sprintf("/memory/recent?limit=%d", limit)
	var out MemoryRecentResponse
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("memory recent: %w", err)
	}
	return &out, nil
}

// MemorySearch llama a POST /memory/search con la query y filtros dados.
func (c *Client) MemorySearch(ctx context.Context, req MemorySearchRequest) (*MemorySearchResponse, error) {
	var out MemorySearchResponse
	if err := c.postJSON(ctx, "/memory/search", req, &out); err != nil {
		return nil, fmt.Errorf("memory search: %w", err)
	}
	return &out, nil
}

// MemorySave llama a POST /memory/save y devuelve la observación guardada.
func (c *Client) MemorySave(ctx context.Context, req MemorySaveRequest) (*MemoryItem, error) {
	var out MemoryItem
	if err := c.postJSON(ctx, "/memory/save", req, &out); err != nil {
		return nil, fmt.Errorf("memory save: %w", err)
	}
	return &out, nil
}

// MemoryStats llama a GET /memory/stats.
func (c *Client) MemoryStats(ctx context.Context) (*MemoryStatsResponse, error) {
	var out MemoryStatsResponse
	if err := c.getJSON(ctx, "/memory/stats", &out); err != nil {
		return nil, fmt.Errorf("memory stats: %w", err)
	}
	return &out, nil
}

// getJSON hace GET al path y deserializa la respuesta en out.
// Envuelve errores de red con un mensaje accionable para el usuario.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("construyendo request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.Authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo contactar el API en %s — ¿está corriendo? (%w)", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API respondió %d en %s", resp.StatusCode, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decodificando respuesta JSON: %w", err)
	}
	return nil
}

// postJSON hace POST al path con body JSON y deserializa la respuesta en out.
func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("codificando body: %w", err)
	}
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("construyendo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.Authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo contactar el API en %s — ¿está corriendo? (%w)", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(raw))
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if jsonErr := json.Unmarshal(raw, &apiErr); jsonErr == nil && strings.TrimSpace(apiErr.Error.Message) != "" {
			msg = apiErr.Error.Message
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decodificando respuesta JSON: %w", err)
	}
	return nil
}
