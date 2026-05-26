# 05 — Memory Core

> Memoria persistente embebida en el binario `battos-api`. SQLite + FTS5 puro Go.
> Patrón inspirado en Engram, implementación propia adaptada a BattOS.

## Por qué propio y no Engram

- BattOS embebe la memoria en su propio binario (single-binary friendly).
- Schema adaptado: `project_id`, `agent_id`, `scope` (Engram no los tiene nativos).
- Sin proceso externo, sin MCP intermedio para acceso local.
- Engram queda como **referencia de diseño**, no como dependencia.

Decisión completa: `docs/adr/0004-memory-core-propio.md`.

Track de cambios upstream de Engram que valga la pena portar: `docs/upstream/engram-sync.md` (cuando se cree).

## Schema

```sql
CREATE TABLE memory_items (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  type        TEXT NOT NULL,        -- 'decision' | 'architecture' | 'bugfix' | 'pattern' | 'discovery' | 'learning' | 'manual'
  title       TEXT NOT NULL,
  content     TEXT NOT NULL,
  topic_key   TEXT,                 -- upsert key (NULL = no upsert)
  project_id  TEXT,
  agent_id    TEXT,
  scope       TEXT NOT NULL DEFAULT 'project',  -- 'project' | 'personal'
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE memory_items_fts USING fts5(
  title, content, topic_key,
  content='memory_items',
  content_rowid='id',
  tokenize='unicode61 remove_diacritics 2'
);
```

Tres triggers (`AFTER INSERT`, `AFTER DELETE`, `AFTER UPDATE`) mantienen el índice FTS5 sincronizado con la tabla principal. El usuario solo toca `memory_items` — FTS5 es transparente.

## API Go (paquete `internal/memory`)

```go
core, err := memory.Open("data/memory/battos.db")  // crea/abre + aplica schema
defer core.Close()

// Save: inserta o upsert (si topic_key ya existe)
saved, err := core.Save(ctx, memory.Observation{
    Type:      memory.TypeDecision,
    Title:     "Stack Go + TypeScript",
    Content:   "Razón: orquestación + binario único + footprint chico",
    TopicKey:  "battos/stack-decision",
    ProjectID: "battos",
    Scope:     memory.ScopeProject,
})

// Recent: últimas N
items, _ := core.Recent(ctx, 20)

// Search: FTS5 + filtros
results, _ := core.Search(ctx, "goroutines", memory.SearchFilter{
    Type:      memory.TypePattern,
    ProjectID: "battos",
    Limit:     10,
})

// Stats: agregados
stats, _ := core.Stats(ctx)
```

## API HTTP

| Endpoint | Método | Body / Query | Devuelve |
|---|---|---|---|
| `/memory/recent` | GET | `?limit=20` | `{items, count}` |
| `/memory/search` | POST | `{query, filter, limit}` | `{results, count, query}` |
| `/memory/save` | POST | `Observation` completo | `Observation` con id |
| `/memory/stats` | GET | - | `{total_items, items_last_24h, ...}` |
| `/memory/{id}` | GET | - | `Observation` o 404 |

Ejemplo de search:
```json
POST /memory/search
{
  "query": "FTS5",
  "filter": { "type": "bugfix", "project_id": "battos" },
  "limit": 10
}
```

Devuelve cada resultado con `rank` BM25 (menor = mejor match):
```json
{
  "results": [
    { "id": 2, "title": "Bugfix: SQLite scan a time.Time", "rank": -0.5, ... }
  ],
  "count": 1, "query": "FTS5"
}
```

## CLI

```bash
battos memory recent
battos memory recent --limit 50

battos memory search "FTS5"
battos memory search "FTS5" --type bugfix
battos memory search "campaña" --project red-nbl --scope project

battos memory save \
  --title "Decisión: Memory Core propio" \
  --content "Embebido en Go con SQLite+FTS5" \
  --type decision \
  --topic-key "battos/memory-core-decision" \
  --project battos

battos memory stats
```

## Concurrencia y rendimiento

- **WAL mode** activado en el DSN (`_pragma=journal_mode(WAL)`): permite N lectores + 1 escritor concurrentes.
- **Connection pool** limitado a 8 conexiones (SQLite tiene un único writer thread internamente).
- **`busy_timeout=5000`**: si hay contención, espera hasta 5s en vez de fallar inmediatamente.

Para BattOS single-user esto es más que suficiente. Multi-tenant futuro: considerar separar Memory Core por tenant (un archivo SQLite por usuario).

## Sanitización de queries FTS5

FTS5 interpreta operadores especiales: `AND`, `OR`, `NOT`, `-`, `"`, `*`, `:`, `(`, `)`. Una query del usuario como `fix auth bug` no debe romper.

Sanitización aplicada en `core.go:sanitizeFTS`:
1. Split por espacios → tokens.
2. Escapar comillas dobles internas (`"` → `""`).
3. Wrappear cada token en quotes: `fix auth bug` → `"fix" "auth" "bug"`.

Esto hace búsqueda de texto literal (AND implícito), no de operadores. Si en el futuro se quiere exponer sintaxis FTS5 cruda, agregar un flag `--raw` al CLI.

## Backup y restore

Como SQLite es un único archivo:
```bash
# Backup en caliente (WAL-safe)
sqlite3 data/memory/battos.db ".backup /tmp/battos-memory-backup.db"

# Restore
cp /tmp/battos-memory-backup.db data/memory/battos.db
```

Comando `battos memory backup` planeado para Fase 6.

## Health check

`/status` reporta el subsistema `memory` con `LatencyMs` del último ping:

```json
{ "name": "memory", "status": "ok", "detail": "SQLite + FTS5 listo", "latency_ms": 0 }
```

## Diferencias contra Engram

| Aspecto | Engram | BattOS Memory Core |
|---|---|---|
| Lenguaje | Rust (creo) | Go |
| Distribución | Binario separado + MCP server | Embebido en `battos-api` |
| Schema | Observaciones genéricas | `+ project_id, agent_id, scope` |
| Búsqueda | FTS5 | FTS5 |
| Upsert | Por `topic_key` | Igual |
| Judgment de duplicados | Sí | No (v0.3+ posible port) |
| API | MCP + CLI | HTTP + CLI (HTTP también puede exponerse como MCP en v0.3+) |

## Roadmap

| Versión | Capacidad |
|---|---|
| **v0.1** ✅ | Schema + FTS5 + Save/Search/Recent/Stats + CLI + HTTP |
| **v0.2** | Soft delete + Update por ID + bulk import |
| **v0.3** | Judgment de duplicados (port de Engram) + MCP server interno |
| **v0.4** | Backup/restore comandos + export JSONL |
| **v0.5** | Cluster mode (Memory Core por tenant) |
