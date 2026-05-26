// Package client es el HTTP client tipado que el CLI usa para hablar con el API.
//
// Mantenemos los structs alineados con packages/core, pero el CLI tiene su
// propia copia para evitar acoplamiento accidental — esto cambiará cuando
// usemos oapi-codegen en Fase 3 (un cliente generado).
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client habla con el API HTTP de BattOS.
type Client struct {
	baseURL string
	http    *http.Client
}

// New crea un client apuntando a la baseURL dada (ej: http://localhost:8000).
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// BaseURL expone la URL para que otros paquetes (commands) construyan requests propias.
// Mantenemos la API mínima — en Fase 3 oapi-codegen va a reemplazar todo esto.
func (c *Client) BaseURL() string { return c.baseURL }

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

// getJSON hace GET al path y deserializa la respuesta en out.
// Envuelve errores de red con un mensaje accionable para el usuario.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("construyendo request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

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
