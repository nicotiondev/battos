# Plan FINAL — BattOS v1.0 (consolidado)

> Guardado el 2026-06-12. Fuente: `C:\Users\nicoa\.claude\plans\la-idea-es-que-snoopy-truffle.md`.

## Context

Este es el plan final y **finito** que consolida todo lo conversado: convertir a
BattOS en un **control plane self-hosted de orquestación multi-agente para coding**
(ADR-0024) — corre un *equipo* de agentes (Claude Code, Codex, Gemini, Pi, Hermes,
OpenClaw) como jobs gobernados, en paralelo, con tus suscripciones, sandbox/tiers/
approvals/dashboard/memoria. Reemplaza al plan anterior (Fases A→D) plegando las
decisiones nuevas: **runtimes reales** (Gemini/Pi + smoke Claude), **detectar+instalar
CLIs con 1 clic gobernado**, **memoria via Engram detrás de `MemoryProvider`**
(ADR-0025), **Nova orquestador + patrones Gentle-AI** (SDD/Judgment-Day/SKILL.md), y
**distribución como binario único** con dashboard embebido (curl|sh/brew).

Son **6 etapas concretas**. El final (Etapa 6, tag v1.0) cierra el proyecto; lo demás
queda como **updates post-v1** (sección al final). Disciplina: dogfood temprano — el
**team-run real de la Etapa 3** es la prueba de fuego.

---

## Estado de etapas (al 2026-06-12) — **v1.0.0 — 2026-06-12**

| Etapa | Objetivo | Estado |
|-------|----------|--------|
| 1 | Runtimes reales (Gemini/Pi/Claude smoke) | ✅ Completada |
| 2 | Detectar + instalar CLIs + monitor | ✅ Completada |
| 3 | Multi-agente: delegación + RunPool + sesiones→memoria | ✅ Completada |
| 4 | Memoria via Engram (ADR-0025) | ✅ Completada |
| 5 | Nova orquestador + patrones Gentle-AI | ✅ Completada |
| 6 | Distribución (binario único) + cierre v1.0 | ✅ Completada |

---

## Estado actual (base — ya hecho y verde)

- **Fase A ✅** trust tiers `sandbox|direct|connected` por run (`execution_mode` +
  `Worker.SandboxFor`), `DirectSandbox` (host), `ConnectedSandbox` (Hermes/OpenClaw
  por config), broker `credstore` (AES-GCM + fallback env), gate de approval
  `execution_mode`, egress proxy, host_session. **Codex validado en vivo.** UI: tier
  selector + `RuntimesPanel`.
- **Fase B parcial:** mailbox `agent_messages` (store) ✅, endpoints send/inbox/read ✅,
  **Team-MCP tools** `team_send_message`/`team_read_inbox`/`team_mark_read` ✅.
- **Ya existe (reusar):** detección de CLIs por `exec.LookPath` + versión
  (`handlers/runtimes.go`, `DetectRuntimeAdapters`, `UpsertCLIToolDetection`,
  `config/cli-tools.yaml`, tabla `cli_tools`); métricas CPU/MEM/NET por SSE
  (`sysmetrics/sysmetrics.go`, `packages/core/types.go` `SystemMetrics`,
  `/events/system-metrics`); seam de memoria (`MemoryContextProvider` en `worker.go`,
  `memory/core.go` Save/Search/Recent/Stats/Context, `handlers/memory.go`);
  `battos mcp install` (Claude Code `.mcp.json` + Codex `config.toml`).

---

## Etapa 1 — Runtimes reales (cierra "asigná una tarea y la corre cualquier CLI")

Objetivo: Claude/Codex/Gemini/Pi todos ejecutables y verificados.

- **Adapters nuevos** en `apps/api/internal/worker/adapters.go` (patrón `CommandAdapter`,
  como `codex`/`claude-code`): `gemini` (`ProviderEnv: GEMINI_API_KEY`, comando
  `gemini`) y `pi` (harness `-p`/JSON; `earendil-works/pi`). Registrar en
  `ApprovedAdapters` + en `approvedRuntimeTools` (`handlers/runtimes.go:92`) + seed en
  `sqlite_schema.sql` (`agent_runtimes`) y `config/cli-tools.yaml`.
- **Smoke real de Claude Code host_session** (replicando el de codex: mount
  `~/.claude`, egress, run real con suscripción). Script en `scripts/`.
- **Verificación:** un run real `succeeded` por cada runtime disponible en la máquina
  (gemini/pi con API key o host; claude por suscripción). `go test ./...` verde.

## Etapa 2 — Detectar + instalar CLIs con 1 clic (gobernado) + monitor completo

Objetivo: el flujo "detecta → si falta, instalá con 1 clic → corre" que pediste.

- **Registro con install:** agregar `install_command` / `install_url` por CLI en
  `config/cli-tools.yaml` + columnas en `cli_tools` (claude/codex/gemini/opencode/
  openclaw/hermes, con su comando oficial: npm/brew/curl|sh/go install).
- **Endpoint de instalación gobernado:** `POST /cli-tools/{id}/install` que ejecuta el
  `install_command` en el host **como mutación que pasa por approval** (mismo modelo
  que host_session/execution_mode — un `run_approvals` kind nuevo o un approval
  dedicado). Reusa el patrón de ejecución de comandos del worker + `redactKnownSecrets`.
- **Métricas completas:** sumar **disco** y **lista de procesos** a `SystemMetrics`
  (`sysmetrics.go` + `packages/core/types.go`); ya hay CPU/MEM/NET.
- **Dashboard:** en `RuntimesPanel`/`DashboardView` mostrar detectado/no-detectado con
  path+versión y botón **"Instalar"** (dispara el endpoint, pide approval); **widget de
  métricas de sistema** (consume el SSE existente).
- **Verificación:** desde el dashboard, ver el estado real de cada CLI, instalar uno
  faltante con aprobación y que quede detectado; widget de RAM/disco/procesos en vivo.

## Etapa 3 — Multi-agente: delegación + paralelo + sesiones→memoria (cierra Fase B)

Objetivo: un *equipo* trabajando junto — la diferenciación. **Prueba de fuego.**

- **B1d — tools de delegación** (`apps/cli/internal/commands/mcp.go` + métodos de
  `client.Client` reusando `/runs` y `/tasks`): `team_spawn_run` (un lead crea un
  child-run; `parent_run_id` en `runs.metadata`), `team_read_board` (lista tasks),
  `team_get_run_status` (poll del run delegado).
- **B2 — `RunPool`** en `worker.go`: N goroutines concurrentes; el claim atómico
  (`ClaimNextQueuedRun`) garantiza exactly-once. Config `worker_concurrency`. Split
  reader/writer SQLite = nota TODO (WAL+busy_timeout alcanza para v1).
- **B3 — sesiones→memoria:** al terminar un run, auto-promover su resumen a memoria
  (reusar el flujo `remember` de `handlers/runs.go`, HITL).
- **Team-run litmus:** un run líder (Claude) delega a Codex + Pi, se comunican por el
  mailbox, completan una tarea real, las sesiones quedan en memoria.
- **Verificación:** el team-run termina `succeeded` con ≥2 agentes intercomunicados;
  tests de `RunPool` (concurrencia exactly-once, `-race`) y de las tools verdes.

## Etapa 4 — Memoria via Engram (ADR-0025)

Objetivo: un cerebro (Engram) con sync multi-máquina, sin atarse al roadmap ajeno.

- **Interfaz `MemoryProvider`** (Save/Search/Recent/Stats/Context) sobre el seam que ya
  existe (`MemoryContextProvider` + `memory/core.go`). Dos impls, **una activa**:
  `BuiltinCore` (default/fallback offline) y `EngramProvider` (HTTP `:7437` / MCP).
- **Convención agente→Engram** (mapear agente a `session_id`/prefijo de `topic_key`),
  porque Engram no tiene `agent_id`. El dashboard sigue igual: la API **proxya** al
  provider activo. Config elige el provider.
- **A verificar antes:** que el HTTP de Engram cubra lo que `handlers/memory.go`
  consume (save/search/recent/stats con filtros project/scope).
- **Verificación:** correr con `EngramProvider`, guardar/buscar desde el dashboard y un
  run, confirmar persistencia; apagar Engram → cae a `BuiltinCore` sin romper.

## Etapa 5 — Nova orquestador + patrones Gentle-AI

Objetivo: le ordenás un objetivo a Nova y propone/lanza el team bajo tu aprobación.

- **Nova orquestador** (`handlers/novacore.go`): tools para leer el board, **proponer
  runs** (runtime/tier/prompt) y —con tu approval— **lanzarlos** (single o team) por
  los mismos gates. **Provider-agnóstico por API** (OpenRouter/Minimax/OpenAI/… via
  `novacore.provider`+`base_url`+`credential_ref` del broker), independiente de las
  suscripciones de los ejecutores.
- **Patrones Gentle-AI nativos:** **SDD** como workflow sobre runs (fases design/
  implement/review, cada una con su runtime/tier/modelo); **Judgment-Day** = review
  adversarial multi-agente (jueces + fix-agent) reusando el team-run; **skills** =
  ingerir el formato **`SKILL.md`** (interop con skills de Claude/Gentle-AI).
- **Verificación:** das un objetivo a Nova → propone un plan multi-agente → lo aprobás
  → lo lanza → el resultado queda en memoria. Un workflow SDD de punta a punta.

## Etapa 6 — Distribución (binario único) + cierre v1.0

Objetivo: "instalás con un comando y abrís localhost".

- **Binario único:** embeber el build estático del dashboard Next.js dentro del binario
  Go (`embed`) y servirlo desde el server HTTP, de modo que `battos` levante API +
  worker + dashboard juntos. Un flag/sub-comando para arrancar todo.
- **Release + instalador:** **GoReleaser** (cross-compile Win/Linux/Mac) + **`curl|sh`**
  y **brew tap** (como Engram). Versionado.
- **IDE bridge (Fase C):** extender `battos mcp install` para sumar las **team tools** y
  más agentes (Cursor/VS Code), cerrando la continuidad IDE↔plataforma.
- **Cierre:** docs (`docs/10-roadmap.md`, README de install), `mem_session_summary`,
  **tag `v1.0.0`**.
- **Verificación:** en una máquina limpia, `curl … | sh` instala y `battos` abre el
  dashboard; un run real anda end-to-end.

---

## Post-v1 (updates siguientes, fuera de esta v1)

- **B4 — adapter ACP** (Gemini nativo; Claude/Codex via adapter) para mediación de
  tool-calls en vivo.
- **A4 v2 — proxy-inject (MITM)**: token que nunca entra al contenedor (CA propia).
- **Engram Cloud avanzado** (enroll/autosync gestionado desde BattOS).
- **Multi-usuario / Postgres** (hoy SQLite single-user).
- **Odysseus** como runtime connected (solo si hace falta; AGPL → solo HTTP arm's
  length, nunca vendorizar).

## Orquestación del build

- Etapas en **cascada** (cada una usa la anterior). Dentro de cada etapa, las piezas de
  archivos disjuntos pueden ir en paralelo, pero **las que tocan el worker/store van
  secuenciales directo en master** (los worktrees aislados forkearon de bases viejas en
  esta sesión — lección aprendida).
- Cada etapa: build/vet/test verde + un **smoke real** como gate de cierre, y commit(s)
  por pieza con mensajes `[Etapa N]`.

## Definición de "v1.0 logrado"

Desde el dashboard (o el IDE), en una instalación de un comando: ves el sistema y tus
CLIs (instalás los que falten con tu OK), asignás una tarea eligiendo agente + tier +
tu suscripción; varios agentes la trabajan en paralelo comunicándose; todo queda en la
memoria persistente (Engram, sync multi-máquina); y Nova puede orquestar el equipo bajo
tu aprobación — con sandbox/egress/approvals en los tres tiers.
