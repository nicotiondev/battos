# 15 - Plan De Objetivos Hacia El Dashboard

> Plan operativo para perseguir el objetivo final de BattOS v0.1: llegar a un
> Mission Control funcional, seguro y testeado, con CLI + dashboard.

## Norte

BattOS v0.1 debe permitir organizar trabajo, conocimiento y agentes; ejecutar
runs supervisados en contenedores; revisar logs, artifacts, diff y consumo; y
aprobar commit/push desde CLI y dashboard.

El dashboard final no se construye como maqueta aislada: cada pantalla debe
estar conectada a APIs reales, datos persistentes, estados de error y pruebas.

## Reglas De Avance

Cada objetivo se considera cerrado solo si cumple:

- API, CLI y docs actualizados cuando aplique.
- Tests unitarios o smoke tests manuales documentados.
- Errores esperados manejados sin crash.
- Estados vacios, degradados y offline con mensajes claros.
- Seguridad revisada: secrets, paths, permisos y acciones peligrosas.
- `docs/10-roadmap.md` actualizado si cambia el alcance.

## Objetivos Pendientes

| Orden | Objetivo | Estado | Criterio de cierre |
|---|---|---|---|
| 0 | Consolidacion dev actual | Completado para dev local | API/DB/CLI/TUI arrancan de forma repetible y sin desfase |
| 1 | Cerrar Knowledge Center | Completado base | Workspaces, journals y artifacts operables por API/CLI |
| 2 | Mejorar Work Board | Completado base | Detalles, estados, asignaciones, Inbox y Kanban base cerrados |
| 3 | Agents Registry | En curso consolidada | Crear agentes con runtime, permisos y prompt base desde API/CLI/dashboard |
| 4 | Runtime detection | Completado base | Claude Code/Codex detectados sin ejecutar nada automaticamente |
| 5 | Execution engine | Completado base | Runs aislados, aprobables, cancelables y auditables; smoke real Codex/Claude pendiente por keys |
| 6 | Memory Bridge | Completado base | Memoria compartida entre Claude Code, Codex, NovaCore y dashboard |
| 7 | Repositories/Git | Completado base | Repo local, branch, diff, commit y push con aprobaciones separadas |
| 8 | NovaCore | Completado base | Chat del OS con contexto read-only; requiere provider key para uso real |
| 9 | Dashboard | En curso base | Command Center, Work Board, Control Room y Knowledge Center en `apps/web` |
| 10 | Usage y budgets | En curso base consolidada | Usage events por run; budgets/alertas base; falta configuracion avanzada |
| 11 | Hardening/release | Pendiente | Backups, instalacion, smoke tests y tag v0.1.0 |

## SQLite Unificado

Estado: implementado base antes del cierre de v0.1.

- BattOS crea/usa `data/battos.db` por defecto y acepta override con
  `BATTOS_DATABASE_PATH` / `database.path`.
- API, worker y Memory Core comparten SQLite; no hay backend dual ni importador
  Postgres en esta fase.
- `sqlc` fue retargeteado a SQLite y los handlers usan tipos portables
  (`string`, `sql.NullString`, `time.Time`, `sql.ErrNoRows`).
- Los mensajes normales del dashboard, router y scripts ya no piden configurar
  Postgres.
- Revalidado el 7 de junio de 2026 con DB fresca en
  `data/.cache/release-smoke/battos.db`: `smoke-battos-dev.ps1
  -RequireDatabase -UseGoRun`, `smoke-battos-web.ps1 -RequireDatabase
  -CheckSSE`, lifecycle dry-run `queued -> succeeded`, `go test
  ./apps/api/... ./apps/cli/... ./packages/core/...`, builds Go y checks web.
- Gate reproducible agregada: `scripts/verify-battos-sqlite-release.ps1`
  crea una SQLite fresca temporal, ejecuta tests/builds/checks, levanta API,
  corre smokes dev/web y valida lifecycle dry-run. Paso con `-SkipWebBuild` el
  7 de junio de 2026; el build web completo fue validado fuera del sandbox.
- La misma gate ahora acepta `-CheckDocker`, `-CheckMemoryDocker` y
  `-CheckRealAdapters -RealAdapter all` para convertir el cierre pendiente en
  un comando unico cuando Docker Desktop/daemon y `OPENAI_API_KEY` /
  `ANTHROPIC_API_KEY` esten disponibles.
- Se agrego el camino recomendado para Codex con suscripcion/OAuth:
  `codex-host-session`, habilitado solo con
  `execution.host_session_enabled=true`, monta la carpeta `.codex` del host en
  modo read-only y se valida con
  `scripts/smoke-battos-codex-host-session-run.ps1` o
  `verify-battos-sqlite-release.ps1 -CheckHostSessionAdapters`.
- Smoke `codex-host-session` paso el 8 de junio de 2026 contra API SQLite local:
  DockerSandbox ejecuto Codex con `CODEX_HOME` efimero, red aprobada, artifact
  `outputs/host-session-smoke.md` y marker `battos-codex-host-session-ok`.
- Se agrego `claude-code-host-session` con montaje `.claude` read-only, copia
  efimera dentro del contenedor y smoke
  `scripts/smoke-battos-claude-host-session-run.ps1`.
- Se corrigio un bug release-critical de sqlc: un comentario no ASCII en
  `apps/api/queries/registries.sql` truncaba queries generadas (`ORDER BY i`,
  `WHERE id =`). Ahora `apps/api/internal/store/pool_test.go` ejecuta queries
  reales de registries contra SQLite limpia.
- Revalidado despues de habilitar Docker Desktop:
  `verify-battos-sqlite-release.ps1 -SkipWebBuild -CheckDocker
  -CheckMemoryDocker` paso el 7 de junio de 2026, incluyendo DockerSandbox y
  Memory Bridge Docker contra SQLite fresca.
- Pendiente fuera de esta fase: importador Postgres -> SQLite,
  allowlist fina de egress, smokes reales con
  provider keys y polish/E2E final.

## 0. Consolidacion Dev Actual

Pendientes:

- Unificar arranque local con `scripts/start-battos-api-dev.ps1`.
- Agregar smoke test local con `scripts/smoke-battos-dev.ps1`.
- Evitar desfase entre `battos.exe` nuevo y API viejo.
- Documentar el bloqueo de `battos-api.exe` por Windows Application Control y
  el launcher dev oficial.
- Verificar comandos base:
  - `battos status`
  - `battos project list`
  - `battos goal list`
  - `battos task list`
  - `battos memory stats`
- Revisar TUI con API apagada, API sin DB y API OK.

Testing/debug:

- Arranque dev:
  `powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 -StopExisting -Background -Wait`.
- Smoke dev:
  `powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-dev.ps1 -RequireDatabase`.
- API apagada: CLI/TUI muestra mensaje accionable, no crash.
- API con `data/battos.db`: Work Board responde y listas globales no piden
  `project_id`.
- Windows: `docs/adr/0015-windows-dev-api-launcher.md` define `go run` via
  launcher como flujo dev oficial; `battos-api.exe` queda para release/instalador.

Evidencia actual:

- `start-battos-api-dev.ps1 -StopExisting -Background -Wait` deja `/status`
  respondiendo con `database: ok`.
- `smoke-battos-dev.ps1 -RequireDatabase` valida `status`, `project list`,
  `goal list`, `task list` y `memory stats`.
- `start-battos-api-dev.ps1 -Port 8001 -Background -Wait` permite probar otro
  API local con su SQLite configurada por `BATTOS_DATABASE_PATH`.
- `battos --api http://127.0.0.1:8999 status` falla con mensaje accionable
  cuando el API esta apagada.
- API sin `DATABASE_URL` arranca igual; `DATABASE_URL` ya no participa en el
  flujo normal de v0.1.
- Tests firmados ejecutados: `apps/api/internal/server` y
  `apps/cli/internal/commands`.
- Decision Windows documentada en ADR-0015: no confiar el certificado local en
  TrustedRoot por defecto; usar launcher dev con `go run`.
- Launcher dev corregido para carpetas con espacios como `CLAUDE CODE` y logs
  de background en `data/logs/start-api-<port>.out.log` /
  `data/logs/start-api-<port>.err.log`.

## 1. Knowledge Center

Pendientes:

- API para `knowledge_workspaces`. Implementado.
- API para `journals`. Implementado.
- API para `artifacts`. Implementado.
- CLI para listar/crear workspaces, journals y artifacts. Implementado.
- Asociar artifacts a proyecto, task y futuro run. Implementado a nivel de
  indice; `run_id` queda listo para Fase 4.
- Definir buckets canonicos: `raw`, `wiki`, `outputs`. Implementado.
- Definir almacenamiento filesystem gestionado para artifacts. Implementado.
- Documentar flujo de brief, referencia y output. Implementado.

Testing/debug:

- Crear workspace por proyecto existente.
- Rechazar workspace para proyecto inexistente.
- Crear journal y artifact Markdown/link.
- Listar artifacts por proyecto.
- Bloquear path traversal y rutas fuera de `data/`. Implementado.
- Probar API con SQLite local inicializada y errores de DB manejados.

Evidencia actual:

- Handler API montado en `/knowledge/workspaces`, `/journals` y `/artifacts`.
- CLI `battos knowledge workspace|journal|artifact list/create`.
- Journals infieren `project_id` desde `workspace_id` y rechazan mismatch.
- Artifacts exigen `content`, `managed_path` o `external_url`.
- Artifacts con `content` se escriben en `data/artifacts/<project>/<bucket>/...`
  cuando no son enlaces externos.
- Buckets canonicos: `raw`, `wiki`, `outputs`.
- Path traversal en `managed_path` rechazado con HTTP 400.
- Smoke dev ahora valida `knowledge workspace list`.
- Smoke manual end-to-end: proyecto temporal, workspace, journal y artifact
  creados y listados por CLI.
- Smoke manual de storage: artifact markdown creado en bucket `wiki`, archivo
  verificado en `data/artifacts` y traversal rechazado.
- API con DB caída responde errores limpios sin pedir configurar Postgres.
- Tests unitarios: `apps/api/internal/handlers`, `apps/api/internal/server` y
  `apps/cli/internal/commands`.

## 2. Work Board Avanzado

Pendientes:

- Decidir Inbox. Implementado como proyecto especial `inbox`, creado
  idempotentemente al capturar una task sin proyecto.
- Agregar detalle de proyecto y task. Implementado en CLI.
- Cambiar estado de task. Implementado en CLI/API.
- Asignar task a proyecto. Implementado en CLI/API.
- Vincular task a goal. Implementado en CLI/API.
- Preparar datos para Kanban. Implementado con `status`, `board_position`,
  `task board`, `/task-board` y `task position`.

Comandos candidatos:

```powershell
battos task assign <task_id> --project landing-acme
battos task link-goal <task_id> --goal <goal_id>
battos task move <task_id> --status in_progress
```

Testing/debug:

- No permitir task vinculada a goal de otro proyecto.
- Rechazar estados invalidos.
- Mantener listados globales y filtrados.
- TUI debe poder volver con `Esc` y salir solo con `Ctrl+C`.

Evidencia actual:

- CLI: `project show`, `goal show`, `task show`, `task board`, `task move`,
  `task assign`, `task link-goal`, `task position`.
- API: `PATCH /tasks/{id}` permite cambiar `project_id` y valida que
  `goal_id` pertenezca al proyecto final de la task.
- Smoke manual con `go run`: link-goal cruzado rechazado con HTTP 400; luego
  `assign`, `link-goal`, `move` y `show` funcionan.
- `smoke-battos-dev.ps1 -UseGoRun` permite validar cuando Windows App Control
  bloquea el `battos.exe` recompilado.
- Inbox resuelto: `battos task create --title "Idea suelta"` crea la task en
  `inbox`; `task assign` la mueve luego a un proyecto real.
- TUI: `/task-new` permite dejar `project id` vacio; Enter usa `inbox`.
- Kanban base: `battos task board [--project <id>]` y `/task-board` agrupan
  tareas por estado; `task position` ajusta `board_position`.
- TUI de tareas expone acciones de operacion: `/task-move`, `/task-assign`,
  `/task-link-goal` y `/task-position`.

## 3. Agents Registry

Objetivo: poder crear identidades de agentes dentro de BattOS, asignarles un
runtime aprobado y luego usarlos en tasks/runs sin depender de seed manual.

Concretado:

- Tabla `agents` existe desde el schema inicial.
- API `GET /agents` lista agentes para dashboard, Work Board y Control Room.
- API `POST /agents` ya crea agentes con `slug`, `name`, `role`,
  `description`, `runtime_id`, `system_prompt`, `risk_level` y `status`.
- CLI `battos agent list|create|show`.
- TUI slash commands `/agents` y `/agent-new`.
- Defaults seguros al crear: `risk_level=medium`, `status=active`,
  `runtime_config={}`, `allowed_tools=[]`, `allowed_projects=[]`,
  `is_lead=false`, `is_meta=false`.
- OpenAPI modela `AgentInput` y `Agent`; el cliente web tiene helper tipado
  `apiClient.createAgent`.
- Tests de handler validan defaults y rechazo cuando falta `runtime_id`.
- Tests CLI validan que `agent create` envie el body esperado y que `agent list`
  lea `/agents`.
- Dashboard `Agents Registry` lista agentes, muestra runtimes, diferencia
  runtimes ejecutables v0.1 de catalogo/futuro, maneja estado vacio/degradado y
  crea agentes con `apiClient.createAgent`.

Pendientes:

- Permisos finos: `allowed_tools`, `allowed_projects` y `runtime_config`.
- API/CLI update para editar agentes existentes.
- Validar UX para cambiar runtime sin perder identidad/memoria.
- Filtros por agente/proyecto en Memory Bridge y Usage.

Testing/debug:

- Crear agente con `runtime_id=codex`.
- Crear agente con `runtime_id=claude-code`.
- Rechazar runtime inexistente por FK con mensaje claro.
- API con DB caída responde 503 limpio.
- Dashboard debe mostrar empty state cuando no hay agentes y CTA para crear.
  Implementado base; falta smoke visual final con API local en navegador.

## 4. Runtime Detection

Pendientes:

- Detector de `claude-code`. Implementado base.
- Detector de `codex`. Implementado base.
- Registro de runtimes/providers. Implementado base.
- Estados: `detected`, `configured`, `unavailable`, `blocked`. Implementado base.
- Validar presencia de provider keys sin exponerlas. Implementado base.
- Adapter interface comun. Pasa a Fase 4B como parte del Execution Engine; en
  4A solo existe inventario seguro.

Testing/debug:

- CLI instalada/no instalada.
- CLI bloqueada por politica del SO.
- Provider sin key.
- Provider con key.
- Detectar no concede permiso de ejecucion.

Evidencia actual:

- API: `GET /runtime-adapters`, `POST /runtime-adapters/detect`,
  `GET /cli-tools`, `GET /providers`, `POST /providers/detect`.
- CLI: `battos runtime list|detect`, `battos provider list|detect`,
  `battos cli-tool list`.
- Detector usa `exec.LookPath` y solo llama `--version` con timeout corto; no
  ejecuta prompts, agentes ni shells arbitrarias.
- `approved_for_execution=false` siempre en el resultado de detection.
- Provider detection solo revisa presencia de env vars (`*_API_KEY`), no imprime
  valores.
- Tests cubren estados `configured`, `detected`, `blocked` y `unavailable` sin
  ejecutar CLIs reales.
- CLI muestra `APPROVED=no` y recuerda que deteccion/configuracion no autoriza
  ejecucion.

## 5. Execution Engine

Estado actual: control plane base y fundacion de worker implementados. BattOS
ya puede proponer runs, persistir approvals, dejar `execute` en `queued`,
habilitar red si fue solicitada, cancelar runs no terminales y consultar logs
persistidos. Internamente ya existe un worker testeado que reclama un run
`queued`, lo pasa a `running`, escribe logs y lo completa o falla mediante un
adapter inyectado. La frontera adapter/sandbox ya existe: un adapter solo
prepara un `ExecutionPlan`, y el `Sandbox` es el unico responsable de ejecutar
o simular. Todavia no ejecuta workloads reales: runtime adapters activos,
contenedor y streaming SSE quedan para el siguiente bloque.

Concretado:

- Migracion `0003_runs.sql` con `runs`, `run_approvals`, `run_logs` y FK desde
  `artifacts.run_id`.
- API `GET/POST /runs`, `GET /runs/{id}`, `POST /runs/{id}/approvals`,
  `POST /runs/{id}/cancel` y `GET /runs/{id}/logs`.
- Estados `draft`, `awaiting_approval`, `queued`, `running`, `succeeded`,
  `failed`, `cancelled`.
- CLI `battos run list|propose|show|approve|cancel|logs`.
- TUI con `/runs`, `/run-propose`, `/run-approve` y `/run-logs`.
- Guardrail: proponer un run no ejecuta nada; aprobar `execute` solo lo deja
  `queued` hasta que exista worker.
- Guardrail: red solo se habilita con approval `network` si el run la solicito.
- Queries de lifecycle: `ClaimNextQueuedRun`, `AppendRunLog`, `CompleteRun` y
  `FailRun`.
- Worker interno con interfaz `Adapter` inyectable y tests de no-op, exito,
  adapter faltante y error de adapter.
- Boundary adapter/sandbox: adapters `codex` y `claude-code` preparan planes
  aprobados; `DryRunSandbox` registra el plan sin ejecutar comandos del host.
- Adapters reales preparados: `codex` usa `codex exec` no interactivo,
  sandbox `workspace-write`, approval interno `never`, prompt desde
  `BATTOS_PROMPT_FILE` y `OPENAI_API_KEY` como unica env de provider;
  `claude-code` usa `claude --bare --print` no interactivo, prompt desde
  `BATTOS_PROMPT_FILE` y `ANTHROPIC_API_KEY` como unica env de provider.
- Guardrail de secretos: DockerSandbox solo pasa env keys declaradas por el
  adapter y redacta valores conocidos si aparecen en stdout/stderr.
- Config `execution.worker_enabled=false`, `sandbox_mode=dry_run` y timeout por
  defecto; modos no implementados fallan temprano.
- Binario dedicado `apps/api/cmd/worker`: procesa runs `queued` en modo dry-run
  con `go run ./apps/api/cmd/worker -once`, sin ejecutar comandos reales.
- Worker loop: `go run ./apps/api/cmd/worker -once=false -poll 2s` mantiene el
  proceso escuchando la cola, respeta shutdown por contexto/Ctrl+C y duerme
  cuando no hay runs.
- Worker Compose opcional: `battos-worker` corre con perfil `worker` en
  `infra/docker-compose.yml`, por defecto en `dry_run`.
- Override DockerSandbox: `infra/docker-compose.worker-docker.yml` activa
  `sandbox_mode=docker` y monta `/var/run/docker.sock` solo cuando el usuario lo
  pide explicitamente.
- Imagen Docker verificada: `docker compose ... build battos-worker` compila
  `battos-api` y `battos-worker` dentro de la imagen.
- Servicio Compose verificado: `battos-worker` arranca con el entrypoint correcto
  y loguea `worker loop started; sandbox=dry_run poll=2s`.
- `DockerSandbox` opcional implementado: crea workspace temporal, escribe
  `BATTOS_PROMPT.md`, ejecuta `docker run --rm`, usa `--network none` por
  defecto o `bridge` solo si el run aprobo red, captura stdout/stderr y limpia
  workspace al terminar.
- Adapter interno `sandbox-smoke` para validar lifecycle real con Docker sin
  depender todavia de binarios `codex` o `claude` dentro de la imagen.
- Smoke automatizado `scripts/smoke-battos-docker-run.ps1` valida API/DB,
  migraciones, Docker, runtime/agente smoke, run `queued`, worker Docker,
  logs esperados y limpieza de workspace.
- Smoke Docker repetido despues de endurecer adapters/env/redaccion: run
  `c033d4cb-b4b7-467d-9b1a-9b2f22a1ed83` paso `queued -> running ->
  succeeded` sin romper `sandbox-smoke`.
- Imagen runtime dedicada agregada: `infra/Dockerfile.runtime-agents` instala
  `codex` y `claude` en una imagen separada de API/worker.
- Script `scripts/build-battos-runtime-agents.ps1` construye
  `battos-runtime-agents:dev` y verifica `codex --version` / `claude --version`
  sin ejecutar prompts ni tocar providers.
- Build real verificado: `battos-runtime-agents:dev` contiene
  `codex-cli 0.136.0` y `Claude Code 2.1.159`.
- DockerSandbox verificado usando la imagen runtime: smoke `sandbox-smoke` paso
  con run `28f7fddb-a1d6-4eb7-9f39-7d463f44eff2`.
- Artifacts de run base implementados: DockerSandbox escanea el workspace
  temporal al terminar, ignora `BATTOS_PROMPT.md`, detecta artifacts Markdown,
  diff e imagenes, y devuelve los outputs al worker.
- Worker registra artifacts producidos por runs: escribe archivo gestionado en
  `data/artifacts/<project>/outputs/...`, guarda indice en SQLite con
  `run_id`, `task_id` y `project_id`, y loguea el resultado. Si falla la
  escritura fisica, no registra un `managed_path` roto.
- Smoke `sandbox-smoke` ahora produce `outputs/smoke.md` para validar el flujo
  completo de artifact en DockerSandbox.
- Smoke Docker con artifact verificado: run
  `c8851834-ec70-4aa1-9fcc-6164d4b5a055` paso `queued -> running ->
  succeeded`, registro `outputs/smoke.md` con `run_id`, valido `managed_path`
  y archivo fisico bajo `data/artifacts`.
- Revalidacion con imagen runtime `battos-runtime-agents:dev`: run
  `752a9bb2-53ca-4d02-8166-803080f90553` paso `queued -> running ->
  succeeded` el 1 de junio de 2026, validando logs, artifact y limpieza de
  workspace sin depender de credenciales de provider.
- Smoke Docker endurecido para Windows App Control: compila
  `data/.cache/dev-bin/battos-worker-dev.exe`, lo firma con el certificado dev
  local documentado y usa ese binario estable en vez del `worker.exe` temporal
  de `go run`.
- Smoke real de adapters preparado: `scripts/smoke-battos-real-adapter-run.ps1`
  valida API/DB/migraciones/Docker, exige la key del provider, verifica que la
  imagen `battos-runtime-agents:dev` contenga la CLI, registra agente de smoke,
  solicita red, aprueba `network` y `execute`, corre el worker con
  `DockerSandbox`, valida logs/artifact y muestra logs si el run falla. Soporta
  `-Adapter codex`, `-Adapter claude-code` y `-Adapter all`.
- Worker one-shot endurecido: `apps/api/cmd/worker` acepta
  `-run-id <uuid>` para procesar un run especifico. Esto evita que smokes
  locales tomen otra corrida cuando existe actividad paralela y hace visible el
  modo activo con `worker once started; sandbox=<modo>`.
- Smokes Docker endurecidos contra carrera de workers: los scripts de
  DockerSandbox y adapters reales usan `-run-id` y abortan con mensaje claro si
  el servicio Compose `battos-worker` esta corriendo, porque un worker en
  `dry_run` puede reclamar el run antes del smoke local.
- Revalidacion DockerSandbox del 4 de junio de 2026: al detener temporalmente
  `battos-worker` Compose, los runs
  `69e1ac6f-e581-42a0-bf2a-c4682c0cb01e` y
  `3190f1f0-95ff-449f-9438-15bfd3c68676` pasaron en
  `sandbox=docker`, con artifact `outputs/smoke.md`, logs esperados y
  workspace limpio.
- SSE base para runs implementado: `GET /events/runs/{id}` emite
  `run.snapshot`, `run.log`, `run.done` y `run.error`, queda montado fuera del
  timeout HTTP de 30s y fue verificado contra el run
  `47485730-a770-4bc4-b8e5-a5b7351ec605`.
- Tests de handlers y shell aliases.

Pendientes:

- Ejecutar y verificar smoke real de `codex` usando la imagen runtime y
  `OPENAI_API_KEY`.
- Ejecutar y verificar smoke real de `claude-code` usando la imagen runtime y
  `ANTHROPIC_API_KEY`.
- Logs de ejecucion real de adapters Codex/Claude.
- Artifacts de run con adapters reales Codex/Claude y limites de tamano/tipo
  para archivos grandes o binarios.
- Consumo reportado por adapter cuando exista.

Testing/debug:

- Run exitoso.
- Run fallido con logs.
- Run cancelado.
- Timeout.
- Contenedor se limpia.
- Docker daemon apagado devuelve error claro.
- Smoke Docker base con red apagada. Verificado.
- Run BattOS real `queued -> running -> succeeded` en Docker sin red.
  Verificado con runs `3e753bfd-96d0-4162-88f0-783ae2b66f8c` y smoke
  automatizado `356c74d2-bb36-4c6d-bb2e-a8c7767e1244`.
- Workspace temporal se limpia al terminar. Verificado.
- Workspace temporal queda `0777` antes de montar para que la imagen runtime
  no-root pueda escribir dentro del contenedor.
- No tocar host.
- No filtrar secrets.
- Planes de adapter no aceptan env keys arbitrarias ni shell generado por
  usuario.
- Artifact producido por contenedor queda registrado con `run_id` y archivo
  gestionado. Implementado en tests unitarios; smoke Docker lo valida mediante
  `outputs/smoke.md`.
- Paths de artifacts producidos por contenedor se normalizan a `/` para que la
  API sea portable entre Windows y Linux.
- SSE reconecta o falla de forma controlada.

## 6. Memory Bridge

Objetivo: convertir Memory Core en una capa transversal para que cualquier
runtime o herramienta pueda leer/escribir contexto persistente del proyecto,
sin quedar amarrado a Claude Code, Codex, Engram o una app especifica.

Concretado:

- Memory Core SQLite + FTS5 con HTTP/CLI.
- Campos nativos `project_id`, `agent_id`, `scope` y `topic_key`.
- CLI `battos memory save|search|recent|stats`.
- ADR-0017 define Memory Bridge como superficie comun.
- CLI `battos memory context` genera un context pack Markdown/JSON por
  proyecto/agente/scope para inyectar en prompts o runs.
- Worker inyecta Memory Context de proyecto en el prompt del run antes de
  ejecutar el sandbox, usando `scope=project` y `project_id` del run.
- Adapter interno `sandbox-memory-smoke` valida que el prompt del contenedor
  incluya `BattOS Memory Context` y una memoria de proyecto esperada.
- Smoke Docker con Memory Bridge verificado: run
  `c4fd076d-59fa-45d5-b29d-c552bd68261b` paso `queued -> running ->
  succeeded`, logueo `memory context injected (1 items)`, el contenedor emitio
  `battos-memory-context-ok` y registro artifact `outputs/smoke.md`.
- Revalidacion de Memory Bridge con imagen runtime: run
  `47485730-a770-4bc4-b8e5-a5b7351ec605` paso `queued -> running ->
  succeeded` el 1 de junio de 2026, guardando memoria de proyecto e
  inyectandola en el prompt del contenedor.
- CLI `battos run remember <run_id>` guarda explicitamente un resumen de run en
  Memory Core. Por defecto exige run terminal, incluye logs, no incluye prompt
  completo salvo `--include-prompt`, y usa `topic_key` estable para upsert.
- Loop Memory Bridge verificado: `battos run remember
  c4fd076d-59fa-45d5-b29d-c552bd68261b --topic-key
  battos/runs/c4fd076d-memory-smoke` guardo memoria #3; luego
  `battos memory context --project smoke-docker-20260601211840` incluyo el
  resumen del run y la memoria previa del proyecto.

- Ampliar inyección automática de contexto en los prompts de worker para combinar observaciones del Memory Core de SQLite mediante políticas ordenadas por relevancia para memorias de proyecto (scope=project), personales del proyecto (scope=personal), del agente específico y memorias globales, deduplicándolas por id de observación en Go.
- Convertir el guardado de resumen de run (`remember`) en propuesta aprobable a través del flujo de approvals HITL, validando tipos de aprobación, e integrando en el handler HTTP para que al aprobarse con `decision: "approved"`, obtenga logs del run, renderice el resumen Markdown del run y lo guarde en Memory Core de manera automatizada.
- Validado con pruebas unitarias en `memory_context_test.go` y `runs_test.go`.

Pendientes (v0.2+):

- Marcar outputs/artifacts promovidos a memoria.
- Vista dashboard de memoria por proyecto/run.
- MCP server propio para que herramientas externas consuman BattOS Memory Core.
- Dedupe/conflict judgment inspirado en Engram.
- Export/import JSONL por proyecto.

Testing/debug:

- Context pack sin memorias muestra estado vacio claro.
- Context pack filtra por `project_id`, `agent_id` y `scope`.
- Worker registra log `memory context injected` cuando adjunta memoria al run.
- Smoke Docker con `-RequireMemoryContext` debe pasar `queued -> succeeded`,
  dejar `memory context injected` en logs y producir artifact. Verificado.
- Revalidacion del 4 de junio de 2026: `smoke-battos-docker-run.ps1
  -RequireMemoryContext` paso con run
  `33329b2e-f96f-41cb-b3cc-bbff4084929c`, confirmando que el contexto de
  memoria llega al prompt dentro de DockerSandbox.
- `run remember` no guarda prompts por defecto y rechaza runs no terminales
  salvo override explicito.
- `memory context` muestra un run recordado junto con memorias previas del
  proyecto. Verificado.
- No incluir secrets ni env vars en memoria.
- Guardado automatico empieza como sugerencia aprobable.
- `topic_key` evita duplicar preferencias/decisiones estables.
- API offline o Memory Core caido debe degradar sin romper runs.

## 7. Repositories Y Git

Concretado:

- Registrar repositorios por base de datos, soportando el CRUD y endpoint HTTP en `/repositories`.
- Crear repositorio local gestionado (`managed_local`) inicializado automáticamente físicamente en el host (`git init` + commit inicial con README).
- Crear rama dedicada por run (`battos-run-<id>`) y clonar el repositorio original en un directorio de trabajo temporal.
- Generar y persistir diff (`git diff`) del run en la base de datos como un artefacto `diff` asociado al `run_id`.
- Implementar los approvals HITL para `commit` y `push`: al aprobarse, se ejecutan `git commit` y `git push` de forma supervisada en el workspace temporal, borrando la carpeta de trabajo y actualizando la base de datos al finalizar el push.
- Validado con pruebas unitarias robustas en [runs_test.go](file:///c:/Users/nicoa/Desktop/CLAUDE CODE/battos/apps/api/internal/handlers/runs_test.go) utilizando repositorios git reales creados en carpetas temporales (`t.TempDir()`).
- Autenticación GitHub por referencia de credenciales (ADR-0019): repos `github`
  clonan y pushean contra un remoto `https` autenticado. El paquete
  `internal/gitauth` resuelve `credential_ref` como nombre de env var, inyecta el
  token (`x-access-token`) en la URL al vuelo y lo redacta de logs/errores. El
  token nunca se persiste en `.git/config` (el clone restaura el remoto limpio).
  Conectar un repo `github` exige `remote_url`; aprobar `push` sin un
  `credential_ref` resoluble devuelve 400. Cubierto por tests en
  `gitauth_test.go` y `runs_test.go`.

Pendientes:

- Pull request GitHub aprobado como paso posterior al push (v0.2).
- UI de gestión de credenciales/repositorios mutables (v0.2).

## 8. NovaCore

Pendientes:

- Crear agente NovaCore como asistente del OS.
- Chat CLI/API.
- Tools read-only: status, listar proyectos, tareas, memoria y docs.
- Tools mutantes con confirmacion: crear proyecto/task y proponer run.
- Contexto desde docs + Memory Core.
- Guardar conversaciones.
- Registrar uso/tokens.

Testing/debug:

- NovaCore no ejecuta sin confirmacion.
- Provider caido no rompe BattOS.
- Mutaciones requieren confirmacion humana.
- No puede leer secrets.
- Responde con estado real del sistema, no con supuestos.

## 9. Dashboard

Estado actual: base visual y funcional en curso dentro de `apps/web`.
La app ya compila como build de produccion y puede consumirse contra la API
local, pero todavia no se considera dashboard final porque faltan cliente API
generado, auth, pruebas E2E y limpieza de deuda TypeScript/React.

Concretado:

- Crear `apps/web` con Next.js/React.
- Layout principal tipo Mission Control con sidebar, metricas superiores y
  paneles base.
- Command Center base.
- Work Board base conectado a endpoints reales.
- Agents Registry base conectado a endpoints reales.
- Control Room base para runs/logs/diff.
- Knowledge Center base para workspaces, journals y artifacts.
- NovaCore chat base con degradacion cuando falta provider key.
- SSE base consumido desde `apps/web/src/lib/sse.ts`.
- Endpoints de dashboard consolidados con smoke:
  `scripts/smoke-battos-web.ps1 -RequireDatabase -CheckSSE` valida `/status`,
  Work Board, agents/skills, runtimes/providers, runs, repositories,
  Knowledge, Memory, NovaCore, Usage y primer evento SSE `system.metrics`.
- API expone `GET /agents` y `GET /skills` para que el dashboard no dependa de
  rutas inexistentes.
- Acceso local del dashboard mejorado: intenta cargar sin token y solo muestra
  prompt si la API responde HTTP 401.
- Knowledge Center dashboard alineado con API real: guardar memoria envia
  `title`, journals crean/usan workspace por proyecto y artifacts usan kinds
  validos (`markdown`, `image`, `link`, `diff`, `build_report`).
- Build web verificado: `npm run build` pasa fuera del sandbox; dentro del
  sandbox de Codex falla por `spawn EPERM` al ejecutar TypeScript, no por error
  de codigo.
- Lint web consolidado: `npm run lint` termina con `0 errors`; quedan warnings
  conocidos por tipos `any`, imports no usados y dependencias de hooks.
- Frontera frontend/API consolidada: el cliente web normaliza respuestas JSON
  `snake_case` a `camelCase`, incluyendo SSE, para evitar filtros vacios o
  detalles rotos cuando Go responde con campos como `project_id`, `goal_id` o
  `runtime_adapter_id`.
- Healthcheck de base de datos corregido: si SQLite no responde, `/status`
  reporta `database: down` y `overall: down`.
- Smoke web valida `/status`, Memory Core, SSE, shell web y endpoints del
  dashboard contra la base SQLite local.
- Estados degradados visibles iniciados: el shell del dashboard muestra un
  banner transversal cuando `/status` reporta `overall != ok` u ocurre offline,
  con detalle del subsistema `database`; Work Board muestra un panel especifico
  si SQLite no esta disponible y bloquea acciones de creacion hasta recuperar DB.
- Estados degradados extendidos: Control Room muestra que runs/approvals/logs
  dependen de la base SQLite local y bloquea proponer runs si la DB cae;
  Knowledge Center y NovaCore pausan operaciones cuando no pueden cargar datos
  o responder por DB/provider.
- SSE del dashboard endurecido: `connectSSE` usa `fetch` con Bearer opcional,
  reintenta con backoff, conserva el ultimo `id:` recibido y reabre streams con
  `Last-Event-ID`/`after` para evitar duplicar logs tras cortes breves.
- Hardening frontend consolidado: se removieron imports/estado muertos, se
  reemplazaron varios `any` por tipos `unknown`/DTOs locales, se estabilizaron
  hooks con `useCallback` y `npm run lint` queda en 0 errores / 0 warnings.
  `npm run build` de Next tambien pasa fuera del sandbox de Codex; dentro del
  sandbox sigue apareciendo `spawn EPERM` por politica de procesos de Windows.
- Usage & Limits ahora tiene vista propia en el dashboard: muestra costo
  mensual, tokens input/output/cache, requests y distribucion por
  proyecto/agente/proveedor/modelo usando `/usage/overview`; si SQLite esta
  caido muestra un estado degradado especifico y conserva el shell operativo.
- Usage & Limits queda consolidado como base de presupuestos: `/usage/overview`
  ahora devuelve nombre de proyecto, `project_monthly_budget_usd` y
  `cost_precision`; la vista muestra alertas por umbral, cards de budget por
  proyecto y fallback local cuando el proyecto no tiene budget configurado.
- Contrato OpenAPI de Usage alineado con la API real: `GET /usage/overview`
  devuelve una lista de `UsageOverviewItem`, no un objeto simple.
- Tipos OpenAPI generados para dashboard: `apps/web` incorpora
  `openapi-typescript`, scripts `generate:api-types` y `check:api-types`, y
  genera `src/lib/generated/openapi.ts` desde
  `packages/openapi/openapi.yaml`. Se agrego `api-contract.ts` como puente
  camelCase para migrar componentes sin romper la normalizacion actual.
- Contrato de runs corregido: `RunProposal.runtime_adapter_id` ahora incluye
  `sandbox-smoke` y `sandbox-memory-smoke`, ademas de `claude-code` y `codex`,
  para que OpenAPI represente los smokes reales de DockerSandbox/Memory Bridge.
- Settings base implementado: muestra URL de API, estado del token local sin
  revelar secretos, salud de API/config/database/memory/sysmetrics, modo
  degradado de SQLite/database y guardrails v0.1 para ejecucion supervisada.
- Settings auth/token local mejorado: permite guardar o reemplazar
  `BATTOS_API_TOKEN` en `localStorage`, limpiar el token y refrescar el estado
  sin revelar el valor. Verificado con `npm run lint`, `npm run build` y
  navegador local sin errores de consola.
- Smoke dashboard completo revalidado el 4 de junio de 2026 con la DB anterior; el
  7 de junio de 2026 fue revalidado contra SQLite local con
  `scripts/smoke-battos-web.ps1 -RequireDatabase -CheckSSE`, cubriendo API,
  Memory Core, endpoints del dashboard y SSE `system.metrics`.
- Smoke de Control Room SSE agregado y verificado: `scripts/smoke-battos-web.ps1
  -RequireDatabase -CheckSSE -CheckRunSSE -CheckWeb` valida tambien
  `/events/runs/{id}` sobre un run real. El 4 de junio de 2026 paso contra
  run `3190f1f0-95ff-449f-9438-15bfd3c68676`, confirmando evento
  `run.snapshot`.
- Consolidacion OpenAPI/dashboard revalidada el 4 de junio de 2026 y de nuevo
  el 7 de junio de 2026:
  `npm run generate:api-types`, `npm run check:api-types`, `npm run lint` y
  `npm run build` pasan; el build debe ejecutarse fuera del sandbox de Codex en
  Windows por `spawn EPERM`. El smoke completo
  `scripts/smoke-battos-web.ps1 -RequireDatabase -CheckSSE` paso contra SQLite
  local en `localhost:8020`.
- Mutaciones tipadas revalidadas el 4 de junio de 2026: `npm run lint`,
  `npm run check:api-types`, `npm run build` y smoke completo del dashboard
  volvieron a pasar tras migrar POST/PATCH principales a `apiClient`.
- Contrato NovaCore revalidado el 4 de junio de 2026: `npm run
  generate:api-types`, `npm run check:api-types`, `npm run lint`, `npm run
  build` y smoke completo del dashboard pasaron tras modelar conversaciones y
  mensajes en OpenAPI.
- Agents Registry dashboard agregado: nueva pestaña en sidebar, lista agentes,
  crea agentes con runtime, muestra runtimes disponibles y conserva el
  guardrail de que seleccionar un runtime no autoriza ejecucion automatica.

Pendientes:

- ADR de frontend/version cerrado: `docs/adr/0018-dashboard-nextjs-16.md`.
- API client/tipos generados: tipos base implementados; helpers de lectura
  principales migrados a `apiClient` tipado por OpenAPI para status, projects,
  goals, tasks, agents, skills, runtime adapters, runs, run logs/diff y usage.
  El puente camelCase preserva acronimos internos (`USD`, `MB`, `KBps`) para
  evitar divergencias entre contratos generados y componentes existentes.
- Mutaciones criticas del dashboard migradas a helpers tipados: crear proyecto,
  goal y task, actualizar task, proponer run, aprobar/cancelar run, buscar y
  guardar memoria, crear workspace/journal/artifact y chat NovaCore. El helper
  `snakeizeBody` permite que la UI use camelCase mientras el API Go recibe
  `snake_case`; los enums tipados endurecen `approval.kind`, `task.status`,
  `memory.scope` y `artifact.kind`.
- NovaCore chat queda mas cubierto por OpenAPI: se agregaron schemas y rutas
  para `GET /novacore/conversations` y
  `GET /novacore/conversations/{id}/messages`; `NovaChat` usa helpers tipados
  para conversaciones, mensajes y chat.
- Auth/token local completo y flujo de renovacion/expiracion.
- Project detail.
- Task detail.
- Usage: budgets y alertas base implementados; faltan budgets configurables por
  agente/provider/modelo, persistencia de thresholds personalizados y precision
  `exact` cuando el provider entregue costos verificados.
- Settings: token local implementado; faltan edicion controlada de
  preferencias, providers por referencia y configuracion modular persistente.
- SSE: reconexion base implementada; smoke con DB local y run real valida
  `run.snapshot`; falta probar reconexion durante logs en ejecucion.
- Estados offline/degraded consistentes por pantalla: falta pulir el lenguaje
  visual final y agregar error boundaries.
- Cliente API generado y tipado compartido desde OpenAPI para endpoints que aun
  no estan modelados, especialmente repositories mutables.
- Adapter `gemini-cli`: siguiente candidato natural tras probar Codex/Claude
  reales, porque ya existe en runtime seed/config y provider `google`, pero
  aun falta adapter ejecutable, imagen runtime con CLI, env `GOOGLE_API_KEY`,
  tests y smoke real.
- Cursor/Antigravity: evaluar como companion/editor connector antes de tratarlos
  como runtime automatico. Solo entrarian como adapters ejecutables si ofrecen
  una CLI/API headless segura, no interactiva y auditable.

Testing/debug:

- API offline.
- Token invalido.
- DB degradada.
- Empty states.
- Error boundaries.
- Responsive.
- SSE reconnection con `Last-Event-ID` y `after`.
- E2E basico con Playwright o equivalente.

## 10. Usage Y Budgets

Pendientes:

- Persistir usage events por provider/model/runtime.
- Tokens input/output.
- Costo reportado o estimado.
- Campo de precision: `exact`, `estimated`, `not_reported`.
- Budgets por proyecto/agente.
- Alertas de uso.

Testing/debug:

- Adapter reporta tokens.
- Adapter no reporta tokens.
- Costos estimados no se muestran como exactos.
- Budget excedido genera alerta, no crash.
- `/usage/overview` con costos cero reporta `not_reported`; con costo positivo
  reporta `estimated` hasta integrar costos exactos de provider.

## 11. Hardening Y Release

Pendientes:

- Auth single-owner estable.
- Secrets por referencia.
- Audit log.
- Backups de SQLite y filesystem gestionado.
- `battos doctor`.
- Guia de instalacion local/VPS.
- Smoke test final.
- Tag `v0.1.0`.

Smoke test final:

```powershell
battos status
battos project create demo --name "Demo"
battos task create --title "Idea suelta"
battos task assign <task_id> demo
battos task create --project demo --title "Crear landing"
battos memory save --title "Decision" --content "..."
battos runtime detect
battos run create ...
battos run logs ...
battos repo diff ...
```

## Prioridad Recomendada

1. Consolidacion dev actual.
2. Knowledge Center.
3. Work Board avanzado e Inbox.
4. Agents Registry.
5. Runtime detection.
6. Execution engine.
7. Memory Bridge.
8. Repositories/Git.
9. NovaCore.
10. Dashboard.
11. Usage/budgets.
12. Hardening/release.

La prioridad evita construir un dashboard visualmente atractivo sobre una base
fragil. Primero se estabiliza la informacion y las acciones; despues se
construye la cabina de control.
