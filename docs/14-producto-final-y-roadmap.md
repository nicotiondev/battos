# 14 - Producto Final Y Roadmap Consolidado

> Especificacion de producto acordada el 26 de mayo de 2026. Define el norte
> de BattOS y prevalece sobre el plan historico inicial. El estado realmente
> implementado se verifica en `docs/10-roadmap.md`.

## Que Es BattOS

BattOS es un **Mission Control agentic self-hosted**. Centraliza proyectos,
objetivos, tareas, agentes, runtimes, memoria, documentos, ejecuciones, uso y
extensiones desde dashboard y CLI.

BattOS no reemplaza Claude Code, Codex, GitHub, Obsidian, MCP, Hermes u
OpenClaw. Los conecta bajo un plano de control propio que conserva memoria,
controla permisos, aísla ejecuciones y deja auditoria.

## Que Podras Hacer

### Organizar trabajo y conocimiento

- Crear domains, proyectos, goals y tasks con vista Kanban.
- Crear agentes y skills versionadas con prompt templates aprobables.
- Mantener journals, briefs y outputs en un Knowledge Center con buckets
  `Raw`, `Wiki` y `Outputs`.
- Adjuntar artifacts de tipo Markdown, imagen o enlace y ver previews.
- Consultar y guardar memoria persistente desde panel o CLI.

### Ejecutar agentes para producir codigo

Ejemplo: crear una landing para un cliente.

1. Creas el proyecto `clinica-norte-web`, su goal y una task.
2. Conectas un repositorio GitHub autorizado o creas un repo Git local
   administrado por BattOS.
3. Adjuntas brief, imagen de referencia y enlaces.
4. Seleccionas un agente `Builder Web` con adapter `claude-code` o `codex`.
5. Escribes: "Construye la landing segun estos artifacts y valida el build".
6. BattOS muestra runtime, repo/rama, permisos, red `ON/OFF` y requiere tu
   confirmacion.
7. Un contenedor efimero ejecuta el agente en un workspace aislado.
8. Revisas logs, diff, artifacts, tests/build y consumo reportado.
9. Apruebas por separado commit y push.

En `v0.1` BattOS podra generar y versionar el codigo de esa web. No desplegara
produccion automaticamente; deploy entra luego como connector aprobado.

### Conversar de dos formas

| Chat | Uso | Limite |
|---|---|---|
| NovaCore | Crear/editar recursos de BattOS, diagnosticar y proponer runs | No ejecuta ni cambia datos sensibles sin confirmacion |
| Chat de agente | Iterar sobre trabajo de proyecto usando Claude Code/Codex | Sólo opera en el workspace aislado del run |

### Medir consumo y actividad

El dashboard mostrara runs, duracion, tokens y costo por proyecto, agente,
runtime y modelo cuando el adapter/proveedor informe esos datos. Cada cifra
se marcara como `exacta`, `estimada` o `no reportada`, para no fingir
precision.

## Dashboard Final Acordado

| Area | Que muestra | Que permite hacer |
|---|---|---|
| Command Center | Health, CPU/MEM/NET, runtimes, providers, MCPs, runs, tokens/costos, alertas | Detectar motores, revisar ejecuciones y budgets |
| Work Board | Domains, projects, goals y Kanban de tasks | Crear y ordenar trabajo, asignar agente/skill, iniciar run |
| Control Room | Agente/runtime/proyecto, chat, logs SSE, permisos, artifacts y diff | Confirmar/cancelar run, aprobar commit/push |
| Knowledge Center | Memory Core, journals, Raw/Wiki/Outputs y previews | Guardar contexto, revisar entregables y exportar en versiones posteriores |
| Extensions | Adapters, connectors, skill packs y actualizaciones | Instalar/actualizar/deshabilitar desde `v0.2` |

## Persistencia

| Almacen | Responsabilidad | Backup |
|---|---|---|
| PostgreSQL | Projects, domains, goals, tasks, agentes, skills, runtimes, runs, approvals, usage, chats, extensions y auditoria | Dump/restore operativo |
| SQLite + FTS5 Memory Core | Decisiones, patrones, aprendizajes, resumenes de runs y busqueda de contexto | Backup SQLite; export/import posterior |
| Filesystem gestionado | Artifacts, journals, repos locales y workspaces temporales/retenciones | Snapshot del directorio gestionado |
| Git/GitHub | Codigo versionado y ramas aprobadas | Historial Git/remoto |
| Vault Markdown opcional | Exportacion legible para Obsidian | Complementario, nunca fuente operacional primaria |

Memory Core ya esta implementado y esta inspirado en Engram. El roadmap toma
de Gentle-AI la idea de guardar memoria de forma automatica por sesion/run,
exportar/importar contexto, mantener skill registries y proteger upgrades con
snapshots y rollback.

## Seguridad

- BattOS arranca vacio: no crea agentes, skills ni proyectos personales.
- Sólo se ejecutan **runtime adapters aprobados**; detectar `docker`, `python`
  o cualquier binario no concede permiso de ejecucion.
- `v0.1` implementa inicialmente adapters de Claude Code y Codex.
- Cada run exige confirmacion y corre en un contenedor efimero aislado.
- La terminal amplia existe dentro del contenedor, nunca sobre el host.
- Red externa desactivada por defecto; el usuario puede activarla por run y
  la decision queda auditada.
- Secrets se referencian/injectan de forma controlada; nunca se guardan en
  logs ni artifacts.
- Commit y push requieren aprobaciones separadas.
- Deploy, instalacion de extensiones sensibles y acciones autonomas requieren
  controles adicionales en versiones posteriores.
- Se registran prompt, runtime, permisos, red, logs, diff, costo reportado,
  approvals y resultado del run.

## Modularidad Y Upgrades

BattOS queda modular por diseño:

| Modulo | Ejemplos |
|---|---|
| Runtime adapters | Claude Code, Codex, OpenCode, Hermes, OpenClaw |
| Connectors | GitHub, MCP, Obsidian Exporter, Vercel/Cloudflare |
| Skill packs | Web development, SEO, investigacion, documentacion |
| Policies | Budgets, approvals, network profiles, permisos |
| UI modules | Galerias, paneles especializados y reportes |

`v0.1` define tipos, versiones y permisos de adapters/skills. `v0.2`
implementa la Extension Platform con manifests, compatibilidad, install,
update, disable, snapshot previo y rollback. Las actualizaciones sensibles no
se aplican automaticamente; se detectan y el usuario las aprueba.

## Roadmap Por Version

### Estado Ya Implementado - Foundation (Fases 0-2)

- API de health/version/status y CLI `battos status`.
- PostgreSQL schema base y store sqlc.
- Memory Core SQLite+FTS5 con HTTP/CLI: `save`, `search`, `recent`, `stats`.

### v0.1.0 - Mission Control Ejecutor Supervisado

- OpenAPI y CRUD de registries.
- Domains, goals, tasks/Kanban, artifacts, journals y knowledge workspaces
  canonicos en BattOS.
- Skills con version, lifecycle y prompt template; skill registry basico.
- Detector de runtimes/providers y adapters aprobados para Claude Code/Codex.
- Repositorios Git locales gestionados o GitHub conectados.
- Runs en contenedor efimero, red toggleada, logs SSE, cancelacion,
  approvals, diff, commit y push aprobados.
- NovaCore opcional con chat para operar BattOS y proponer runs.
- Dashboard completo: Command Center, Work Board, Control Room y Knowledge
  Center.
- Usage dashboard con tokens/costos exactos, estimados o no reportados.
- Snapshots basicos de configuracion critica antes de cambios manuales.

### v0.2 - Portabilidad Y Extension Platform

- Exportacion unidireccional a vault Markdown compatible con Obsidian.
- Memory Core export/import por proyecto, consolidacion de nombres y
  exposicion MCP inicial.
- Instalar, actualizar, deshabilitar y restaurar adapters/connectors/skill
  packs mediante manifest versionado y rollback.
- Workflow SDD opcional para tareas grandes, inspirado en Gentle-AI.
- Pull request GitHub aprobado como paso posterior al push.

### v0.3 - Delivery Y Ecosistema De Motores

- Connectors de deployment aprobable para targets concretos.
- Mas adapters maduros: OpenCode/Gemini/Aider o equivalentes priorizados por
  uso real.
- Ollama/modelos locales y routing basado en reglas.
- Telemetria de costo, tiempo ahorrado y valor entregado.
- Artifact promotion hacia conocimiento y reviews de skills.

### v0.4+ - Automatizacion Supervisada

- Hermes/OpenClaw y runtimes always-on.
- Goal Mode restringido: schedules, budgets, cancelacion, sandbox y HITL.
- ROI real y evaluacion comparativa de skills/modelos.
- Loop/Dreaming como recomendaciones auditables; nunca auto-mutacion ciega.
- Sync bidireccional de knowledge sólo si se resuelven identidad, versionado y
  conflictos.

## Explicitamente Fuera Del Nucleo Inicial

- Captura continua de pantalla/audio tipo OMI.
- Shell arbitrario contra el host.
- Deploy automatico sin confirmacion.
- Ejecucion autonomamente indefinida.
- Obsidian como base primaria.
- Soporte inmediato para todos los CLIs y plugins posibles.

## Fuentes De Inspiracion Incorporadas

- Informes Agent OS: work model, artifacts, dashboard, local-first,
  knowledge plane y aprendizaje controlado.
- Engram: observaciones persistentes, FTS, sesiones, export/import y futura
  interfaz MCP, implementados como Memory Core propio.
- Gentle-AI: adapters/componentes, skill registry, SDD opcional,
  snapshots/rollback y upgrades gobernados.
