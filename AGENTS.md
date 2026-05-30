# AGENTS.md - BattOS

Instrucciones para futuras sesiones de Codex que trabajen en este repo.

## Contexto rapido

BattOS es un Mission Control agentic self-hosted que organiza trabajo,
conocimiento y agentes, y ejecuta tareas de forma supervisada.

**Estado**: v0.1.0 en construccion. Fases 0-3A cerradas; Fase 3B tiene
persistencia y Work Board API/CLI, y continua con Knowledge Center. La especificacion final vive en
`docs/14-producto-final-y-roadmap.md`; el avance implementable en
`docs/10-roadmap.md`.

**Docs vivos**: cada cambio relevante actualiza algun archivo en `docs/`. Las
decisiones tecnicas no triviales van como ADR en
`docs/adr/NNNN-<slug>.md`.

## Stack

- **Go** (API, CLI y worker de runs)
- **Next.js 15 + TS** (dashboard)
- **PostgreSQL 16** + **sqlc** (fuente operacional; no ORM)
- **SQLite + FTS5** (Memory Core embebido en API)
- **Docker** (contenedor efimero por run)
- **chi**, **goose**, **cobra**, **viper**
- **OpenAPI + oapi-codegen** (contratos tipados)
- **SSE** para streaming al dashboard (no WebSockets en v0.1)

## Layout

```text
apps/api/         -> server HTTP y futuro worker Go
apps/cli/         -> binario battos
apps/web/         -> Next.js dashboard (pendiente)
packages/core/    -> tipos compartidos Go
packages/openapi/ -> openapi.yaml vigente + codigo generado posterior
agents/           -> definiciones MD de agentes
skills/           -> definiciones MD de skills
config/           -> YAML de runtime
docs/             -> documentacion viva y ADRs
infra/            -> Docker y .env.example
scripts/          -> helpers PowerShell
data/             -> DBs, logs, repos, runs y artefactos locales (gitignored)
```

## Comandos comunes

> Algunos comandos solo existen cuando la fase correspondiente se implemente.

```powershell
go build ./apps/api/cmd/api
go build ./apps/cli/cmd/battos

cd apps/api; go run ./cmd/api
cd apps/cli; go run ./cmd/battos status
cd apps/cli; go run ./cmd/battos shell
cd apps/cli; go run ./cmd/battos memory stats
cd apps/cli; go run ./cmd/battos project list

go test ./apps/api/... ./apps/cli/... ./packages/core/...

cd apps/api; sqlc generate
docker compose -f infra/docker-compose.yml up -d
docker compose -f infra/docker-compose.yml down
```

## Convenciones

### Go

- Modulos: `github.com/nicotion/battos/<path>`.
- Usar `internal/` para codigo no exportado fuera de su app.
- Envolver errores con `fmt.Errorf("contexto: %w", err)`.
- Usar `slog`: JSON en produccion, texto en dev.
- Funciones con IO reciben `context.Context` primero.
- SQL solo en queries/store y generado con sqlc; nunca inline en handlers.

### Frontend

- App Router; Server Components por defecto.
- TanStack Query para fetching y `useSSE` en `lib/sse.ts` para streaming.
- El cliente API generado no se edita a mano.
- UI con shadcn/ui + Tremor.

### Naming

- Go: `snake_case.go`; TS/TSX: `kebab-case.tsx`.
- SQL: tablas `plural_snake_case`.
- REST: recursos plurales (`/runs`, `/projects/{id}`).

## Guardrails de implementacion

- `packages/openapi/openapi.yaml` sera la fuente de verdad del contrato:
  editar, regenerar y revisar diffs.
- Las migraciones aplicadas son append-only; nunca reescribirlas.
- No editar store generado a mano.
- Los formatos de `config/*.yaml` son contratos de boot/runtime.
- v0.1 ejecuta solo adapters aprobados para `claude-code` y `codex`, dentro
  de contenedores efimeros y con aprobacion humana.
- Detectar una CLI no concede permiso de ejecucion.
- Red apagada por defecto; commit y push tienen aprobaciones independientes.
- Nada de shell arbitraria en el host ni deploy automatico en v0.1.

## Estrategia de documentacion y memoria

1. Cada fase cerrada actualiza al menos un doc en `docs/`.
2. Cada decision tecnica no trivial genera un ADR correlativo.
3. Antes de trabajo no trivial, consultar Engram para el proyecto `battos`.
4. La sesion termina guardando resumen de memoria; una fase cerrada registra
   ademas su cierre.

## Fases

| Fase | Objetivo | Estado |
|---|---|---|
| 0 | Bootstrap del repo | Completado |
| 1 | `battos status` funcional | Completado |
| 2 | DB + Memory Core | Completado |
| 3A | Contratos, ADRs finales y OpenAPI | Completado |
| 3B | Work model y Knowledge Center | En curso: Work Board API/CLI y TUI CLI v1; Knowledge pendiente |
| 4A | Runtime adapters Claude Code/Codex | Pending |
| 4B | Runs aislados, approvals y logs | Pending |
| 4C | Repositories, diff, commit y push | Pending |
| 5A | NovaCore opcional | Pending |
| 5B | Dashboard completo y usage | Pending |
| 6 | Hardening, seguridad y tag v0.1.0 | Pending |

## Decisiones criticas cerradas

- Stack Go + TS: `docs/adr/0001-go-stack.md`.
- SSE, no WebSockets: `docs/adr/0002-sse-no-websockets.md`.
- Memory Core propio, inspirado en Engram: `docs/adr/0004-memory-core-propio.md`.
- sqlc, no ORM: `docs/adr/0005-sqlc-vs-orm.md`.
- El antiguo scope read-only de ADR-0006 fue reemplazado.
- Knowledge Workspace/Obsidian opcional: `docs/adr/0010-knowledge-workspace-opcional.md`.
- v0.1 ejecuta runs supervisados: `docs/adr/0011-v01-ejecucion-supervisada.md`.
- Upgrades modulares con Extension Platform: `docs/adr/0012-extension-platform-modular.md`.
- Auth single-owner y secretos por referencia: `docs/adr/0013-auth-y-secretos-v01.md`.
- Runs y approvals auditables: `docs/adr/0014-run-lifecycle-y-approvals.md`.
