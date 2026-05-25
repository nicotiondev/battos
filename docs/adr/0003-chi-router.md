# ADR-0003: chi como router HTTP (no gin, echo, fiber, ni stdlib pelado)

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

Necesitamos un router HTTP para el API. Las opciones razonables en Go son:

- **`net/http` + `http.ServeMux`** (stdlib pelado).
- **chi** (`github.com/go-chi/chi/v5`).
- **gin** (`github.com/gin-gonic/gin`).
- **echo** (`github.com/labstack/echo/v4`).
- **fiber** (`github.com/gofiber/fiber/v3`).

Criterios para BattOS:
1. Sub-routers tipados y montables (`/projects`, `/agents`, etc. cada uno con su middleware).
2. Compatible con `http.Handler` estándar — sin acoplarse a una abstracción propia.
3. Middleware composable y reutilizable.
4. Mínima magia, código del API limpio y leíble.
5. Comunidad activa, sin abandonware.

## Decision

**chi v5.**

## Consequences

### Positivas

- **`http.Handler` puro** — no inventa un `Context` propio como gin. Cualquier middleware estándar (`net/http`) funciona directo.
- **Sub-routers** con `r.Route("/projects", func(r chi.Router) { ... })`. Encaja perfecto con la separación por área de handlers.
- **Middleware curado** en `chi/middleware`: RequestID, RealIP, Recoverer, Timeout, Logger — los justos, sin bloat.
- **Sin dependencias transitivas pesadas.** chi es básicamente stdlib + algunas helpers.
- **Bien mantenido** — releases regulares, v5 estable, repo activo.
- **API estable** — proyectos importantes (Caddy, Heroku, Cloudflare) lo usan en producción.

### Negativas

- **Sin features "batería incluida"** tipo binding/validation automático que gin trae. Mitigación: la validación de inputs vivirá en cada handler explícitamente o vía `go-playground/validator` cuando aparezca la primera necesidad.
- **No incluye renderer** (templates, JSON helpers). Tenemos un helper `server.WriteJSON()` propio de 5 líneas.

### Neutrales

- El learning curve es mínimo si conocés `net/http`. La diferencia es `r.Get("/x", handler)` vs `mux.HandleFunc("GET /x", handler)`.
- Para el tipado de path params: `chi.URLParam(r, "id")` — manual, no estructural. Aceptable.

## Alternatives considered

### `net/http` + `http.ServeMux` (stdlib pelado)
- ✅ Cero deps.
- ✅ Go 1.22+ trae routing nativo aceptable.
- ❌ Sin sub-routers prolijos.
- ❌ Sin middleware composable estándar (lo armás vos cada vez).
- ❌ Para 50+ endpoints futuros se vuelve verboso.
- **Lo descartamos** porque chi cuesta una dep insignificante y nos ahorra mucho boilerplate.

### gin
- ✅ Más popular en el ecosistema Go.
- ❌ Inventa su propio `gin.Context` — todo middleware se acopla a gin.
- ❌ Decisiones de diseño cuestionables (panics como flujo normal en algunos casos).
- ❌ La salida JSON es magia (`c.JSON(200, ...)`), menos explícita.
- **Lo descartamos** porque queremos `http.Handler` puro y handlers explícitos.

### echo
- ✅ Performance buena, features completas.
- ❌ Mismo problema que gin: contexto propio.
- ❌ Menos foco en simplicidad.
- **Lo descartamos** por la misma razón que gin.

### fiber
- ✅ Performance top de Go.
- ❌ No usa `net/http`: está basado en `fasthttp`. Eso rompe compatibilidad con middleware estándar y herramientas de profiling.
- ❌ Si en el futuro queremos streaming SSE o subir/bajar imágenes/archivos grandes, fasthttp tiene quirks.
- **Lo descartamos** porque la incompatibilidad con `net/http` es un costo enorme a largo plazo.

## Implementation notes

- Router montado en `apps/api/internal/server/router.go`.
- Middleware aplicado globalmente: RequestID, RealIP, Recoverer, CORS, StructuredLogger, Timeout.
- Sub-routers se montan con `r.Route(...)` cuando se agreguen los endpoints de Fase 3.
- Helper `server.WriteJSON()` y `server.WriteError()` para no repetir headers/encoding.

## Related

- `apps/api/internal/server/router.go` — la implementación.
- `docs/04-api-reference.md` — endpoints registrados.
- `ADR-0001` — decisión de Go como stack principal.
