# 02 — Decisiones de stack

Por qué cada elección. Los detalles "estilo ADR" están en `adr/`. Este doc es la vista resumida.

## Backend y worker: Go (no Python, no Node, no Rust)

**Decisión**: Go para API, CLI y worker de runs de v0.1. El workspace/API se validan actualmente con Go 1.25; módulos auxiliares todavía pueden declarar compatibilidad mínima con Go 1.23.

**Razones**:
1. **Single binary** — el CLI `battos` se distribuye como un ejecutable sin runtime. Crítico para self-hosted.
2. **Concurrencia nativa** — el Command Center mostrará ~30 streams concurrentes + watch de procesos CLI. Goroutines lo resuelven trivialmente.
3. **Footprint bajo** — API + worker en Go: ~30–80 MB RAM idle. En Python sería 3–5× más.
4. **Ecosistema infra** — los SDKs oficiales de Anthropic, OpenAI, Google, MCP y Docker existen en Go y son first-class.
5. **Curva razonable** — más fácil que Rust, más rápido y robusto que Python para esta categoría de software (orquestación, gateways, CLIs).

**Tradeoff aceptado**: la primera vez con Go cuesta 2–3 semanas extra. A cambio se gana toda la operación a partir de ahí.

ADR: `adr/0001-go-stack.md`.

## Frontend: Next.js 16 + TypeScript

**Decisión**: Next.js 15 (App Router) + Tailwind + shadcn/ui + Tremor.

**Razones**:
1. **Tremor** da exactamente los tiles, sparklines, gauges y panels del mockup. Cero reinvención.
2. **shadcn/ui** cubre el resto (tablas, dropdowns, dialogs) con tema dark consistente.
3. **App Router** + Server Components hace que la mayoría de páginas sean rápidas sin estado de cliente innecesario.
4. **TanStack Query** + `EventSource` cubre fetching y streaming sin librerías exóticas.

ADR: `adr/0018-dashboard-nextjs-16.md`.

## DB layer: sqlc (no ORM)

**Decisión**: sqlc genera código Go tipado desde archivos `.sql`. No GORM, no ent, no SQLBoiler.

**Razones**:
1. **SQL puro** — el desarrollador escribe SQL real, no DSL. Cuando algo no funciona, el plan de query es transparente.
2. **Tipado fuerte** — sqlc lee el schema y genera structs Go que matchean exactamente.
3. **Sin runtime overhead** — el código generado es el que harías a mano. No hay reflection ni magia.
4. **Pedagógico** — para alguien nuevo en Go, sqlc enseña tanto SQL como Go idiomático. Mejor que un ORM mágico.

**Tradeoff aceptado**: el ciclo "editar schema → regenerar → ajustar handlers" es manual. Lo automatiza `scripts/generate.ps1`.

ADR: `adr/0005-sqlc-vs-orm.md` (Fase 2).

## Realtime: SSE, no WebSockets

**Decisión**: Server-Sent Events para todos los streams del dashboard.

**Razones**:
1. **Más simple** — HTTP plano, sin handshake, sin librerías cliente exóticas. `EventSource` está en cada navegador.
2. **Suficiente** — el dashboard solo necesita server→client. No hay client→server por streaming.
3. **Resiliente** — reconnect automático del browser.
4. **Compatible con HTTP/2 y proxies** — Traefik/Nginx no requieren config especial.

**Cuándo lo cambiaríamos**: si v0.5 expone un terminal interactivo real (input + output bidireccional). Hasta ahí, SSE alcanza.

ADR: `adr/0002-sse-no-websockets.md`.

## Memory Core propio (no Engram dep)

**Decisión**: implementar Memory Core en Go con SQLite + FTS5 dentro del API.

**Razones**:
1. **Control total** — schema de `memory_items` adaptado a BattOS (project_id, agent_id, type, etc.).
2. **Sin proceso externo** — Engram requiere correr otro binario y mantenerlo sincronizado. Para v0.1 es complicación innecesaria.
3. **Single binary** — alinea con la filosofía Go: una cosa que se instala y corre.
4. **modernc.org/sqlite** es driver puro Go (sin CGo) → mantiene cross-compilation simple.

Engram queda como **referencia de diseño** (estructura de observaciones, FTS, judgment) pero no como dependencia.

ADR: `adr/0004-memory-core-propio.md` (Fase 2).

## Ejecucion supervisada: contenedores y SQLite

**Decision**: v0.1 ejecuta Claude Code y Codex mediante adapters aprobados,
siempre en un contenedor efimero por run y con aprobacion humana. El estado de
runs y sus aprobaciones vive en SQLite (`data/battos.db`, ADR-0021); para la
primera version el worker Go reclama trabajo desde esa base sin introducir Redis.

**Razones**:
1. Aisla el filesystem y limita el radio de una ejecucion.
2. Permite auditar red, logs, artefactos, diff, commit y push.
3. Mantiene una operacion self-hosted sencilla para un usuario/equipo pequeno.
4. Deja abierta una cola especializada solo si la escala real la exige.

ADR: `adr/0011-v01-ejecucion-supervisada.md`.

## Contratos: OpenAPI + oapi-codegen

**Decisión**: `packages/openapi/openapi.yaml` es la fuente de verdad. oapi-codegen genera:
- Server stubs en Go (interface que los handlers implementan).
- Cliente tipado en Go para el CLI (`apps/cli/internal/client`).
- Cliente tipado en TypeScript para el frontend (`apps/web/lib/api-client.ts`).

**Razones**:
1. **Un cambio en el contrato** se refleja automáticamente en server, CLI y frontend.
2. **Documentación auto-generada** (Swagger UI) gratis.
3. **Sin tRPC** (que ataría todo a TypeScript).

## Lo que NO se eligió y por qué

| Descartado | Por qué |
|---|---|
| **Rust** | Ganancia marginal (~10% perf) sobre Go, costo 2–3× en desarrollo. No vale acá. |
| **Python + FastAPI** | El mockup exige streaming pesado y footprint chico. Python sufre. |
| **Node + NestJS** | Ecosistema AI más débil que Python; performance peor que Go. Peor de dos mundos. |
| **GORM / ent** | ORM mágico esconde el SQL. sqlc enseña mejor y es más simple. |
| **Celery / RabbitMQ / Redis temprano** | v0.1 persiste y reclama runs desde SQLite; una cola extra solo entra si la carga real lo justifica. |
| **GraphQL** | OpenAPI auto-generado da los mismos beneficios con menos overhead conceptual. |
| **tRPC** | Encierra todo en TypeScript. Backend en Go ya descarta esto. |
| **Monorepo con Turborepo/Nx** | Complejidad innecesaria. `go.work` + carpetas planas alcanzan. |
| **Kubernetes** | Docker Compose hasta que duela. |
