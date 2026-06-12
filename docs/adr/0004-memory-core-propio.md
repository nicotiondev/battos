# ADR-0004: Memory Core propio (SQLite + FTS5 embebido) vs dependencia de Engram

- **Status**: Accepted — **revisado por [ADR-0025](0025-memoria-engram-memoryprovider.md)** (2026-06-11): la premisa "Engram no está en Go" ya es falsa; se adopta `MemoryProvider` con Engram como motor y este Core como fallback offline.
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

BattOS necesita memoria persistente para:
- Decisiones técnicas y de producto.
- Bugfixes documentados.
- Patrones reutilizables.
- Aprendizajes operativos.
- Contexto por proyecto que sobrevive sesiones.

Existe **Engram** (Gentleman-Programming/engram) como referencia: agente-agnóstico, SQLite + FTS5, expone MCP + CLI + HTTP. Cumple gran parte de lo que necesitamos.

Las opciones:

1. **Depender de Engram como proceso externo** y comunicarse vía MCP.
2. **Importar Engram como librería** (si fuera Go).
3. **Implementar Memory Core propio** inspirado en Engram, embebido en `battos-api`.

## Decision

**Implementar Memory Core propio en Go (SQLite + FTS5 con `modernc.org/sqlite`), embebido dentro del binario `battos-api`.**

Engram se mantiene como **referencia conceptual** — track de cambios upstream en `docs/upstream/engram-sync.md` cuando aparezcan cambios que valga la pena portar.

## Consequences

### Positivas

- **Single binary.** `battos-api.exe` incluye la memoria. Sin proceso aparte, sin orquestar startup order, sin configuración cruzada.
- **Schema adaptado a BattOS.** Agregamos `project_id`, `agent_id`, `scope` que Engram no tiene nativos. Útil para filtrar memoria por proyecto/agente desde el dashboard.
- **API HTTP de primera clase.** Engram expone MCP principalmente; BattOS expone HTTP REST que el frontend Next.js y el CLI consumen directamente.
- **Sin CGo.** `modernc.org/sqlite` es puro Go — cross-compile a Linux/Windows/Darwin con un solo comando, sin librerías nativas.
- **Control total.** Cuando queramos agregar features (judgment de duplicados, export JSONL, etc.) lo hacemos directo.
- **Pedagógico.** El usuario aprende cómo funciona FTS5 leyendo nuestro código en `internal/memory/`.

### Negativas

- **Trabajo de implementación.** Tuvimos que escribir ~400 líneas Go en lugar de simplemente declarar una dependencia.
- **Mantenimiento propio.** Si Engram fixea un bug o agrega un feature útil, hay que portarlo manualmente.
- **Mitigación**: `docs/upstream/engram-sync.md` mantiene un changelog de qué se portó / qué no.

### Neutrales

- En v0.3+ podemos exponer el Memory Core de BattOS **como MCP server propio**, así otros agentes pueden consumirlo igual que consumen Engram. Mejor de los dos mundos.

## Alternatives considered

### Engram como proceso externo + MCP
- ✅ Cero código a escribir.
- ✅ Aprovecha el roadmap upstream automáticamente.
- ❌ Otro binario para distribuir y mantener vivo en el VPS.
- ❌ Configuración cruzada (URLs, MCP transport, autenticación).
- ❌ Latencia extra de IPC para cada read/write.
- ❌ El schema no encaja con BattOS — necesitaríamos un mapper.
- ❌ Si el proceso Engram se cae, BattOS pierde memoria.

### Engram como librería Go importada
- ❌ Engram no está en Go (necesitaría rewrite o bindings).
- Inviable.

### Sin memoria persistente en v0.1
- ❌ Toda la propuesta de "memoria viva del OS" se cae.
- ❌ NovaCore opcional de v0.1 y los runs supervisados necesitan contexto; sin Memory Core arrancan en cero cada vez.

### Postgres-only (sin SQLite)
- ✅ Una sola DB.
- ❌ Postgres no tiene FTS comparable a FTS5 (`tsvector` es bueno pero menos ergonómico para fuzzy/ranking).
- ❌ Memoria local-first: si Postgres se cae, no debería caer la memoria.

## Implementation notes

- Paquete: `apps/api/internal/memory`.
- Driver: `modernc.org/sqlite` (puro Go, sin CGo).
- DSN: `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`.
- Schema aplicado idempotentemente en `Open()` — no hay migraciones separadas para SQLite (todavía).
- FTS5 sincronizada via triggers `AFTER INSERT/UPDATE/DELETE`.
- Tokenizer: `unicode61 remove_diacritics 2` (sirve para español sin tildes).
- Sanitización de queries FTS5: cada término del usuario se wrappea en comillas (`"foo" "bar"`).

## Cuándo se reevalúa

- Si BattOS termina necesitando funcionalidad muy parecida a Engram (que agreguen judgment, multi-tenant, distributed memory) y duplicar trabajo deja de tener sentido → reconsiderar adoptar Engram como sidecar.
- Si Engram saca un cliente Go oficial → reconsiderar usarlo como librería embebida.

## Related

- `docs/05-memory-core.md` — uso técnico.
- `docs/12-novacore.md` — NovaCore consume Memory Core como una de sus tools.
- `docs/upstream/engram-sync.md` (futuro) — track de cambios upstream.
- ADR-0001 — decisión de Go como stack principal (habilitó esta).
- ADR-0010 — el Knowledge Workspace Markdown es complementario, no reemplaza Memory Core.
