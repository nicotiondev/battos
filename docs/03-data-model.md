# 03 - Modelo De Datos

> Estado v0.1: BattOS usa una base SQLite unificada en `data/battos.db`.
> Postgres ya no es requisito del flujo normal; queda como referencia historica
> y posible importador futuro.

## Base Unica

| Backend | Para que | Driver | Ubicacion |
|---|---|---|---|
| SQLite + FTS5 | Work Board, registries, Knowledge, repositories, runs, approvals, usage, NovaCore, audit y Memory Core | `database/sql` + `modernc.org/sqlite` + sqlc | `data/battos.db` por defecto, configurable con `database.path` o `BATTOS_DATABASE_PATH` |
| Filesystem gestionado | Repositorios, journals, artifacts y workspaces temporales de runs | Go stdlib | `data/` |

La decision esta registrada en `docs/adr/0021-sqlite-unificado.md`. El schema
operacional vive en `apps/api/internal/store/sqlite_schema.sql` y las queries
tipadas en `apps/api/queries/*.sql`.

## Areas Principales

### Registries

- `agent_runtimes`: catalogo de runtimes conocidos (`claude-code`, `codex`,
  `manual`, `mcp`, etc.).
- `providers` y `models`: proveedores/modelos sin secretos inline.
- `agents`, `skill_sources`, `skills`, `cli_tools`, `mcp_connections`: identidad
  de agentes, skills versionadas, herramientas detectadas y conexiones MCP.

Detectar un binario o provider no concede ejecucion. La autorizacion real vive
en runs y approvals.

### Work Board

- `domains`: areas, clientes o lineas de negocio.
- `projects`: espacios operables dentro de un domain opcional.
- `goals`: resultados esperados de un proyecto.
- `tasks`: acciones concretas del board, con `project_id` opcional para inbox.

### Knowledge Center

- `knowledge_workspaces`: workspace canonico por proyecto.
- `journals`: notas Markdown fechadas.
- `artifacts`: indice operacional de archivos, links, diffs y outputs.

El contenido gestionado vive bajo `knowledge.artifacts_dir`
(`data/artifacts` por defecto). La ruta guardada en `managed_path` siempre es
relativa a esa raiz.

Layout canonico:

```text
data/artifacts/
  <project_id>/
    raw/
    wiki/
    outputs/
```

### Runs Supervisados

- `runs`: entidad visible de orquestacion.
- `run_approvals`: approvals HITL (`execute`, `network`, `commit`, `push`,
  `remember` cuando aplica).
- `run_logs`: logs persistidos para SSE, CLI y auditoria.
- `artifacts.run_id`: vinculo entre outputs y runs.

Un run parte como `awaiting_approval`; `execute/approved` lo mueve a `queued`.
El worker reclama runs desde SQLite, ejecuta en `dry_run` o DockerSandbox y
persistira logs, resultado, artifacts y timestamps terminales.

### Usage, NovaCore Y Audit

- `usage_events`: tokens, costo y metadata de consumo cuando un adapter/provider
  lo reporta.
- `novacore_conversations` y `novacore_messages`: chat opcional de NovaCore.
- `audit_events`: trazabilidad de acciones relevantes.

### Memory Core

`memory_items` y `memory_items_fts` viven en la misma base SQLite. FTS5 usa
triggers para mantener el indice sincronizado.

```sql
CREATE TABLE memory_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  topic_key TEXT,
  project_id TEXT,
  agent_id TEXT,
  scope TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## Convenciones SQLite

- IDs publicos y UUIDs existentes se guardan como `TEXT`.
- Timestamps se guardan como `TEXT` con `CURRENT_TIMESTAMP`.
- JSON flexible se guarda como `TEXT`; la validacion se hace en Go cuando
  corresponde.
- FKs se habilitan con `PRAGMA foreign_keys = ON`.
- WAL y `busy_timeout` se configuran al abrir la base.
- `sql.ErrNoRows` es el sentinel para filas ausentes.
- Las migraciones historicas Postgres no son el camino de boot de v0.1; el
  schema SQLite embebido crea una instalacion limpia.

## Generacion De Codigo Tipado

`apps/api/sqlc.yaml` configura sqlc con:

- `engine: sqlite`
- schema `apps/api/internal/store/sqlite_schema.sql`
- queries `apps/api/queries/*.sql`
- paquete Go generado en `apps/api/internal/store`

Ciclo de cambio:

```powershell
cd apps/api
sqlc generate
go test ./...
```

Para validar el workspace completo:

```powershell
go test ./apps/api/... ./apps/cli/... ./packages/core/...
```

## Cuando Agregar Una Tabla Nueva

1. Editar `apps/api/internal/store/sqlite_schema.sql`.
2. Agregar o ajustar queries en `apps/api/queries/<area>.sql`.
3. Regenerar sqlc.
4. Actualizar handlers/tests y, si expone HTTP, `packages/openapi/openapi.yaml`.
5. Actualizar docs vivos y ADR si cambia una decision arquitectonica.
