# ADR-0025 — Revisión de la estrategia de memoria: Engram como motor detrás de `MemoryProvider`

- **Estado**: Accepted — revisa y actualiza [ADR-0004](0004-memory-core-propio.md)
- **Fecha**: 2026-06-11
- **Decidido por**: Nico + Claude

## Context

ADR-0004 (2026-05-25) decidió construir un **Memory Core propio** en Go en vez de
depender de Engram, con dos razones centrales que **hoy ya no se sostienen**:

1. *"Engram no está en Go (necesitaría rewrite o bindings). Inviable."* →
   **Falso a junio 2026.** Engram (Gentleman-Programming/engram) es un **binario
   Go, MIT, v1.16.1**, con HTTP API (:7437) + 19 MCP tools + CLI + TUI + **Engram
   Cloud** (git-sync / autosync / dashboard cloud) → **resuelve la portabilidad
   multi-máquina** (pendrive ↔ server de casa ↔ nube) que de otro modo
   tendríamos que construir.
2. Mantenimiento propio vs. ride del roadmap upstream.

**Verificación crítica (schema real de las tools instaladas):** Engram scopea por
`project` + `session` + `scope (project|personal)`. **NO tiene `agent_id` ni
filtro por agente.** El Memory Core de BattOS sí (`project_id` + `agent_id` +
`scope`). La dimensión **agente** —central para el multi-agente de BattOS— es el
único gap real.

(Engram sí importable como librería = **bloqueado**: su lógica vive en `internal/`,
que Go prohíbe importar desde otro módulo. El camino de auto-update es
acoplamiento flojo: binario/HTTP/MCP.)

## Decision

**Introducir una interfaz `MemoryProvider`** (Save/Search/Recent/Context) con dos
implementaciones, **una activa por deployment** (sin split-brain):

1. **`BuiltinCore`** — el Memory Core actual. Pasa de "lo que se crece
   activamente" a **default seguro / modo offline / single-binary** (p.ej. el
   escenario pendrive sin Engram al lado).
2. **`EngramProvider`** — habla a un Engram corriendo (HTTP :7437 / MCP). Cuando
   es el motor activo, **rides sus updates + Engram Cloud da el sync
   multi-máquina gratis**.

**La dimensión agente, cuando Engram es el motor, se resuelve como convención fina
en BattOS** (mapear agente → `session_id` o prefijo de `topic_key`), **no** como
segundo store. Un cerebro (Engram) + un índice/mapeo delgado en código BattOS.

## Rationale

- BattOS **no es un motor de memoria** (ADR-0024): es ejecución + dashboard +
  comms. Delegar la memoria a Engram libera energía para el moat.
- "Dos memorias corriendo a la vez" = anti-patrón (split-brain). Por eso **una
  activa**, la otra como fallback detrás de la misma interfaz.
- El gap de agent-scope es **aproximable** (convención), no fatal — la memoria de
  equipo es compartida; la procedencia vive en `topic_key`/`session`/contenido.
- Reversibilidad: la interfaz permite volver a `BuiltinCore` sin reescribir
  consumidores.

## Consequences

- Los endpoints de memoria del dashboard pasan a **proxyar al provider activo**;
  el contrato del front no cambia (la API de BattOS hace de adaptador).
- **A verificar antes de migrar**: que el HTTP de Engram cubra lo que el dashboard
  consume (save/search/recent/stats con filtros de project/scope).
- La convención agente→Engram la **posee BattOS** (no Engram); documentarla.
- ADR-0004 queda **vigente como descripción del BuiltinCore**, pero su decisión
  de "propio en vez de Engram" queda **revisada por este ADR**.

## Cuándo se reevalúa

- Si el filtrado de memoria **por agente** se vuelve necesidad de primera clase
  que la convención no banca → meter una capa-índice de agente delgada en BattOS
  encima de Engram (sigue siendo un cerebro).
- Si Engram expone paquetes Go públicos (no `internal/`) → reconsiderar embeber
  como librería.
