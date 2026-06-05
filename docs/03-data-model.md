# 03 — Modelo de datos

> Schema base implementado al cierre de Fase 2: `apps/api/migrations/0001_init.sql`.
> El alcance final de `v0.1` agrega migraciones append-only para work model,
> knowledge, repositories, runs, approvals, chats y extensiones.

## Dos bases de datos

BattOS tiene **dos backends de persistencia** corriendo en simultáneo:

| Backend | Para qué | Driver | Archivo |
|---|---|---|---|
| **PostgreSQL 16** | Registries (projects, agents, skills, models, providers, MCPs, CLI tools, executions, usage, logs, NovaCore conversations) | `jackc/pgx/v5` + sqlc-generated | configurado vía `DATABASE_URL` |
| **SQLite + FTS5** | Memory Core (observaciones, búsqueda full-text) | `modernc.org/sqlite` (puro Go) | `data/memory/battos.db` |

Razones de la separación: ver `docs/adr/0004-memory-core-propio.md`.

## Tablas Postgres (14 + triggers)

### 1. `agent_runtimes` — catálogo de motores donde corren los agentes

| Campo | Tipo | Descripción |
|---|---|---|
| `id` | TEXT (PK) | `claude-code`, `codex`, `openclaw`, `hermes`, `mcp`, `n8n-webhook`, `direct-api`, `manual`, ... |
| `name` | TEXT | Nombre humano |
| `kind` | TEXT | `subprocess` \| `http` \| `mcp` \| `webhook` \| `manual` \| `direct-api` |
| `status` | TEXT | `available` \| `unavailable` \| `disabled` |
| `binary_path` | TEXT | Path detectado (si kind=subprocess) |
| `endpoint_url` | TEXT | Si kind=http/webhook |
| `capabilities` | JSONB | Lista de features (`["code_editing","mcp",...]`) |
| `config_schema` | JSONB | JSON schema validando `agents.runtime_config` |
| `risk_level` | TEXT | `low` \| `medium` \| `high` |

**Seed inicial**: 13 runtimes conocidos (claude-code, codex, opencode, gemini-cli, kimi-cli, qwen-cli, aider, openclaw, hermes, mcp, n8n-webhook, direct-api, manual). Todos arrancan `unavailable` hasta que CLI Detector confirme.

### 2. `providers` — APIs LLM

`openai`, `anthropic`, `google`, `openrouter`. Cada uno con `env_key`, `monthly_budget_usd`, `monthly_spend_usd`. Status: `configured` (env key presente) / `not_configured` / `down`.

### 3. `models` — registry de modelos por tier

| Campo clave | Descripción |
|---|---|
| `provider_id` | FK → providers |
| `tier` | 0..5 (§21.4 doc maestro) |
| `context_window` | tokens |
| `input_price_per_1m`, `output_price_per_1m` | USD/1M tokens |

### 4. `projects`

Contenedores de contexto. `slug` único, `owner_agent_id` (FK lógica), `monthly_budget_usd`, `metadata` JSONB libre.

### 5. `agents`

```sql
runtime_id        REFERENCES agent_runtimes(id)  -- qué motor lo ejecuta
runtime_config    JSONB                          -- params del runtime
allowed_tools     JSONB                          -- ["mcp_registry","memory",...]
allowed_projects  JSONB                          -- [] = todos los proyectos
is_lead           BOOLEAN                        -- NovaCore = true
is_meta           BOOLEAN                        -- opera el OS, no proyectos
```

`is_lead`/`is_meta` permiten que NovaCore (opcional desde `v0.1` segun ADR-0011) viva en la misma tabla que los demás agentes.

### 6. `skill_sources` + `skills`

```
skill_sources    fuentes desde donde se importan skills (oficial, community, personal)
skills           skills propias + importadas (con source_id, source_ref, version)
```

Esto soporta el **skill marketplace** (`battos skill install <url>`).

### 7. `cli_tools` — CLIs detectadas en el host

Detector escanea PATH cada N minutos y mantiene esta tabla. Relacionada con `agent_runtimes` vía `runtime_id` (ej: la CLI `claude` está vinculada al runtime `claude-code`).

### 8. `mcp_connections`

Servers MCP registrados: `transport` (stdio/http/sse), `command/args/env` o `url`, `permissions`, `health_score`. Mismo patrón marketplace que skills.

### 9. `executions` — log de cada invocación de agente

Una fila por invocación (`battos agent run X "..."`). Incluye `input_tokens`, `output_tokens`, `estimated_cost_usd`, `latency_ms`, `status`. Indexado por `(project_id, created_at)` y `(agent_id, created_at)` para sparklines rápidas.

### 10. `usage_events` — eventos de tokens/costo para agregar

Una fila por llamada a LLM (puede haber varias por execution). Indexado por `(provider_id, created_at)` y `(project_id, created_at)`. Alimenta el Usage tracker del panel.

### 11. `system_logs` — eventos de sistema no-HTTP

Detector encontró nueva CLI, MCP cambió de estado, Memory Core corrupted, etc. Niveles `debug/info/warn/error`, `source` arbitrario, `context` JSONB.

### 12. `novacore_conversations` + `novacore_messages`

Sesiones de chat con NovaCore (`is_lead=true`). Track de tokens/costo por conversación para budget enforcement.

## Tablas De Producto Para Completar v0.1

La migracion append-only `0002_work_knowledge.sql` agrega el Work Board y el
Knowledge Center sin editar `0001_init.sql`; las tablas de repositorios y
ejecucion continúan en fases posteriores:

| Area | Tablas / conceptos | Proposito |
|---|---|---|
| Work model | `domains`, `goals`, `tasks` | Agregado en `0002_work_knowledge.sql` |
| Knowledge | `knowledge_workspaces`, `journals`, `artifacts` | Agregado en `0002_work_knowledge.sql` |
| Repositories | `repositories`, `repository_credentials` por referencia segura | Repo local gestionado o GitHub autorizado |
| Execution | `runs`, `run_approvals`, `run_logs`, `artifacts.run_id` | Control plane agregado en `0003_runs.sql`; worker/contenedor quedan en 4B |
| Extensibility | `skills.prompt_template`, `skills.lifecycle` | Agregado en `0002`; adapters llegan en Fase 4A |

En Work Board, `domains` funcionan como areas mayores, clientes o lineas de
negocio. `projects` son espacios operables dentro de un domain opcional.
`goals` describen resultados esperados de un proyecto y `tasks` son acciones
concretas del board, opcionalmente asociadas a un goal. Las vistas API/CLI
pueden listar goals y tasks globalmente, pero la creacion v0.1 conserva
`project_id` obligatorio para no perder trazabilidad.

ADR-0014 resolvio la relacion: `runs` es la entidad de orquestacion visible
al usuario y `executions` conserva invocaciones tecnicas. La migracion
append-only `0003_runs.sql` agrega `runs`, `run_approvals`, `run_logs` y la FK
desde `artifacts.run_id`. En la base actual un run puede quedar
`awaiting_approval`, pasar a `queued` con approval `execute`, habilitar red con
approval `network`, cancelarse si no es terminal y asociar artifacts/logs; la
fundacion de worker ya agrega `started_at`, `completed_at`, `error_message` y
queries para reclamar/completar/fallar runs. La ejecucion real por adapter y
worker aislado se completa en el siguiente bloque de Fase 4B. La frontera
adapter/sandbox ya esta modelada en codigo: el adapter produce un plan y el
sandbox lo ejecuta o simula, evitando que un runtime ejecute directamente en el
host.

## Knowledge Center Artifacts

`artifacts` es el indice operacional en Postgres. El contenido gestionado vive
en filesystem bajo `knowledge.artifacts_dir` (`data/artifacts` por defecto).
La ruta guardada en `managed_path` siempre es relativa a esa raiz.

Layout canonico:

```text
data/artifacts/
  <project_id>/
    raw/       # briefs, referencias e inputs originales
    wiki/      # documentos curados para lectura humana
    outputs/   # entregables generados por agentes/runs
```

Cuando el API recibe `content` sin `external_url`, escribe el archivo dentro de
ese layout y rechaza rutas absolutas o con `..`. Esto deja el dashboard y la
futura exportacion Markdown/Obsidian usando la misma fuente canonica.

## Convenciones

- **IDs como TEXT** en tablas user-facing (slug-friendly, leíbles en URLs y CLI).
- **UUIDs** en tablas técnicas de logs/eventos (high-volume, no necesitan ser legibles).
- **`TIMESTAMPTZ`** siempre — nunca `TIMESTAMP` sin TZ.
- **JSONB** para campos flexibles (`metadata`, `capabilities`, `config`, `allowed_*`).
- **Triggers `set_updated_at`** automatizados en tablas user-facing.
- **Sin FKs duras** entre `projects ↔ agents` (slugs como referencias suaves, evita deps circulares).
- **FKs duras** en tablas de logs/eventos (`executions → projects/agents/skills/models/runtimes`) para integridad referencial.

## Generación de código tipado: sqlc

El archivo `apps/api/sqlc.yaml` configura **sqlc**: lee `queries/*.sql` + `migrations/*.sql` y genera Go tipado en `internal/store/`.

Ciclo de cambio:
```
edit migrations/000N.sql   ← agregar columna/tabla
edit queries/*.sql         ← agregar query nueva
sqlc generate              ← regenera internal/store/
go test ./apps/api/... ./apps/cli/... ./packages/core/...  ← verifica workspace
```

Razones: `docs/adr/0005-sqlc-vs-orm.md`.

## Tabla SQLite (Memory Core)

```sql
CREATE TABLE memory_items (
  id          INTEGER PK AUTOINCREMENT,
  type        TEXT     -- decision | architecture | bugfix | pattern | discovery | learning | manual
  title       TEXT NOT NULL,
  content     TEXT NOT NULL,
  topic_key   TEXT,    -- upsert key
  project_id  TEXT,
  agent_id    TEXT,
  scope       TEXT,    -- project | personal
  created_at  TIMESTAMP,
  updated_at  TIMESTAMP
);

-- FTS5 virtual table sincronizada via triggers AFTER INSERT/UPDATE/DELETE
CREATE VIRTUAL TABLE memory_items_fts USING fts5(
  title, content, topic_key,
  content='memory_items', content_rowid='id',
  tokenize='unicode61 remove_diacritics 2'
);
```

Detalle en `docs/05-memory-core.md`.

## ER simplificado

```
                    agent_runtimes ◄────┐
                          ▲              │
                          │              │
   skill_sources          │              │
        ▲                 │              │
        │                 │              │
     skills        agents ─────owns──── projects
                    │  ▲                  │
                    │  └── mcp_connections │
                    │                     │
                    ▼                     ▼
              executions ─────tracks──── usage_events
                    │                     │
                    ▼                     ▼
              system_logs           (per-provider aggs)


  novacore_conversations
        │
        ▼
  novacore_messages
```

Y separado:

```
SQLite Memory Core:
  memory_items + memory_items_fts (FTS5)
```

## Cuándo agregar una tabla nueva

1. Crear migration `migrations/000N_<feature>.sql` con `-- +goose Up` / `-- +goose Down`.
2. Si necesita queries → archivo en `queries/<area>.sql`.
3. `sqlc generate` → regenera `internal/store/`.
4. Si la tabla necesita health check → agregar al `SystemHandler.Status`.
5. Si expone endpoints → handler nuevo en `internal/handlers/`.
6. Actualizar este doc.
