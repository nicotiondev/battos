# 10 - Roadmap

> Estado revisado el 26 de mayo de 2026. El producto final y sus capacidades
> se describen en `docs/14-producto-final-y-roadmap.md`.

## Estado Actual Verificado

| Fase | Entrega | Estado |
|---|---|---|
| 0 | Bootstrap, stack y docs iniciales | Completada |
| 1 | API `/health`, `/version`, `/status` y CLI `status` | Completada |
| 2 | PostgreSQL base + Memory Core SQLite/FTS5 + CLI/HTTP memory | Completada |

Validacion disponible:

```powershell
go test ./apps/api/... ./apps/cli/... ./packages/core/...
```

## Cambio De Alcance

El plan inicial trataba `v0.1` como dashboard/configuracion sin ejecutar
agentes. Esa decision fue reemplazada por ADR-0011: `v0.1` completara un ciclo
de trabajo real, pero supervisado, aislado y auditable.

BattOS mantiene sus principios:

- Plano de control propio, no reemplazo de los runtimes.
- Lienzo en blanco: sin proyectos/agentes/skills personales precargados.
- Memory Core propio; Engram y Gentle-AI son referencias para evolucionarlo.
- Obsidian opcional como exportacion Markdown, no base operacional.
- Sin shell arbitrario, deploy automatico ni autonomia continua.

## v0.1.0 - Mission Control Ejecutor Supervisado

| Fase | Objetivo | Resultado esperado |
|---|---|---|
| 3A | Contratos y decisiones | OpenAPI; ADRs sincronizados; contrato de CRUD, runs, chats y approvals |
| 3B | Modelo de producto | Domains, goals, tasks, artifacts, journals, workspaces y skills/prompt templates |
| 4A | Runtimes ejecutables | Detector/providers y adapters aprobados `claude-code` y `codex` |
| 4B | Execution engine | Runs en contenedor efimero, red configurable, logs SSE, cancelacion y auditoria |
| 4C | Repositories | Repo local gestionado o GitHub, ramas, diff, commit/push aprobados |
| 5A | NovaCore | Chat opcional para operar BattOS y proponer runs con HITL |
| 5B | Dashboard | Command Center, Work Board, Control Room y Knowledge Center |
| 6 | Hardening/release | Seguridad, backups, tests, instalacion VPS y tag `v0.1.0` |

### Que Podras Hacer En v0.1

- Crear un proyecto, objetivo y task con artifacts de referencia.
- Crear un agente conectado a Claude Code o Codex.
- Pedirle construir o modificar codigo y ejecutar build/tests en contenedor.
- Revisar logs, diff, artifacts, tokens/costo reportado y memoria del run.
- Aprobar commit y push sin habilitar deployment automatico.
- Usar NovaCore para administrar el OS y preparar runs.

## Versiones Posteriores

| Version | Alcance |
|---|---|
| `v0.2` | Export Markdown/Obsidian; Extension Platform con manifests, snapshots/rollback; Memory export/import/consolidate/MCP; SDD opcional; PR aprobado |
| `v0.3` | Deployment connectors aprobables; adapters adicionales; Ollama/model routing; medicion valor/costo; artifact promotion |
| `v0.4+` | Hermes/OpenClaw always-on; Goal Mode limitado; ROI; skill evaluation; Loop/Dreaming recomendado; posible sync knowledge controlado |

## Fronteras De Seguridad

- Adapters ejecutables aprobados, no cualquier CLI detectada.
- Contenedor por run; terminal amplia sólo dentro del contenedor.
- Red `OFF` por defecto y activacion auditada.
- Secrets controlados y nunca expuestos en logs.
- Approvals separados para run, commit, push y futuros deployments.
- OMI/captura ambiental excluida del roadmap base.
