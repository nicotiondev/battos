// Package handlers contiene los HTTP handlers del API.
//
// Convención: un archivo por área (system, projects, agents, ...).
// Cada handler es un método sobre una struct que recibe sus dependencias
// vía constructor (NewXxxHandler). Esto facilita inyectar mocks en tests.
package handlers

import (
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
	sampler *sysmetrics.Sampler
}

// NewSystemHandler crea el handler con sus dependencias.
func NewSystemHandler(sampler *sysmetrics.Sampler) *SystemHandler {
	return &SystemHandler{sampler: sampler}
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
		// db, memory, registries — se sumarán en Fase 2/3.
		{Name: "database", Status: core.HealthUnknown, Detail: "Fase 2"},
		{Name: "memory", Status: core.HealthUnknown, Detail: "Fase 2"},
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
