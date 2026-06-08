# 10 - Roadmap

> Estado revisado el 4 de junio de 2026. El producto final y sus capacidades
> se describen en `docs/14-producto-final-y-roadmap.md`.
> El plan perseguible con pendientes, testing y criterios de cierre vive en
> `docs/15-plan-de-objetivos.md`.

## Estado Actual Verificado

| Fase | Entrega | Estado |
|---|---|---|
| 0 | Bootstrap, stack y docs iniciales | Completada |
| 1 | API `/health`, `/version`, `/status` y CLI `status` | Completada |
| 2 | SQLite unificado + Memory Core FTS5 + CLI/HTTP memory | Completada |
| 3A | OpenAPI, autenticacion/secretos y lifecycle de runs/approvals | Completada |
| 3B | Work model y Knowledge Center | Completada |
| 4A | Runtimes y adapters (Claude Code/Codex) | Completada |
| 4B | Runs aislados, logs SSE y persistencia | Completada |
| 4C | Repositories, diff, commit y push supervisados | Completada |
| 4D | Memory Bridge | Completada |
| 5A | NovaCore | Completada base |
| 5B | Dashboard y usage | En curso base |

Validacion disponible:

```powershell
go test ./apps/api/... ./apps/cli/... ./packages/core/...
```

Actualizacion SQLite unificado:

- BattOS v0.1 usa un unico archivo `data/battos.db` como fuente operacional.
- `apps/api/sqlc.yaml` esta retargeteado a `engine: sqlite` con
  `database/sql`.
- API, worker, CLI, dashboard y smokes dev ya no requieren `DATABASE_URL` ni
  servicio Postgres.
- Postgres queda fuera del camino normal de v0.1; las migraciones antiguas se
  conservan como referencia historica append-only.
- Revalidacion del 7 de junio de 2026:
  - `smoke-battos-dev.ps1 -RequireDatabase -UseGoRun` paso contra SQLite fresca.
  - `smoke-battos-web.ps1 -RequireDatabase -CheckSSE` paso contra SQLite fresca.
  - Lifecycle dry-run `queued -> succeeded` paso con worker sobre la misma DB.
  - `go test ./apps/api/... ./apps/cli/... ./packages/core/...`, builds Go,
    `npm run lint`, `npm run check:api-types` y `npm run build` pasaron.
  - Se agrego y ejecuto `scripts/verify-battos-sqlite-release.ps1
    -SkipWebBuild`; la gate levanta API con SQLite fresca, corre smokes dev/web
    y valida lifecycle dry-run.
  - La gate ahora soporta `-CheckRealAdapters -RealAdapter all` para cerrar
    `codex` y `claude-code` reales en el mismo flujo cuando existan Docker y
    provider keys.
  - Se inicio el modo `host_session` para Codex: `codex-host-session` queda
    registrado solo cuando `execution.host_session_enabled=true`, monta
    `.codex` read-only dentro del runtime image y tiene smoke dedicado.
  - Smoke real `scripts/smoke-battos-codex-host-session-run.ps1` paso el 8 de
    junio de 2026 con DockerSandbox, red aprobada, sesion local `.codex`, marker
    `battos-codex-host-session-ok` y artifact registrado.
  - Se agrego `claude-code-host-session` y smoke dedicado para validar la sesion
    local `.claude` sin `ANTHROPIC_API_KEY`.
  - DockerSandbox y Memory Bridge Docker se revalidaron el 7 de junio de 2026
    con `verify-battos-sqlite-release.ps1 -SkipWebBuild -CheckDocker
    -CheckMemoryDocker`; ambos pasaron contra SQLite fresca.

En Windows con Application Control, algunos binarios temporales de `go test`
pueden bloquearse; para paquetes afectados se usa binario de test estable bajo
`data/.cache/dev-bin` o tests focalizados.

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
| 3A | Contratos y decisiones | Completada: OpenAPI; ADR-0013/0014; contrato de CRUD, runs, chats y approvals |
| 3B | Modelo de producto | Domains, goals, tasks, artifacts, journals, workspaces y skills/prompt templates |
| 4A | Runtimes ejecutables | Detector/providers y adapters aprobados `claude-code` y `codex` |
| 4B | Execution engine | Runs en contenedor efimero, red configurable, logs SSE, cancelacion y auditoria |
| 4C | Repositories | Repo local gestionado o GitHub, ramas, diff, commit/push aprobados |
| 4D | Memory Bridge | Context packs, memoria por proyecto/agente, futuro MCP y captura aprobable |
| 5A | NovaCore | Chat opcional para operar BattOS y proponer runs con HITL |
| 5B | Dashboard | Command Center, Work Board, Control Room y Knowledge Center |
| 6 | Hardening/release | Seguridad, backups, tests, instalacion VPS y tag `v0.1.0` |

**Fase 3B** ya tiene operable el Work Board y el Knowledge Center base. Estan
activos el almacenamiento tipado y el CRUD API/CLI para `domains`, `projects`,
`goals`, `tasks`, `knowledge_workspaces`, `journals` y `artifacts`; las listas
de `goals` y `tasks` pueden verse globalmente o filtrarse por proyecto.
Knowledge Center usa buckets `raw`, `wiki` y `outputs`, con artifacts
gestionados en `data/artifacts` y validacion contra path traversal. Tambien
existe una TUI CLI v1 con welcome deck amplio, mascota BattOS pixel-art,
flechas, command palette `/`, footer fijo de atajos, selector de idioma
espanol/ingles, panel de resultados para comandos y fallback lineal para operar
sin salir de sesion, siguiendo `packages/openapi/openapi.yaml`.

Agents Registry empezo a cerrarse como superficie operativa: el contrato
OpenAPI `POST /agents` ya esta conectado al API real y crea identidades de
agente con `slug`, `name`, `runtime_id`, prompt base, estado y defaults seguros
(`risk_level=medium`, `status=active`, `is_lead=false`, `is_meta=false`). Esto
habilita crear agentes ejecutores y luego asignarlos a tasks/runs sin depender
de seed manual. El CLI ya expone `battos agent list`, `battos agent create` y
`battos agent show`; la TUI tambien reconoce `/agents` y `/agent-new`. El
dashboard ahora incorpora `Agents Registry`, una pantalla conectada a
`GET/POST /agents` y `GET /runtime-adapters`, con empty state, creacion de
agente, descripcion de runtime y guardrail explicito: crear un agente no
aprueba ejecucion automatica. Permisos finos y edicion de agentes quedan como
trabajo posterior.

Inbox queda resuelto como proyecto especial `inbox`: BattOS no lo precarga al
arrancar, pero lo crea idempotentemente cuando el usuario captura una task sin
`project_id`. Asi se pueden anotar tareas sueltas y asignarlas despues sin
romper trazabilidad ni volver nullable el modelo operacional. En la TUI,
`/task-new` permite dejar `project id` vacio y usar Inbox.

La base Kanban queda expuesta por CLI/TUI con `task board [--project <id>]`,
`/task-board` y `task position`; el dashboard reutilizara estos mismos campos
`status` y `board_position`. La carpeta TUI `/tasks` tambien expone mover,
asignar, vincular a goal y ordenar tareas.

**Fase 4A** queda completada como base de inventario seguro: BattOS detecta
`claude-code` y `codex`, registra CLIs, marca providers configurados por env
vars y clasifica `configured/detected/unavailable/blocked` sin ejecutar agentes
ni conceder permisos. La interfaz comun de ejecucion queda para Fase 4B, junto
al Execution Engine.

**Fase 4B** comenzo con el control plane de runs: `runs`, `run_approvals` y
`run_logs` viven ahora en SQLite; API/CLI/TUI permiten proponer un run,
listar/ver detalle, aprobar `execute/network/commit/push`, cancelar estados no
terminales y consultar logs persistidos. En esta base, aprobar `execute` mueve
el run a `queued`; todavia no hay worker ni contenedor, por lo que no ejecuta
codigo en host ni en sandbox. Tambien existe una fundacion interna de worker:
puede reclamar `queued`, pasar a `running`, escribir logs y completar/fallar
mediante adapters inyectados en tests. El siguiente bloque de 4B conecta ese
worker a adapters reales, contenedor efimero, captura de logs y SSE.
La frontera de ejecucion ya esta separada: los adapters solo preparan un
`ExecutionPlan` y el `Sandbox` decide como ejecutarlo. Por defecto el modo
configurado es `dry_run`, que registra el plan sin tocar el host; para pruebas
controladas ya existe `sandbox_mode=docker`, que ejecuta en contenedor efimero
con workspace aislado.
Tambien existe un binario dedicado `apps/api/cmd/worker` para procesar runs en
modo dry-run (`go run ./apps/api/cmd/worker -once`) y validar el lifecycle sin
activar ejecucion real.
El mismo binario puede quedar en modo loop con `-once=false -poll 2s`, respetar
Ctrl+C/SIGTERM y dormir cuando no hay runs en cola.
El loop tambien queda empaquetado como servicio opcional `battos-worker` en
Docker Compose, con perfil `worker`, modo `dry_run` por defecto y override
`infra/docker-compose.worker-docker.yml` para activar DockerSandbox montando el
socket de Docker de forma explicita. La imagen Docker fue verificada con build
real y el servicio Compose arranco con `worker loop started; sandbox=dry_run
poll=2s`.
`DockerSandbox` ya esta implementado como modo opcional: usa workspace temporal,
`docker run --rm`, red `none` por defecto, red `bridge` solo con approval,
captura stdout/stderr y limpia al terminar. La validacion real de contenedor
paso con `docker run --rm --network none alpine:3.20`, ejecutado fuera del
sandbox de Codex por permisos del pipe de Docker Desktop.
Tambien paso un run BattOS real con `sandbox-smoke`: `queued -> running ->
succeeded`, stdout persistido en `run_logs`, red deshabilitada y workspace
temporal limpio. El primer intento detecto y corrigio un bug de volumen Docker:
las rutas montadas deben ser absolutas en Windows. El smoke automatizado volvio
a pasar el 1 de junio de 2026 con run `96e0b6a5-d7a2-4d51-b49c-818644dd36d8`.
La captura base de artifacts de run tambien quedo integrada: el contenedor
puede escribir archivos dentro del workspace temporal, DockerSandbox los
devuelve al worker, y el worker los guarda como artifacts gestionados en
`data/artifacts/<project>/outputs/...` con `run_id`. El adapter `sandbox-smoke`
genera `outputs/smoke.md` para probar este flujo sin depender todavia de
credenciales de providers. El smoke Docker paso con run
`c8851834-ec70-4aa1-9fcc-6164d4b5a055`, incluyendo registro de artifact,
`managed_path` y archivo fisico. En Windows, el smoke firma el worker dev en
una ruta estable para evitar el bloqueo de Application Control sobre binarios
temporales de `go run`.
El 1 de junio de 2026 se revalido el mismo flujo usando la imagen runtime
`battos-runtime-agents:dev`: run
`752a9bb2-53ca-4d02-8166-803080f90553` paso con artifact y workspace limpio.
El 4 de junio de 2026 se volvio a consolidar DockerSandbox con API/DB
reales: runs `69e1ac6f-e581-42a0-bf2a-c4682c0cb01e` y
`3190f1f0-95ff-449f-9438-15bfd3c68676` pasaron en `sandbox=docker`,
registrando `outputs/smoke.md`, logs y limpieza de workspace. En la
investigacion se detecto que un servicio Compose `battos-worker` en `dry_run`
puede reclamar la cola antes del smoke local; por eso el worker one-shot ahora
soporta `-run-id <uuid>` y los smokes avisan si el worker Compose esta activo.
Los adapters `codex` y `claude-code` ya generan planes no interactivos para
ejecucion en sandbox: ambos leen el prompt desde `BATTOS_PROMPT_FILE`, pasan
solo la env key de provider que necesitan y dependen de una imagen runtime que
incluya la CLI correspondiente. La imagen runtime dedicada vive en
`infra/Dockerfile.runtime-agents` y se construye con
`scripts/build-battos-runtime-agents.ps1`; el build real ya verifico
`codex-cli 0.136.0` y `Claude Code 2.1.159`. Tambien quedo preparado el smoke
real `scripts/smoke-battos-real-adapter-run.ps1`, que crea un run con red
solicitada, aprueba `network` y `execute`, procesa el worker con DockerSandbox
y valida logs/artifact usando `OPENAI_API_KEY` o `ANTHROPIC_API_KEY`. El
siguiente paso es ejecutarlo con credenciales configuradas y registrar la
evidencia de `codex` y `claude-code`.
La primera superficie SSE de runs tambien quedo activa:
`GET /events/runs/{id}` emite snapshots, logs y cierre terminal
(`run.snapshot`, `run.log`, `run.done`) sin el timeout HTTP de 30s. Fue
verificada contra el run `47485730-a770-4bc4-b8e5-a5b7351ec605`, preparando la
base para el futuro Control Room del dashboard.

**Fase 4C** queda completada. Añadimos el soporte para conectar y gestionar repositorios locales e integraciones supervisadas. El worker aísla los runs en ramas temporales (`battos-run-<id>`), clona localmente, ejecuta y calcula el diff final guardándolo como un artefacto `diff`. Adicionalmente, el endpoint `/runs/{id}/approvals` implementa los approvals para `commit` y `push`: al aprobarse, se ejecutan de manera automatizada en el workspace temporal, realizando el commit local y subiendo los cambios (push) a su origen original, eliminando físicamente la carpeta de trabajo temporal y actualizando la metadata del run. El flujo fue validado con tests unitarios robustos con repositorios git locales simulados en filesystem temporal (`runs_test.go`).
Para repos `github`, el clone y el push usan un remoto `https` autenticado: el
paquete `internal/gitauth` resuelve `credential_ref` como nombre de variable de
entorno, inyecta el token (`x-access-token`) en la URL al vuelo y lo redacta de
logs/errores; el token nunca queda persistido en `.git/config` del workspace
temporal. Decisión documentada en `docs/adr/0019-github-push-auth.md`.

**Fase 4D** queda completada. Consolidamos el **Memory Bridge** como capa transversal de memoria entre agentes y herramientas de BattOS.
Por un lado, la inyección automática de contexto en los prompts del worker ahora busca y combina observaciones en el Memory Core de SQLite mediante políticas ordenadas por relevancia para memorias de proyecto (scope=project), personales del proyecto (scope=personal), del agente específico y memorias globales, deduplicándolas por id de observación en caliente.
Por otro lado, convertimos el guardado de resumen de run en propuesta aprobable (acción `"remember"` HITL). El soporte vive en `run_approvals`, se actualizaron validaciones de tipo en la API, y se integró en el handler HTTP para que al aprobarse con `decision: "approved"`, obtenga los logs del run, renderice el resumen Markdown del run y lo guarde en Memory Core de manera automatizada. Todo fue validado con pruebas unitarias en `memory_context_test.go` y `runs_test.go`.
El MCP server propio y el judgment de conflictos quedan como evolución posterior para v0.2.
El 1 de junio de 2026 se revalido la inyeccion de memoria con imagen runtime:
run `47485730-a770-4bc4-b8e5-a5b7351ec605` paso con memoria de proyecto
inyectada y artifact registrado.
El 4 de junio de 2026 se revalido el mismo flujo con `-RequireMemoryContext`:
run `33329b2e-f96f-41cb-b3cc-bbff4084929c` paso en DockerSandbox, inyecto
memoria de proyecto y registro artifact `outputs/smoke.md`.

**Fase 5B** queda en curso con dashboard base en `apps/web`: Command Center,
Work Board, Control Room, Knowledge Center, Usage & Limits, Settings y NovaCore drawer. La version
implementada usa Next.js 16, documentada en ADR-0018. La validacion actual deja
`npm run lint` sin errores bloqueantes y `npm run build` pasando fuera del
sandbox de Codex; dentro del sandbox de Codex falla con `spawn EPERM` al
ejecutar TypeScript por politica de procesos de Windows. El smoke
`scripts/smoke-battos-web.ps1 -RequireDatabase -CheckSSE` ya valida contra API
real los endpoints usados por Command Center, Work Board, Control Room,
Knowledge, NovaCore, Usage y el primer evento SSE de metricas. La frontera
frontend/API normaliza respuestas `snake_case` a `camelCase`, incluyendo SSE,
para mantener componentes React consistentes con los DTOs Go. El helper SSE del
dashboard ahora reintenta con backoff y conserva `Last-Event-ID`/`after` para
retomar streams sin duplicar logs. El smoke tambien
tiene modo degradado sin `-RequireDatabase`: valida `/status`, Memory Core, SSE
y web shell, saltando endpoints operacionales si `database` esta `down`. Usage &
Limits ya muestra budgets por proyecto, umbrales de alerta y precision de costo
`estimated/not_reported`, con contrato OpenAPI alineado al array real de
`UsageOverviewItem`. El 4 de junio de 2026 el smoke web completo paso con
DB OK, SSE `system.metrics`, shell web y, con `-CheckRunSSE`, el stream
`/events/runs/{id}` de Control Room sobre el run
`3190f1f0-95ff-449f-9438-15bfd3c68676`. Settings ya permite guardar,
reemplazar y limpiar `BATTOS_API_TOKEN` local sin exponer el valor. Tambien
quedo la base de tipos OpenAPI generados para el dashboard con
`openapi-typescript`, `generate:api-types`, `check:api-types` y un puente
camelCase que preserva acronimos internos (`USD`, `MB`, `KBps`). Ya se migraron
helpers de lectura principales a `apiClient` tipado por OpenAPI para status,
work board, registries, runtime adapters, runs/logs/diff y usage. Tambien se
migraron mutaciones criticas de Work Board, Control Room, Knowledge Center y
NovaCore chat usando request bodies camelCase convertidos a `snake_case` por
`snakeizeBody`, con enums tipados para approvals, task status, memory scope y
artifact kind. NovaCore tambien tiene contrato OpenAPI para listar
conversaciones y mensajes, y `NovaChat` ya usa helpers tipados para esas
lecturas. La validacion actual pasa `npm run lint`,
`npm run check:api-types`, `npm run build` fuera del sandbox de Codex y smoke completo
`scripts/smoke-battos-web.ps1 -RequireDatabase -CheckSSE -CheckRunSSE -CheckWeb`.
Quedan pendientes modelar repositories mutables en OpenAPI, estados
offline/degraded finales, budgets configurables por agente/provider/modelo,
providers por referencia y E2E.

Prerequisito seguro ya implementado para Fase 3B: middleware Bearer
configurable, `BATTOS_API_TOKEN` en la CLI y rechazo de auth desactivada sobre
bind publico.

Para desarrollo local Windows, ADR-0015 define que el API se levanta con
`scripts/start-battos-api-dev.ps1` usando `go run`; `battos-api.exe` queda para
release/instalador por las restricciones de Windows Application Control. El
launcher dev ya maneja rutas con espacios y deja logs de background en
`data/logs/start-api-<port>.*.log`.

### Que Podras Hacer En v0.1

- Crear un proyecto, objetivo y task con artifacts de referencia.
- Crear un agente conectado a Claude Code o Codex.
- Pedirle construir o modificar codigo y ejecutar build/tests en contenedor.
- Revisar logs, diff, artifacts, tokens/costo reportado y memoria del run.
- Aprobar commit y push sin habilitar deployment automatico.
- Usar NovaCore para administrar el OS y preparar runs.

Nota multi-runtime: en v0.1 `claude-code` y `codex` son los adapters
ejecutables aprobados. `gemini-cli` ya existe como runtime conocido y provider
Google, pero queda como siguiente adapter a implementar y validar. Cursor y
Antigravity deben evaluarse primero como companion/editor connectors, salvo que
ofrezcan una CLI/API headless segura que pueda pasar por el mismo modelo de
adapter, sandbox, logs y approvals.

### Punto MD A Considerar: Memoria Persistente Transversal

- **Problema que resuelve**: evitar que el contexto quede atrapado en una sola
  herramienta. Si un proyecto se trabajo con Claude Code y luego continua en
  Codex, NovaCore u otro runtime, BattOS debe entregar el mismo historial
  operativo.
- **Base actual**: Memory Core propio con SQLite + FTS5, API/CLI,
  `project_id`, `agent_id`, `scope` y `topic_key`.
- **Primera superficie**: `battos memory context --project <id>` genera un
  context pack Markdown/JSON para pegar o inyectar en un agente.
- **Integracion esperada**: antes de cada run, BattOS busca memoria relevante y
  la adjunta al prompt/contexto. La primera inyeccion automatica usa memoria
  `scope=project`; al terminar, propone guardar resumenes, decisiones, bugfixes
  y preferencias como memoria aprobable.
- **Guardrails**: no guardar secrets, no capturar ruido automaticamente, usar
  `scope=personal` para preferencias del usuario y `scope=project` para
  decisiones del proyecto.
- **Evolucion**: MCP server propio, export/import JSONL, dedupe y conflict
  judgment inspirado en Engram.

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

## Decisiones De Fase 3A

- `docs/adr/0013-auth-y-secretos-v01.md`: token administrador para API,
  secretos por referencia y nunca en logs/artifacts.
- `docs/adr/0014-run-lifecycle-y-approvals.md`: `runs` es la entidad
  supervisada; ejecutar, habilitar red, commit y push se aprueban por
  separado.
