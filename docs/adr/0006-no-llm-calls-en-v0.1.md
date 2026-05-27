# ADR-0006: v0.1 no ejecuta agentes ni llama modelos LLM

**Status**: Superseded by ADR-0011
**Date**: 2026-05-25

## Context

BattOS v0.1 necesita validar inventario, configuración, health, memoria y experiencia de control antes de introducir acciones con costo o acceso a herramientas externas.

## Decision

La versión v0.1 es de observación y configuración: puede registrar providers, modelos, runtimes y conexiones, pero no ejecuta agentes, CLIs externas ni llamadas a APIs LLM.

## Supersession

El 26 de mayo de 2026 el alcance de producto cambio: `v0.1` debe entregar
ejecucion supervisada de agentes mediante adapters aprobados, contenedores
efimeros, approvals y auditoria. Esta decision ya no rige el roadmap vigente;
ver ADR-0011.

## Historical Consequences

- `executions` y `usage_events` pueden existir en el schema como preparación, sin flujos productores reales en v0.1.
- El dashboard y el CLI pueden mostrar estado y configuración sin requerir secretos activos.
- Workers, routing de modelos y aprobación humana quedan para versiones posteriores.

## Related

- `docs/10-roadmap.md`
- `docs/adr/0008-lienzo-en-blanco.md`
- `docs/adr/0009-novacore-meta-agent.md`
- `docs/adr/0011-v01-ejecucion-supervisada.md`
