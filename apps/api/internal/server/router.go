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
	Config       *config.Config
	Logger       *slog.Logger
	System       *handlers.SystemHandler
	Memory       *handlers.MemoryHandler
	Work         *handlers.WorkHandler
	Knowledge    *handlers.KnowledgeHandler
	Registries   *handlers.RegistriesHandler
	Runtime      *handlers.RuntimeHandler
	Runs         *handlers.RunHandler
	Messages     *handlers.MessagesHandler
	Repositories *handlers.RepositoriesHandler
	NovaCore     *handlers.NovaCoreHandler
	Usage        *handlers.UsageHandler
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

	// Public endpoints stay available to process healthchecks.
	r.Get("/health", deps.System.Health)
	r.Get("/version", deps.System.Version)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware(deps.Config.Auth.Mode, deps.Config.APIToken))

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(30 * time.Second))

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

			// --- Work Board endpoints (Fase 3B) ---
			if deps.Work != nil {
				mountWorkRoutes(r, deps.Work)
			} else {
				mountUnavailableWorkRoutes(r)
			}

			// --- Knowledge Center endpoints (Fase 3B) ---
			if deps.Knowledge != nil {
				mountKnowledgeRoutes(r, deps.Knowledge)
			} else {
				mountUnavailableKnowledgeRoutes(r)
			}

			// --- Runtime/provider detection endpoints (Fase 4A) ---
			if deps.Registries != nil {
				mountRegistryRoutes(r, deps.Registries)
			} else {
				mountUnavailableRegistryRoutes(r)
			}

			// --- Runtime/provider detection endpoints (Fase 4A) ---
			if deps.Runtime != nil {
				mountRuntimeRoutes(r, deps.Runtime)
			} else {
				mountUnavailableRuntimeRoutes(r)
			}

			// --- Supervised run control plane (Fase 4B base) ---
			if deps.Messages != nil {
				mountMessageRoutes(r, deps.Messages)
			}
			if deps.Runs != nil {
				mountRunRoutes(r, deps.Runs)
			} else {
				mountUnavailableRunRoutes(r)
			}

			// --- Repositories endpoints (Fase 4C) ---
			if deps.Repositories != nil {
				mountRepositoryRoutes(r, deps.Repositories)
			} else {
				mountUnavailableRepositoryRoutes(r)
			}

			// --- NovaCore Assistant endpoints (Fase 5A) ---
			if deps.NovaCore != nil {
				mountNovaCoreRoutes(r, deps.NovaCore)
			} else {
				mountUnavailableNovaCoreRoutes(r)
			}

			// --- Usage/Token consumption endpoints (Fase 5B) ---
			if deps.Usage != nil {
				mountUsageRoutes(r, deps.Usage)
			} else {
				mountUnavailableUsageRoutes(r)
			}
		})

		// --- SSE streams (sin timeout HTTP para permitir reconexion del dashboard) ---
		r.Get("/events/system-metrics", deps.System.StreamSystemMetrics)
		if deps.Runs != nil {
			mountRunEventRoutes(r, deps.Runs)
		} else {
			mountUnavailableRunEventRoutes(r)
		}
	})

	// 404 prolijo en JSON, no HTML.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotFound, "endpoint no encontrado")
	})

	return r
}

func mountRunRoutes(r chi.Router, runs *handlers.RunHandler) {
	r.Get("/runs", runs.ListRuns)
	r.Post("/runs", runs.CreateRun)
	r.Get("/runs/{id}", runs.GetRun)
	r.Post("/runs/{id}/approvals", runs.ApproveRunAction)
	r.Post("/runs/{id}/cancel", runs.CancelRun)
	r.Get("/runs/{id}/logs", runs.ListRunLogs)
	r.Get("/runs/{id}/artifacts", runs.ListRunArtifacts)
	r.Get("/runs/{id}/diff", runs.GetRunDiff)
}

func mountRunEventRoutes(r chi.Router, runs *handlers.RunHandler) {
	r.Get("/events/runs/{id}", runs.StreamRunEvents)
}

func mountMessageRoutes(r chi.Router, messages *handlers.MessagesHandler) {
	r.Post("/agent-messages", messages.SendMessage)
	r.Post("/agent-messages/{id}/read", messages.MarkRead)
	r.Get("/agents/{id}/messages", messages.ListInbox)
}

func mountUnavailableRunRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Runs no disponible: verifica la base SQLite local")
	}
	r.Get("/runs", unavailable)
	r.Post("/runs", unavailable)
	r.Get("/runs/{id}", unavailable)
	r.Post("/runs/{id}/approvals", unavailable)
	r.Post("/runs/{id}/cancel", unavailable)
	r.Get("/runs/{id}/logs", unavailable)
	r.Get("/runs/{id}/artifacts", unavailable)
	r.Get("/runs/{id}/diff", unavailable)
}

func mountUnavailableRunEventRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Run events no disponible: verifica la base SQLite local")
	}
	r.Get("/events/runs/{id}", unavailable)
}

func mountRuntimeRoutes(r chi.Router, runtimeHandler *handlers.RuntimeHandler) {
	r.Get("/runtime-adapters", runtimeHandler.ListRuntimeAdapters)
	r.Post("/runtime-adapters/detect", runtimeHandler.DetectRuntimeAdapters)
	r.Get("/cli-tools", runtimeHandler.ListCLITools)
	r.Get("/providers", runtimeHandler.ListProviders)
	r.Post("/providers/detect", runtimeHandler.DetectProviders)
}

func mountRegistryRoutes(r chi.Router, registries *handlers.RegistriesHandler) {
	r.Get("/agents", registries.ListAgents)
	r.Post("/agents", registries.CreateAgent)
	r.Get("/skills", registries.ListSkills)
}

func mountUnavailableRegistryRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Agents/skills no disponible: verifica la base SQLite local")
	}
	r.Get("/agents", unavailable)
	r.Post("/agents", unavailable)
	r.Get("/skills", unavailable)
}

func mountUnavailableRuntimeRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Runtime detection no disponible: verifica la base SQLite local")
	}
	r.Get("/runtime-adapters", unavailable)
	r.Post("/runtime-adapters/detect", unavailable)
	r.Get("/cli-tools", unavailable)
	r.Get("/providers", unavailable)
	r.Post("/providers/detect", unavailable)
}

func mountWorkRoutes(r chi.Router, work *handlers.WorkHandler) {
	r.Get("/domains", work.ListDomains)
	r.Post("/domains", work.CreateDomain)
	r.Get("/domains/{id}", work.GetDomain)
	r.Patch("/domains/{id}", work.UpdateDomain)

	r.Get("/projects", work.ListProjects)
	r.Post("/projects", work.CreateProject)
	r.Get("/projects/{id}", work.GetProject)
	r.Patch("/projects/{id}", work.UpdateProject)

	r.Get("/goals", work.ListGoals)
	r.Post("/goals", work.CreateGoal)
	r.Get("/goals/{id}", work.GetGoal)
	r.Patch("/goals/{id}", work.UpdateGoal)

	r.Get("/tasks", work.ListTasks)
	r.Post("/tasks", work.CreateTask)
	r.Get("/tasks/{id}", work.GetTask)
	r.Patch("/tasks/{id}", work.UpdateTask)
}

func mountKnowledgeRoutes(r chi.Router, knowledge *handlers.KnowledgeHandler) {
	r.Get("/knowledge/workspaces", knowledge.ListWorkspaces)
	r.Post("/knowledge/workspaces", knowledge.CreateWorkspace)
	r.Get("/journals", knowledge.ListJournals)
	r.Post("/journals", knowledge.CreateJournal)
	r.Get("/artifacts", knowledge.ListArtifacts)
	r.Post("/artifacts", knowledge.CreateArtifact)
}

func mountUnavailableKnowledgeRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Knowledge Center no disponible: verifica la base SQLite local")
	}
	r.Get("/knowledge/workspaces", unavailable)
	r.Post("/knowledge/workspaces", unavailable)
	r.Get("/journals", unavailable)
	r.Post("/journals", unavailable)
	r.Get("/artifacts", unavailable)
	r.Post("/artifacts", unavailable)
}

func mountUnavailableWorkRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Work Board no disponible: verifica la base SQLite local")
	}
	r.Get("/domains", unavailable)
	r.Post("/domains", unavailable)
	r.Get("/domains/{id}", unavailable)
	r.Patch("/domains/{id}", unavailable)

	r.Get("/projects", unavailable)
	r.Post("/projects", unavailable)
	r.Get("/projects/{id}", unavailable)
	r.Patch("/projects/{id}", unavailable)

	r.Get("/goals", unavailable)
	r.Post("/goals", unavailable)
	r.Get("/goals/{id}", unavailable)
	r.Patch("/goals/{id}", unavailable)

	r.Get("/tasks", unavailable)
	r.Post("/tasks", unavailable)
	r.Get("/tasks/{id}", unavailable)
	r.Patch("/tasks/{id}", unavailable)
}

func mountRepositoryRoutes(r chi.Router, repos *handlers.RepositoriesHandler) {
	r.Get("/repositories", repos.ListRepositories)
	r.Post("/repositories", repos.ConnectRepository)
}

func mountUnavailableRepositoryRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Repositories no disponible: verifica la base SQLite local")
	}
	r.Get("/repositories", unavailable)
	r.Post("/repositories", unavailable)
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

func mountNovaCoreRoutes(r chi.Router, nova *handlers.NovaCoreHandler) {
	r.Get("/novacore/conversations", nova.ListConversations)
	r.Get("/novacore/conversations/{id}/messages", nova.GetConversationMessages)
	r.Post("/novacore/chat", nova.Chat)
}

func mountUnavailableNovaCoreRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "NovaCore no disponible: verifica la base SQLite local")
	}
	r.Get("/novacore/conversations", unavailable)
	r.Get("/novacore/conversations/{id}/messages", unavailable)
	r.Post("/novacore/chat", unavailable)
}

func mountUsageRoutes(r chi.Router, usage *handlers.UsageHandler) {
	r.Get("/usage/overview", usage.Overview)
	r.Get("/usage/runs/{id}", usage.GetUsageByRun)
}

func mountUnavailableUsageRoutes(r chi.Router) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusServiceUnavailable, "Usage telemetry no disponible: verifica la base SQLite local")
	}
	r.Get("/usage/overview", unavailable)
	r.Get("/usage/runs/{id}", unavailable)
}
