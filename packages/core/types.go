// Package core contiene tipos compartidos entre apps/api y apps/cli.
//
// Estos tipos viajan en el cuerpo JSON del API. Cualquier cambio de campo
// rompe la frontera api↔cli, así que se versionan con cuidado.
package core

import "time"

// HealthStatus describe el estado de salud de un subsistema.
type HealthStatus string

const (
	HealthOK       HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthDown     HealthStatus = "down"
	HealthUnknown  HealthStatus = "unknown"
)

// HealthResponse es la respuesta de GET /health.
type HealthResponse struct {
	Status    HealthStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
}

// VersionResponse es la respuesta de GET /version.
type VersionResponse struct {
	Version   string `json:"version"`    // semver de BattOS
	Commit    string `json:"commit"`     // git short SHA (build-time)
	BuildDate string `json:"build_date"` // RFC3339 (build-time)
	GoVersion string `json:"go_version"` // runtime.Version()
}

// SubsystemHealth describe el estado de cada componente reportado en /status.
type SubsystemHealth struct {
	Name      string       `json:"name"` // "database", "memory", "config", ...
	Status    HealthStatus `json:"status"`
	Detail    string       `json:"detail,omitempty"` // mensaje legible (no técnico)
	LatencyMs int          `json:"latency_ms,omitempty"`
}

// StatusResponse es la respuesta agregada de GET /status.
// Contiene el "latido" completo del OS — qué subsistemas están vivos
// y métricas en vivo del sistema host.
type StatusResponse struct {
	Version    VersionResponse   `json:"version"`
	Overall    HealthStatus      `json:"overall"`
	Subsystems []SubsystemHealth `json:"subsystems"`
	Metrics    SystemMetrics     `json:"metrics"`
	Timestamp  time.Time         `json:"timestamp"`
}

// SystemMetrics es una snapshot de CPU/MEM/NET/DISK del host donde corre el API.
// Estos valores también se transmiten en vivo por SSE en /events/system-metrics.
type SystemMetrics struct {
	CPUPercent      float64         `json:"cpu_percent"` // 0..100
	MemPercent      float64         `json:"mem_percent"` // 0..100
	MemUsedMB       uint64          `json:"mem_used_mb"`
	MemTotalMB      uint64          `json:"mem_total_mb"`
	NetUploadKBps   float64         `json:"net_upload_kbps"`
	NetDownloadKBps float64         `json:"net_download_kbps"`
	DiskPercent     float64         `json:"disk_percent"` // 0..100, partición principal
	DiskUsedGB      float64         `json:"disk_used_gb"`
	DiskTotalGB     float64         `json:"disk_total_gb"`
	TopProcesses    []ProcessSample `json:"top_processes,omitempty"`
}

// ProcessSample es un proceso del host en la última muestra (top por memoria).
type ProcessSample struct {
	PID        int64   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemMB      uint64  `json:"mem_mb"`
}
