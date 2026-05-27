// Package server configura el router chi, middleware compartido y helpers
// de respuesta (JSON, errores, SSE).
//
// Los handlers viven en internal/handlers/. Este paquete solo arma el árbol
// de rutas y aplica middleware.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/handlers"
)

// Deps agrupa las dependencias que los handlers necesitan inyectadas.
// Se llena en cmd/api/main.go y se pasa a NewRouter.
type Deps struct {
	Config *config.Config
	Logger *slog.Logger
	System *handlers.SystemHandler
	Memory *handlers.MemoryHandler
}

// NewRouter construye el router chi con middleware base y todas las rutas.
//
// Middleware aplicado a todas las requests:
//   - RequestID: agrega header X-Request-Id (útil para trazar logs).
//   - RealIP: respeta X-Forwarded-For si viene de un proxy confiable.
//   - Recoverer: convierte panics en 500 sin matar el proceso.
//   - Timeout: 30s por defecto (SSE/streams se montan en sub-routers sin este).
//   - CORS: orígenes definidos en battos.yaml.
//   - StructuredLogger: log JSON con duración y status code por request.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(deps.Config.API.CORSOrigins))
	r.Use(structuredLogger(deps.Logger))
	r.Use(middleware.Timeout(30 * time.Second))

	// Public endpoints stay available to process healthchecks.
	r.Get("/health", deps.System.Health)
	r.Get("/version", deps.System.Version)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(deps.Config.Auth.Mode, deps.Config.APIToken))

		// --- System endpoint (Fase 1) ---
		r.Get("/status", deps.System.Status)

		// --- Memory Core endpoints (Fase 2) ---
		if deps.Memory != nil {
			r.Route("/memory", func(r chi.Router) {
				r.Get("/recent", deps.Memory.Recent)
				r.Post("/search", deps.Memory.Search)
				r.Post("/save", deps.Memory.Save)
				r.Get("/stats", deps.Memory.Stats)
				r.Get("/{id}", deps.Memory.GetByID)
			})
		}
	})

	// 404 prolijo en JSON, no HTML.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotFound, "endpoint no encontrado")
	})

	return r
}

func authMiddleware(mode, token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mode == "disabled" {
				next.ServeHTTP(w, r)
				return
			}
			const prefix = "Bearer "
			auth := r.Header.Get("Authorization")
			if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix ||
				subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), []byte(token)) != 1 {
				WriteError(w, http.StatusUnauthorized, "token de acceso invalido o ausente")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WriteJSON serializa v como JSON y responde con el status code dado.
// Si la serialización falla, escribe un 500 sin payload (para no recursionar).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Si falla acá ya respondimos status — solo loguear.
		slog.Error("encoding json response", "error", err)
	}
}

// WriteError responde un error estructurado en JSON.
// Formato: {"error": {"message": "...", "code": <http_status>}}
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": msg,
			"code":    status,
		},
	})
}

// corsMiddleware permite los orígenes configurados en battos.yaml.
// Simple: no usamos rs/cors para mantener deps mínimas.
func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// structuredLogger emite un JSON log por request con método, ruta, status y duración.
func structuredLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http.request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
