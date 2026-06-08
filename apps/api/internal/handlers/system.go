// Package handlers contiene los HTTP handlers del API.
//
// Convención: un archivo por área (system, projects, agents, ...).
// Cada handler es un método sobre una struct que recibe sus dependencias
// vía constructor (NewXxxHandler). Esto facilita inyectar mocks en tests.
package handlers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/nicotion/battos/apps/api/internal/sysmetrics"
	"github.com/nicotion/battos/packages/core"
)

// Version info — se inyecta vía ldflags en build:
//   go build -ldflags "-X main.Version=v0.1.0 -X main.Commit=abc123 -X main.BuildDate=..."
//
// Por ahora son vars públicas para que main las setee.
var (
	Version   = "v0.1.0-alpha"
	Commit    = "dev"
	BuildDate = "unknown"
)

// SystemHandler maneja los endpoints de salud y estado del OS.
type SystemHandler struct {
	sampler  *sysmetrics.Sampler
	pingDB   func(context.Context) error // null si DB no configurada
	pingMem  func(context.Context) error // null si Memory no configurada
}

// NewSystemHandler crea el handler con sus dependencias.
//
// pingDB y pingMem son closures que el main inyecta — así desacoplamos
// el handler del paquete concreto (database/sql / memory.Core).
func NewSystemHandler(sampler *sysmetrics.Sampler, pingDB, pingMem func(context.Context) error) *SystemHandler {
	return &SystemHandler{sampler: sampler, pingDB: pingDB, pingMem: pingMem}
}

// writeJSON es una versión local para evitar import circular con package server.
// (server importa handlers, handlers no puede importar server.)
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = encodeJSON(w, v)
}

// Health responde un OK simple. Pensado para load balancers / healthchecks
// de Docker. No toca la DB ni otros subsistemas — eso es /status.
//
//	GET /health
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, core.HealthResponse{
		Status:    core.HealthOK,
		Timestamp: time.Now(),
	})
}

// Version devuelve la versión del binario, commit, build date y versión de Go.
//
//	GET /version
func (h *SystemHandler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, core.VersionResponse{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	})
}

// Status agrega el estado completo del OS:
//   - Versión.
//   - Salud de cada subsistema (config, sysmetrics, db, memory, ...).
//   - Snapshot de métricas en vivo (CPU/MEM/NET).
//
// Es lo que muestra `battos status` en la terminal y alimenta el panel
// principal del Command Center.
//
//	GET /status
func (h *SystemHandler) Status(w http.ResponseWriter, r *http.Request) {
	subsystems := []core.SubsystemHealth{
		{Name: "config", Status: core.HealthOK, Detail: "battos.yaml cargado"},
		{Name: "sysmetrics", Status: sysmetricsHealth(h.sampler)},
		checkSubsystem(r.Context(), "database", h.pingDB, "SQLite local conectado"),
		checkSubsystem(r.Context(), "memory", h.pingMem, "SQLite + FTS5 listo"),
	}

	resp := core.StatusResponse{
		Version: core.VersionResponse{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
		},
		Overall:    overallHealth(subsystems),
		Subsystems: subsystems,
		Metrics:    h.sampler.Latest(),
		Timestamp:  time.Now(),
	}

	writeJSON(w, http.StatusOK, resp)
}

// sysmetricsHealth reporta degraded si el sampler todavía no produjo datos.
func sysmetricsHealth(s *sysmetrics.Sampler) core.HealthStatus {
	m := s.Latest()
	if m.MemTotalMB == 0 && m.CPUPercent == 0 {
		return core.HealthDegraded
	}
	return core.HealthOK
}

// checkSubsystem ejecuta el ping y devuelve el SubsystemHealth correspondiente.
// Si la closure es nil → unknown (subsistema no inyectado).
// Si el ping responde rápido → ok. Si tarda o falla → degraded/down.
func checkSubsystem(ctx context.Context, name string, ping func(context.Context) error, okDetail string) core.SubsystemHealth {
	if ping == nil {
		return core.SubsystemHealth{Name: name, Status: core.HealthUnknown, Detail: "no inicializado"}
	}
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := ping(pingCtx)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return core.SubsystemHealth{
			Name: name, Status: core.HealthDown,
			Detail: err.Error(), LatencyMs: latency,
		}
	}
	return core.SubsystemHealth{
		Name: name, Status: core.HealthOK,
		Detail: okDetail, LatencyMs: latency,
	}
}

// overallHealth agrega los subsistemas en un único veredicto.
// Regla: si alguno está Down → Down. Si alguno Degraded → Degraded.
// Unknown se ignora (todavía no implementado, no es falla).
func overallHealth(subs []core.SubsystemHealth) core.HealthStatus {
	worst := core.HealthOK
	for _, s := range subs {
		switch s.Status {
		case core.HealthDown:
			return core.HealthDown
		case core.HealthDegraded:
			worst = core.HealthDegraded
		}
	}
	return worst
}

// StreamSystemMetrics inicia un canal SSE continuo que emite las métricas en vivo.
//
//	GET /events/system-metrics
func (h *SystemHandler) StreamSystemMetrics(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "SSE no disponible en este servidor", "code": 500}})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Primer evento inmediato
	h.emitMetrics(w, r.Context())
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			h.emitMetrics(w, r.Context())
			flusher.Flush()
		}
	}
}

func (h *SystemHandler) emitMetrics(w http.ResponseWriter, ctx context.Context) {
	subsystems := []core.SubsystemHealth{
		{Name: "config", Status: core.HealthOK, Detail: "battos.yaml cargado"},
		{Name: "sysmetrics", Status: sysmetricsHealth(h.sampler)},
		checkSubsystem(ctx, "database", h.pingDB, "SQLite local conectado"),
		checkSubsystem(ctx, "memory", h.pingMem, "SQLite + FTS5 listo"),
	}

	resp := core.StatusResponse{
		Version: core.VersionResponse{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
		},
		Overall:    overallHealth(subsystems),
		Subsystems: subsystems,
		Metrics:    h.sampler.Latest(),
		Timestamp:  time.Now(),
	}

	_ = writeSSEEvent(w, "system.metrics", resp)
}
