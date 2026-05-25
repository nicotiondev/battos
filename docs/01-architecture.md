# 01 — Arquitectura

## Vista por capas

```
┌──────────────────────────────────────────────────────────┐
│  Frontend (Next.js 15 + TS + Tailwind + shadcn + Tremor) │
│  Command Center · Projects · Agents · Skills · CLI · ... │
└──────────────────────────────────────────────────────────┘
                          │ HTTP/JSON + SSE
┌──────────────────────────────────────────────────────────┐
│  API (Go + chi)                                          │
│  ├─ handlers/    endpoints REST                          │
│  ├─ server/      router, middleware, SSE                 │
│  ├─ registry/    carga projects/agents/skills desde MD   │
│  ├─ detector/    detección de CLIs                       │
│  ├─ providers/   estado de OpenAI/Anthropic/Google/...   │
│  ├─ memory/      Memory Core (SQLite+FTS5)               │
│  ├─ sysmetrics/  CPU/MEM/NET sampler                     │
│  ├─ store/       sqlc-generated (Postgres)               │
│  └─ config/      viper loader                            │
└──────────────────────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
┌──────────────┐  ┌───────────────┐  ┌──────────────┐
│ PostgreSQL   │  │ SQLite (FTS5) │  │ Filesystem   │
│  - projects  │  │  memory_items │  │  agents/*.md │
│  - agents    │  │  + índice     │  │  skills/*/   │
│  - skills    │  │     FTS       │  │  config/*.yml│
│  - models    │  └───────────────┘  └──────────────┘
│  - cli_tools │
│  - executions│
│  - usage     │
└──────────────┘
                          │
┌──────────────────────────────────────────────────────────┐
│  CLI `battos` (Go + cobra + lipgloss)                    │
│  Cliente HTTP del API, formatea con colores              │
└──────────────────────────────────────────────────────────┘
```

## Por qué tres procesos (API, CLI, Web)

- **API** es el cerebro. Lectura y escritura única a la DB. Expone HTTP+SSE.
- **CLI** es un cliente liviano. No accede a DB directamente. Llama al API.
- **Web** también es cliente. No accede a DB directamente.

Beneficio: la lógica vive en un solo lado. Si mañana sale otro cliente (Telegram bot, MCP server), solo necesita hablar HTTP con el API.

## Modelo de datos (resumen)

Tablas Postgres (v0.1):
- `projects`, `agents`, `skills`
- `providers`, `models`
- `cli_tools`, `mcp_connections`
- `executions`, `usage_events`, `system_logs`

DB del Memory Core (SQLite):
- `memory_items` + tabla virtual FTS5 `memory_items_fts`

Detalle completo en `03-data-model.md` (se llena en Fase 2).

## Streaming en vivo

El Command Center muestra ~30 series concurrentes (CPU/MEM/NET, tokens por proyecto, status de CLIs, health, etc.). Se resuelve con **SSE** desde `GET /events/...`. Ver `adr/0002-sse-no-websockets.md`.

## Por qué no hay Redis/queue en v0.1

v0.1 es read-only. No hay jobs asíncronos todavía. Cuando entren workers (v0.2) se agrega **river** (Postgres-backed, no necesita Redis) o `asynq` (Redis-backed). Decisión postergada.

## Despliegue

- **Dev local**: `docker compose up -d` levanta `api` + `postgres`. Frontend con `npm run dev`. CLI con `go run` o binario compilado en `./bin/`.
- **VPS**: el mismo `docker-compose.yml` + Traefik/Nginx delante. Detalle en `08-install-vps.md`.

## Lo que viene en v0.2 (no aquí)

- Workers + queue para ejecución asíncrona.
- LiteLLM como sidecar para llamadas LLM con routing/fallback.
- Model Advisor con políticas reales y aprendizaje.
- Usage tracking real (no placeholder).
- MCP server propio expuesto.
- HITL (aprobación humana para acciones de riesgo).
