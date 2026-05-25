# 02 — Decisiones de stack

Por qué cada elección. Los detalles "estilo ADR" están en `adr/`. Este doc es la vista resumida.

## Backend: Go (no Python, no Node, no Rust)

**Decisión**: Go 1.23 para API, CLI y workers futuros.

**Razones**:
1. **Single binary** — el CLI `battos` se distribuye como un ejecutable sin runtime. Crítico para self-hosted.
2. **Concurrencia nativa** — el Command Center mostrará ~30 streams concurrentes + watch de procesos CLI. Goroutines lo resuelven trivialmente.
3. **Footprint bajo** — API + worker en Go: ~30–80 MB RAM idle. En Python sería 3–5× más.
4. **Ecosistema infra** — los SDKs oficiales de Anthropic, OpenAI, Google, MCP y Docker existen en Go y son first-class.
5. **Curva razonable** — más fácil que Rust, más rápido y robusto que Python para esta categoría de software (orquestación, gateways, CLIs).

**Tradeoff aceptado**: la primera vez con Go cuesta 2–3 semanas extra. A cambio se gana toda la operación a partir de ahí.

ADR: `adr/0001-go-stack.md`.

## Frontend: Next.js 15 + TypeScript

**Decisión**: Next.js 15 (App Router) + Tailwind + shadcn/ui + Tremor.

**Razones**:
1. **Tremor** da exactamente los tiles, sparklines, gauges y panels del mockup. Cero reinvención.
2. **shadcn/ui** cubre el resto (tablas, dropdowns, dialogs) con tema dark consistente.
3. **App Router** + Server Components hace que la mayoría de páginas sean rápidas sin estado de cliente innecesario.
4. **TanStack Query** + `EventSource` cubre fetching y streaming sin librerías exóticas.

ADR: `adr/0007-frontend-stack.md` (Fase 5).

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
| **Celery / RabbitMQ** | Overkill. Si en v0.2 hace falta queue, `river` (Postgres) o `asynq` (Redis) alcanzan. |
| **GraphQL** | OpenAPI auto-generado da los mismos beneficios con menos overhead conceptual. |
| **tRPC** | Encierra todo en TypeScript. Backend en Go ya descarta esto. |
| **Monorepo con Turborepo/Nx** | Complejidad innecesaria. `go.work` + carpetas planas alcanzan. |
| **Kubernetes** | Docker Compose hasta que duela. |
