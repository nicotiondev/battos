# BattOS

> **Mission Control agentic self-hosted** para administrar proyectos, agentes,
> skills, memoria, conocimiento, modelos y ejecuciones desde dashboard y CLI.

BattOS no reemplaza Linux, Docker, GitHub, Claude Code, Codex, Obsidian ni
n8n. Los orquesta con contexto, permisos, persistencia y auditoria.

```text
Linux administra la maquina.
Docker aisla los runs.
Git conserva los cambios.
BattOS administra trabajo, agentes, memoria, ejecucion y aprobaciones.
```

## Estado actual

**v0.1.0 - en construccion.** Las Fases 0, 1, 2, 3A, 3B, 4A, 4B, 4C, 4D y
5A tienen base implementada. La Fase 5B esta en curso con dashboard Next.js,
usage base, SSE y smokes locales consolidados. Queda correr smokes reales con
credenciales de providers, hardening y release.

Implementado actualmente:

- API Go con `GET /health`, `GET /version` y `GET /status`.
- CLI `battos status`.
- Schema PostgreSQL inicial y queries tipadas con sqlc.
- Memory Core propio (SQLite + FTS5) con HTTP y CLI: `recent`, `search`,
  `save`, `stats`.
- Contrato OpenAPI v0.1 y decisiones de autenticacion, secretos, runs y
  approvals.
- Middleware Bearer configurable y soporte CLI para `BATTOS_API_TOKEN`; en
  desarrollo sin token el API solo escucha en `127.0.0.1`.
- Fase 3B base: persistencia `sqlc`, API/CLI del Work Board para domains,
  projects, goals y tasks; API/CLI inicial del Knowledge Center para
  workspaces, journals y artifacts.
- Fase 4A base: deteccion segura de runtimes `claude-code` y `codex`, CLIs y
  providers sin ejecutar agentes ni exponer secretos.
- Fase 4B base: runs aprobables, worker, DockerSandbox, logs y artifacts de
  run; smokes DockerSandbox base y Memory Bridge verificados; smoke real de
  adapters preparado para `codex` y `claude-code`.
- Fase 4C base: repositorios locales gestionados, branch por run, diff,
  commit y push mediante approvals separados.
- Memory Bridge base: `battos memory context`, inyeccion de memoria de proyecto
  en runs y `remember` aprobable para guardar resumenes.
- NovaCore base: chat API/dashboard con contexto operacional del OS; requiere
  provider key real para responder.
- Dashboard base en `apps/web`: Command Center, Work Board, Control Room,
  Knowledge Center, Usage, Settings y NovaCore drawer conectados al API local.
  Settings permite guardar/reemplazar o limpiar `BATTOS_API_TOKEN` local sin
  revelar el valor.
- TUI interactiva `battos` / `battos shell` con welcome deck amplio,
  mascota pixel-art, navegacion por flechas, command palette `/`, footer fijo
  selector de idioma y panel de resultados para comandos.

En Docker/VPS se debe definir `BATTOS_API_TOKEN`; el compose habilita
`auth.mode: token` al publicar el API.

Objetivo final de **v0.1**:

- Modelo de trabajo: domains, projects, goals, tasks y board.
- Knowledge Center: journals, artefactos y previews administrados.
- Agentes y skills versionadas con adapters iniciales para Claude Code y
  Codex.
- Runs aprobados en contenedores efimeros, con logs, consumo, diff y
  artefactos.
- Repositorios Git locales gestionados o GitHub autorizado; commit y push con
  aprobaciones separadas.
- NovaCore opcional para conversar con el OS, organizar trabajo y proponer
  runs.
- Dashboard completo: Command Center, Work Board, Control Room y Knowledge
  Center.

Un ejemplo: creas un proyecto para un cliente, adjuntas un diseno, pides una
landing page, eliges un agente Claude Code o Codex y apruebas el run. BattOS
lo ejecuta en un contenedor, muestra logs y consumo, guarda la entrega y te
presenta el diff antes de autorizar commit o push.

Ver [producto final](docs/14-producto-final-y-roadmap.md) y
[roadmap operativo](docs/10-roadmap.md).

## Alcance posterior

| Version | Alcance principal |
|---|---|
| v0.2 | Extension Platform con manifests/rollback, export Markdown para Obsidian, Memory export/import, SDD y pull requests aprobados |
| v0.3 | Deployment connectors aprobados, mas adapters, Ollama/routing y metricas de valor |
| v0.4+ | Hermes/OpenClaw, Goal Mode limitado y automatizacion avanzada con guardrails |

No entra en v0.1: deploy automatico, ejecucion arbitraria sobre el host,
sincronizacion bidireccional con Obsidian, autonomia indefinida ni instalacion
general de plugins.

## Persistencia y seguridad

| Necesidad | Solucion |
|---|---|
| Recursos, runs, approvals, usage y auditoria | PostgreSQL 16 |
| Memoria persistente buscable | SQLite + FTS5, Memory Core propio |
| Repositorios, journals, artefactos y snapshots | Filesystem gestionado |
| Historial entregable del codigo | Git/GitHub con aprobacion |
| Lectura humana en Obsidian | Export Markdown opcional desde v0.2 |

Los runs solo se abren mediante adapters aprobados (`claude-code` y `codex`
en v0.1), dentro de un contenedor efimero. La red esta apagada por defecto y
su activacion queda visible y auditada. Secretos no se imprimen ni se guardan
como texto plano; commit y push requieren confirmaciones independientes.

## Stack

| Capa | Tecnologia |
|---|---|
| API, worker y CLI | Go |
| Router / config / CLI | chi, viper, cobra |
| DB principal / migraciones | PostgreSQL 16, sqlc, goose |
| Memory Core | SQLite + FTS5 (`modernc.org/sqlite`) |
| Knowledge artifacts | Filesystem gestionado en `data/artifacts` |
| Streaming | SSE |
| Contratos | OpenAPI + oapi-codegen |
| Frontend | Next.js 16 + TypeScript + shadcn/ui + Tremor |
| Aislamiento de runs | Docker container por ejecucion |

## Quickstart actual

```powershell
# Terminal 1: API; Memory Core funciona aunque Postgres no este configurado.
go run ./apps/api/cmd/api

# Terminal 2: estado y memoria
go run ./apps/cli/cmd/battos status
go run ./apps/cli/cmd/battos memory stats
go run ./apps/cli/cmd/battos project list

# Verificacion disponible
go test ./apps/api/... ./apps/cli/... ./packages/core/...
```

### Firma local de desarrollo en Windows

Si Windows bloquea `battos.exe` por control de aplicaciones tras recompilarlo,
puedes firmar el binario con un certificado local de desarrollo. Por defecto,
el script confia el certificado solo como `TrustedPublisher` del usuario actual
y exporta el certificado publico a `data/certs/battos-dev-code-signing.cer`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\sign-battos-dev.ps1
```

Para confiar el mismo certificado publico en otro Windows:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\trust-battos-dev-cert.ps1 `
  -CertificatePath .\data\certs\battos-dev-code-signing.cer
```

Esto no reemplaza una firma comercial para distribuir BattOS; solo deja el
binario local firmado y confiado en la maquina de desarrollo. Algunas politicas
de Windows App Control pueden exigir tambien confianza de raiz (`-TrustRoot`) o
un certificado comercial/empresarial.

## CLI disponible

La terminal usa un `ASCII wordmark` propio de BattOS, un bat-mark/mascota
original nocturno y una paleta negro/amarillo/gris como cabecera visual. La
TUI ocupa el ancho disponible para mostrar un welcome deck y, al ejecutar una
accion, captura la salida del comando en un panel de resultado para no volver a
la salida suelta de consola. En pantallas angostas cambia a layout compacto,
pero conserva la misma mascota pixel-art para que la identidad no salte entre
versiones visuales distintas.
Puedes usarla de dos formas: comandos directos o TUI interactiva.

```bash
battos
battos shell
battos status
battos memory recent
battos memory search "ficha"
battos memory save --title "..."
battos memory stats
battos domain create clientes --name "Clientes"
battos project create landing-acme --name "Landing Acme" --domain clientes
battos goal create --project landing-acme --title "Publicar landing"
battos task create --title "Idea suelta"
battos task create --project landing-acme --title "Preparar brief"
battos task list
battos task list --project landing-acme
battos task board --project landing-acme
battos task show <task_id>
battos task move <task_id> in_progress
battos task assign <task_id> landing-acme
battos task link-goal <task_id> <goal_id>
battos task position <task_id> 10
battos knowledge workspace create --project landing-acme --name "Landing Acme Knowledge"
battos knowledge workspace list
battos knowledge journal create --workspace <uuid> --title "Brief inicial" --content "Notas..."
battos knowledge artifact create --project landing-acme --name "Brief" --kind markdown --content "# Brief"
battos knowledge artifact create --project landing-acme --name "Wiki" --kind markdown --bucket wiki --content "# Documento curado"
battos knowledge artifact list --project landing-acme
battos runtime detect
battos runtime list
battos provider detect
battos cli-tool list
```

Runtime detection es inventario seguro: `configured` significa CLI + provider
presentes, pero `approved_for_execution` sigue en `false` hasta crear y aprobar
un run.

Para desarrollo local con Postgres en Docker, levanta el API con:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 -StopExisting -Background -Wait
```

Ese helper usa `DATABASE_URL` del entorno si existe; si no, usa el Postgres
local de desarrollo (`battos/change-me` en `127.0.0.1:5432`).
En Windows este es el launcher dev oficial del API; ver
`docs/adr/0015-windows-dev-api-launcher.md`.

Para validar el entorno dev:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-dev.ps1 -RequireDatabase
```

Para validar el worker con DockerSandbox real:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-docker-run.ps1
```

Este smoke requiere API y Postgres activos, Docker Desktop/daemon corriendo y
migraciones al dia. Crea un run `sandbox-smoke`, lo aprueba, lo procesa en un
contenedor sin red y valida logs, artifact y limpieza de workspace.
Si tienes el servicio Compose `battos-worker` corriendo, detenlo antes del
smoke local para que no reclame la cola en modo `dry_run`:

```powershell
docker compose -f infra/docker-compose.yml --env-file infra/.env stop battos-worker
```

El worker one-shot usado por los smokes procesa el run exacto con
`-run-id <uuid>` para evitar carreras con actividad paralela.

Para validar la integracion dashboard/API:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-web.ps1 -RequireDatabase -CheckSSE -CheckRunSSE -CheckWeb
```

Este smoke revisa los endpoints que consume `apps/web`: status, Work Board,
agents/skills, runtimes/providers, runs, repositories, Knowledge, Memory,
NovaCore, Usage con budgets base, Settings base y el primer evento SSE de
metricas. Con `-CheckRunSSE` tambien valida el stream de Control Room
`/events/runs/{id}` usando el run mas reciente disponible.

Para regenerar los tipos TypeScript del contrato OpenAPI del dashboard:

```powershell
cd apps\web
npm run generate:api-types
npm run check:api-types
```

`check:api-types` falla si `packages/openapi/openapi.yaml` y
`apps/web/src/lib/generated/openapi.ts` quedan desincronizados.

Si Postgres/Docker no esta levantado, puedes validar el modo degradado sin
exigir DB:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-web.ps1 -CheckSSE -CheckWeb
```

En ese modo el smoke valida `/status`, Memory Core, SSE y la shell web, y salta
los endpoints respaldados por Postgres cuando `database` esta `down`.

Para validar inyeccion de memoria en el sandbox:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-docker-run.ps1 `
  -DockerImage battos-runtime-agents:dev -RequireMemoryContext
```

Para construir la imagen runtime que contiene las CLIs aprobadas `codex` y
`claude`, usa:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-battos-runtime-agents.ps1
```

Esa imagen queda etiquetada como `battos-runtime-agents:dev` y se usa como
`BATTOS_EXECUTION_DOCKER_IMAGE` cuando el worker opera con `DockerSandbox`.
La imagen runtime es distinta a la imagen API/worker: el worker orquesta Docker;
el runtime efimero contiene las herramientas de agente.

Para validar un adapter real cuando tengas credenciales cargadas:

```powershell
# Requiere OPENAI_API_KEY
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-real-adapter-run.ps1 -Adapter codex

# Requiere ANTHROPIC_API_KEY
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-real-adapter-run.ps1 -Adapter claude-code
```

Para validar un adapter real dentro de `DockerSandbox`, con red aprobada para
hablar con el provider, usa el smoke dedicado. Requiere API/Postgres activos,
Docker Desktop/daemon, la imagen `battos-runtime-agents:dev` y la key del
provider en el entorno:

```powershell
$env:OPENAI_API_KEY = "..."
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-real-adapter-run.ps1 -Adapter codex

$env:ANTHROPIC_API_KEY = "..."
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-real-adapter-run.ps1 -Adapter claude-code
```

El script registra un agente de smoke, crea un run con `requested_network=true`,
aprueba `network` y `execute`, procesa la cola con el worker en modo Docker y
muestra los logs persistidos si el adapter falla.

Para dejar el worker procesando runs en cola:

```powershell
$env:DATABASE_URL = "postgresql://battos:change-me@127.0.0.1:5432/battos?sslmode=disable"
$env:BATTOS_EXECUTION_SANDBOX_MODE = "docker" # o dry_run
$env:BATTOS_EXECUTION_DOCKER_IMAGE = "battos-runtime-agents:dev"
go run ./apps/api/cmd/worker -once=false -poll 2s
```

En Docker Compose, el worker queda como servicio opcional. Por defecto corre en
`dry_run`, sin montar el socket de Docker:

```powershell
docker compose -f infra/docker-compose.yml --env-file infra/.env --profile worker up -d battos-worker
```

Para activar `DockerSandbox`, usa el override dedicado. Esto monta
`/var/run/docker.sock`, por lo que debe usarse solo en una maquina/VPS dedicada
a BattOS y manteniendo approvals humanos:

```powershell
docker compose -f infra/docker-compose.yml -f infra/docker-compose.worker-docker.yml --env-file infra/.env --profile worker up -d battos-worker
```

Si Windows App Control bloquea el `battos.exe` local despues de recompilar,
puedes validar usando el CLI via `go run` sin cambiar la politica de seguridad:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-dev.ps1 -RequireDatabase -UseGoRun
```

Dentro de `battos shell`, escribe `/` para abrir el menu inicial o usa atajos:

```text
↑/↓ navegar
Enter ejecutar
/ abrir command palette
Esc volver/cerrar palette
l cambiar idioma
Ctrl+C salida de emergencia
/status
/projects
/project-new
/tasks
/tasks landing-acme
/task-new
/memory
/language
/help
/exit
```

En modo TUI, `Esc` o `Enter` vuelven desde un resultado al Mission Control y
`Ctrl+C` sale de la interfaz. La TUI parte por defecto en espanol; puedes
cambiar a ingles desde `/language`, con la tecla `l`, usando `--lang en` o
definiendo `BATTOS_LANG=en`.

La TUI organiza Work Board como carpetas: entra a `/work-board` y luego a
`/domains`, `/projects`, `/goals` o `/tasks`. Dentro de cada carpeta puedes
listar o crear: por ejemplo `/project-new`, `/goal-new` y `/task-new`. Las
acciones `/goals` y `/tasks` listan todo por defecto; desde el modo slash
simple puedes usar `/goals <project>`, `/tasks <project>` o
`/task-board <project>` para filtrar. Crear objetivos pide `project id`;
crear tareas permite dejarlo vacio y usa `inbox` para capturar trabajo suelto
sin perder trazabilidad. En `/tasks` tambien puedes usar `/task-move`,
`/task-assign`, `/task-link-goal` y `/task-position`.

Work Board se organiza como una carpeta de trabajo: `domain` es el area,
cliente o unidad mayor; `project` vive dentro de un dominio; `goal` representa
un resultado esperado del proyecto; `task` es la accion concreta y puede
apuntar a un goal. Ejemplo: domain `clientes`, project `landing-acme`, goal
`Publicar landing`, tasks `Preparar brief` y `Conectar formulario`.

Knowledge Center ya permite crear un workspace por proyecto, guardar journals y
registrar artifacts por filesystem gestionado o por referencia externa. Los
buckets canonicos son `raw`, `wiki` y `outputs`: `raw` para briefs/referencias,
`wiki` para documentos curados y `outputs` para entregables. Todavia faltan
previews y la exportacion Markdown/Obsidian opcional.

La CLI de v0.1 agregara repositorios, adapters, creacion y aprobacion de runs,
logs y uso.

## Documentacion

| Doc | Contenido |
|---|---|
| [docs/14-producto-final-y-roadmap.md](docs/14-producto-final-y-roadmap.md) | Vision final, capacidades, persistencia, seguridad y versiones |
| [docs/10-roadmap.md](docs/10-roadmap.md) | Fases operativas para implementar v0.1 y posteriores |
| [docs/15-plan-de-objetivos.md](docs/15-plan-de-objetivos.md) | Pendientes perseguibles, criterios de cierre, testing y hardening |
| [packages/openapi/openapi.yaml](packages/openapi/openapi.yaml) | Contrato API fuente de verdad desde Fase 3A |
| [docs/01-architecture.md](docs/01-architecture.md) | Arquitectura por capas y flujo de ejecucion |
| [docs/03-data-model.md](docs/03-data-model.md) | Persistencia y tablas |
| [docs/05-memory-core.md](docs/05-memory-core.md) | Memory Core |
| [docs/11-agent-runtimes.md](docs/11-agent-runtimes.md) | Runtime adapters |
| [docs/12-novacore.md](docs/12-novacore.md) | Chat de administracion |
| [docs/13-comparativa-agent-os-sources.md](docs/13-comparativa-agent-os-sources.md) | Comparativa con fuentes investigadas |
| [docs/adr/0010-knowledge-workspace-opcional.md](docs/adr/0010-knowledge-workspace-opcional.md) | Obsidian/Markdown opcional |
| [docs/adr/0011-v01-ejecucion-supervisada.md](docs/adr/0011-v01-ejecucion-supervisada.md) | Ejecucion en v0.1 |
| [docs/adr/0012-extension-platform-modular.md](docs/adr/0012-extension-platform-modular.md) | Upgrades y extensiones |
| [docs/adr/0013-auth-y-secretos-v01.md](docs/adr/0013-auth-y-secretos-v01.md) | Token administrador y secretos por referencia |
| [docs/adr/0014-run-lifecycle-y-approvals.md](docs/adr/0014-run-lifecycle-y-approvals.md) | Estados de runs y aprobaciones |
| [docs/adr/0015-windows-dev-api-launcher.md](docs/adr/0015-windows-dev-api-launcher.md) | Launcher dev del API en Windows |
| [docs/adr/0018-dashboard-nextjs-16.md](docs/adr/0018-dashboard-nextjs-16.md) | Dashboard Next.js 16 |

## Licencia

TBD.
