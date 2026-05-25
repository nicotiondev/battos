# CLAUDE.md — BattOS

Instrucciones para futuras sesiones de Claude Code que trabajen en este repo.

---

## Contexto rápido

BattOS es una capa agentic self-hosted que orquesta proyectos, agentes, skills, modelos, memoria, MCP, CLIs y workflows.

**Estado**: v0.1.0 en construcción. Ver `docs/10-roadmap.md` para qué entra y qué no.

**Docs vivos**: cada cambio relevante actualiza algún archivo en `docs/`. Las decisiones técnicas no triviales van como ADR en `docs/adr/NNNN-<slug>.md`.

---

## Stack

- **Go 1.23** (backend + CLI + workers)
- **Next.js 15 + TS** (frontend)
- **PostgreSQL 16** + **sqlc** (no ORM)
- **SQLite + FTS5** (Memory Core, embebido en el API)
- **chi** (router), **goose** (migraciones), **cobra** (CLI), **viper** (config)
- **OpenAPI + oapi-codegen** (contratos tipados)
- **SSE** para streaming al dashboard (no WebSockets)

Detalle de "por qué" en `docs/02-stack-decisions.md` y `docs/adr/`.

---

## Layout

```
apps/api/         → server HTTP Go
apps/cli/         → binario battos
apps/web/         → Next.js dashboard
packages/core/    → tipos compartidos Go
packages/openapi/ → openapi.yaml + código generado
agents/           → MD de agentes (cargados al boot)
skills/           → MD de skills
config/           → YAML de runtime
docs/             → documentación viva
infra/            → Docker + .env.example
scripts/          → helpers PowerShell
data/             → DBs locales, logs, workspaces (gitignored)
```

---

## Comandos comunes

> Estos comandos se implementan a lo largo de las fases. Si todavía no existen es porque la fase correspondiente no se cerró.

```powershell
# Build
go build ./apps/api/cmd/api
go build ./apps/cli/cmd/battos
cd apps/web; npm run build

# Run dev
cd apps/api; go run ./cmd/api
cd apps/cli; go run ./cmd/battos status
cd apps/web; npm run dev

# Tests
go test ./...
cd apps/web; npm run typecheck

# Generators (cuando se agreguen sqlc/oapi-codegen)
./scripts/generate.ps1

# Seeds
./scripts/seed.ps1

# Docker
docker compose -f infra/docker-compose.yml up -d
docker compose -f infra/docker-compose.yml down
```

---

## Convenciones

### Go
- Módulos: `github.com/nicotion/battos/<path>`.
- `internal/` para código que NO se exporta fuera de su app.
- Errores: envolver con `fmt.Errorf("contexto: %w", err)`. Nunca tragar errores.
- Logs: `slog` de la stdlib, formato JSON en producción, texto en dev.
- Context: todo handler/función IO acepta `context.Context` como primer argumento.
- DB: nunca SQL strings inline en handlers. Todo pasa por `internal/store/` generado por sqlc.

### Frontend
- App Router (no pages).
- Server Components por defecto. `'use client'` solo cuando hace falta (state, EventSource, eventos).
- Fetching: TanStack Query.
- Streaming: hook `useSSE` en `lib/sse.ts`.
- Tipos del API: `lib/api-client.ts` generado por oapi-codegen. **No editar a mano.**
- Componentes: shadcn/ui + Tremor. Si necesitás algo custom, va en `components/command-center/` o equivalente.

### Naming
- Archivos Go: `snake_case.go`.
- Archivos TS/TSX: `kebab-case.tsx`.
- Tablas SQL: `plural_snake_case`.
- Endpoints REST: `/recurso` (plural), `/recurso/{id}` para específico.

---

## Lo que NO se toca sin pensar

- **`packages/openapi/openapi.yaml`** — fuente de verdad del contrato. Cambiar acá rompe server y client. Ciclo: editar → `./scripts/generate.ps1` → revisar diffs.
- **`apps/api/migrations/*.sql`** — son append-only. Nunca editar una migración ya aplicada. Crear una nueva.
- **`apps/api/queries/*.sql`** — los cambios disparan regeneración de `internal/store/`. No editar el código generado.
- **`config/*.yaml`** — son contratos con el runtime. Romper formato rompe el boot.

---

## Estrategia de documentación

1. **Cada fase cerrada** → actualiza al menos un doc en `docs/`.
2. **Cada decisión técnica no trivial** → ADR nuevo en `docs/adr/NNNN-<slug>.md` (numeración correlativa).
3. **Cada sesión de Claude Code** debería terminar llamando `mem_save` con resumen y, si la sesión cerró una fase, también `mem_session_summary`.
4. **Aprendizajes operativos** (cómo armar algo en Go, gotchas) van a `docs/go-primer.md` o `learnings.md` locales del módulo.

Memoria del agente: ya existe en Engram (proyecto `battos`). Antes de empezar trabajo no trivial, `mem_search` con keywords del módulo.

---

## Fases

| Fase | Objetivo | Estado |
|---|---|---|
| 0 | Bootstrap del repo | ✅ Completado |
| 1 | `battos status` funcional | En curso |
| 2 | DB + Memory Core | Pending |
| 3 | Registries API + CLI | Pending |
| 4 | CLI Detector + Providers | Pending |
| 5 | Frontend Command Center | Pending |
| 6 | Polish + tag v0.1.0 | Pending |

Plan completo: `C:\Users\nicoa\.claude\plans\adelante-y-haz-un-hidden-meerkat.md`.

---

## Decisiones críticas ya cerradas

- **Repo en `C:\Users\nicoa\Desktop\CLAUDE CODE\battos`** — movido desde `C:\dev\battos` el 2026-05-25. No vive en `G:\` porque Google Drive virtual no soporta dev artifacts ni carpetas en raíz. `G:\Mi unidad\BattOS\` se mantiene para el spec original (`battOS.md`) + mockups del producto.
- **Stack Go + TS** — ver `docs/adr/0001-go-stack.md`.
- **Memory Core propio** (no Engram dep) — ver `docs/adr/0004-memory-core-propio.md` (Fase 2).
- **SSE, no WebSockets** — ver `docs/adr/0002-sse-no-websockets.md` (Fase 1).
- **sqlc, no ORM** — ver `docs/adr/0005-sqlc-vs-orm.md` (Fase 2).
- **Scope v0.1 read-only** — sin ejecución, sin LLM calls. Ver `docs/adr/0006-no-llm-calls-en-v0.1.md` (Fase 4).
