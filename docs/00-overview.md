# 00 - Overview

## Que es BattOS

BattOS es un **Mission Control agentic self-hosted** para organizar trabajo y
ejecutar agentes de forma supervisada desde un panel y una CLI.

Administra:

- Dominios, proyectos, objetivos y tareas.
- Agentes, skills versionadas, modelos, proveedores y conexiones MCP.
- Repositorios Git autorizados y artefactos producidos.
- Memory Core persistente, journals y Knowledge Center.
- Runs de agentes aprobados, logs, uso, costos y auditoria.

BattOS no reemplaza Linux, Docker, GitHub, Claude Code, Codex, Obsidian ni
n8n. Los integra bajo una capa de control con permisos y trazabilidad.

## Que podras hacer en v0.1

Ejemplo: desde el dashboard eliges un proyecto de un cliente, creas la tarea
"landing page", adjuntas referencias, eliges un agente que use Claude Code o
Codex y apruebas la ejecucion. BattOS abre un contenedor efimero para ese run,
muestra logs y consumo, conserva outputs y diff, y te pide aprobacion separada
antes de hacer commit o push.

Tambien podras:

- Usar NovaCore para convertir una idea en proyecto, objetivo, tareas o una
  propuesta de run; nunca ejecuta sin confirmacion.
- Navegar un Work Board con objetivos y tareas.
- Consultar memoria, journals, documentos y outputs desde Knowledge Center.
- Trabajar por web o por `battos` CLI.

## Tesis central

> Usar el agente correcto, con el contexto, skill, modelo, memoria y permiso
> correctos, para cada tarea.

```text
Linux administra la maquina.
Docker aisla la ejecucion.
Git conserva el trabajo.
BattOS administra objetivos, agentes, memoria, ejecucion y aprobaciones.
```

## Flujo operativo de v0.1

```text
Idea o tarea del usuario
  -> proyecto / objetivo / tarea
  -> agente + skill + runtime aprobado (Claude Code o Codex)
  -> confirmacion humana
  -> run en contenedor efimero, red apagada por defecto
  -> logs SSE + memoria + artefactos + uso
  -> revision de diff
  -> aprobacion opcional de commit y push
```

## Persistencia y conocimiento

- SQLite unificado (`data/battos.db`) es la fuente de verdad operacional para
  recursos, runs, aprobaciones, uso, auditoria y Memory Core.
- Memory Core usa FTS5 dentro de la misma base, inspirado en Engram pero
  integrado a BattOS.
- El filesystem administrado guarda repositorios, artefactos y journals.
- Postgres queda fuera del camino normal de v0.1; solo se conserva como
  referencia historica/futura para escenarios multiusuario o importacion.
- Desde v0.2, un export Markdown opcional permite abrir Knowledge Workspace en
  Obsidian sin convertirlo en dependencia ni base principal.

## Runtimes y extensibilidad

En v0.1 solamente se ejecutan adapters aprobados para `claude-code` y
`codex`. Detectar una herramienta instalada no la autoriza a ejecutar.
Versiones posteriores agregan Extension Platform con manifests, instalacion,
actualizacion, desactivacion y rollback; despues podran entrar mas adapters,
MCP avanzado, n8n, Ollama o despliegues aprobados.

La especificacion vigente esta en `docs/14-producto-final-y-roadmap.md` y el
trabajo de implementacion en `docs/10-roadmap.md`.
