# 01 - Arquitectura

## Objetivo de v0.1

BattOS v0.1 sera un control plane self-hosted con dashboard y CLI capaz de
modelar trabajo, conservar conocimiento y ejecutar de manera supervisada dos
runtimes iniciales: Claude Code y Codex.

Al cierre de Fase 2 ya existen API system/memory, CLI status/memory, config,
PostgreSQL base y Memory Core SQLite + FTS5. El modelo de producto, motor de
runs, repositorios, NovaCore y frontend aun deben implementarse.

## Capas

```text
+---------------------------------------------------------------------+
| Dashboard Next.js                                                   |
| Command Center | Work Board | Control Room | Knowledge Center       |
+-------------------------------+-------------------------------------+
                                | HTTP/JSON + SSE
+-------------------------------v-------------------------------------+
| API Go + chi                                                       |
| contracts | resources | memory | knowledge | runs | repos | usage   |
| adapters (claude-code/codex) | NovaCore opcional | audit | config  |
+---------+--------------------+----------------------+---------------+
          |                    |                      |
+---------v----------+ +-------v-----------+ +--------v--------------+
| PostgreSQL         | | SQLite + FTS5     | | Filesystem gestionado |
| recursos, runs,    | | Memory Core       | | repos, journals,      |
| approvals, usage   | |                  | | artifacts, snapshots  |
+--------------------+ +-------------------+ +-----------------------+
                                |
+-------------------------------v-------------------------------------+
| Worker Go: crea contenedor efimero, ejecuta adapter, captura logs   |
| y diff; red OFF por defecto, activacion visible y auditada          |
+-------------------------------+-------------------------------------+
                                |
                   +------------v-------------+
                   | Claude Code / Codex CLI  |
                   +--------------------------+

CLI `battos` -> misma API; no accede directamente a las bases.
```

## Componentes

- **API**: unica autoridad de lectura/escritura, contratos REST/SSE,
  autenticacion, autorizacion, auditoria y orquestacion de recursos.
- **Worker**: reclama runs persistidos en PostgreSQL y los ejecuta en
  contenedores efimeros. Para v0.1 no requiere Redis: PostgreSQL mantiene
  estado, lock y recuperacion del run.
- **Dashboard**: el producto principal para operar trabajo, ejecuciones,
  conocimiento y extensiones.
- **CLI**: cliente para operar las mismas capacidades desde terminal.
- **Runtime adapters**: interfaz controlada; v0.1 comienza con `claude-code`
  y `codex`, no con ejecucion arbitraria de binarios del host.
- **NovaCore**: chat opcional que administra recursos y propone runs con
  confirmacion; el trabajo tecnico lo ejecutan los adapters.

## Persistencia

| Capa | Responsabilidad |
|---|---|
| PostgreSQL 16 | projects, domains, goals, tasks, agents, skills, repositories, runs, approvals, usage, audit |
| SQLite + FTS5 | memorias operativas buscables por proyecto/agente/scope |
| Filesystem gestionado | clones Git, journals, artefactos, previews y snapshots |
| Git/GitHub | historial entregable del codigo, solo mediante aprobaciones |
| Markdown/Obsidian opcional | export humano en v0.2; nunca fuente canonica temprana |

## Ejecucion supervisada

1. El usuario o NovaCore propone un run ligado a proyecto/tarea/agente.
2. BattOS presenta runtime, permisos, repositorio, red y estimacion disponible.
3. El usuario aprueba; el worker crea un contenedor efimero con workspace
   controlado.
4. La red parte apagada y solo se habilita mediante toggle auditado.
5. Logs y estado llegan al dashboard por SSE; memoria y artefactos se guardan.
6. El usuario revisa resultados y diff. Commit y push requieren aprobaciones
   independientes. Deploy queda fuera de v0.1.

## Seguridad base

- Secretos por referencias seguras/env, nunca en logs ni prompts persistidos
  sin sanitizacion.
- Ejecucion solo mediante adapters permitidos y contenedores por run.
- Acceso a repositorios expresamente conectado o creado en BattOS.
- Red, commit y push son acciones visibles y auditadas.
- Sin shell arbitraria sobre el host ni autonomia indefinida.

## Despliegue

- **Desarrollo**: API y CLI Go, PostgreSQL y Docker local; web Next.js cuando
  la fase de interfaz comience.
- **VPS/self-hosted**: API, web, PostgreSQL y worker con Docker Engine para los
  contenedores de runs; proxy TLS delante. Obsidian no se instala en el VPS.

Ver `docs/10-roadmap.md`, `docs/14-producto-final-y-roadmap.md`,
`docs/adr/0010-knowledge-workspace-opcional.md` y
`docs/adr/0011-v01-ejecucion-supervisada.md`.
