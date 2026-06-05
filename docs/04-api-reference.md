# 04 - API Reference

> Fuente de verdad contractual desde Fase 3A:
> `packages/openapi/openapi.yaml`. Este documento explica cobertura y uso.
> Que un endpoint este contratado no significa que ya este implementado.

Base URL en dev: `http://localhost:8000`
Base URL en prod: configurable, normalmente detrás de Traefik/Nginx.

## Endpoints disponibles actualmente (Fases 1-3B parcial)

ADR-0013 ya esta implementado en la frontera actual: `/health` y `/version`
son publicos; `/status` y `/memory/*` pasan por middleware Bearer cuando
`auth.mode: token`. El modo local configurado usa `auth.mode: disabled` solo
escuchando en `127.0.0.1`.

### `GET /health`
Latido simple. Pensado para load balancers y healthchecks de Docker.

**No toca DB ni otros subsistemas** — siempre responde rápido.

Response 200:
```json
{
  "status": "ok",
  "timestamp": "2026-05-25T17:18:37Z"
}
```

### `GET /version`
Versión del binario y runtime.

Response 200:
```json
{
  "version": "v0.1.0-alpha",
  "commit": "dev",
  "build_date": "unknown",
  "go_version": "go1.25.x"
}
```

Los valores `version`, `commit`, `build_date` se inyectan en tiempo de build vía `-ldflags`. Ver `infra/Dockerfile.api`.

### `GET /status`
Estado agregado del OS — el endpoint principal del Command Center.

Devuelve:
- Versión (mismo payload que `/version`).
- `overall`: estado consolidado (`ok` | `degraded` | `down` | `unknown`).
- `subsystems`: lista de subsistemas con su salud individual.
- `metrics`: snapshot en vivo de CPU/MEM/NET del host.

Response 200:
```json
{
  "version": { "version": "v0.1.0-alpha", "commit": "dev", "build_date": "unknown", "go_version": "go1.25.x" },
  "overall": "ok",
  "subsystems": [
    { "name": "config", "status": "ok", "detail": "battos.yaml cargado" },
    { "name": "sysmetrics", "status": "ok" },
    { "name": "database", "status": "unknown", "detail": "no inicializado" },
    { "name": "memory", "status": "ok", "detail": "SQLite + FTS5 listo", "latency_ms": 0 }
  ],
  "metrics": {
    "cpu_percent": 54.1,
    "mem_percent": 69.0,
    "mem_used_mb": 22397,
    "mem_total_mb": 32015,
    "net_upload_kbps": 72.85,
    "net_download_kbps": 198.03
  },
  "timestamp": "2026-05-25T17:18:39Z"
}
```

**Regla de agregación de `overall`**:
- Si algún subsistema está `down` → overall = `down`.
- Si algún subsistema está `degraded` → overall = `degraded`.
- `unknown` (subsistema no implementado todavía) se ignora.

Si `DATABASE_URL` no está definido, `database` se informa como `unknown`; Memory Core funciona de forma independiente.

### Memory Core (Fase 2)

| Endpoint | Método | Descripción |
|---|---|---|
| `/memory/recent?limit=20` | GET | Últimas observaciones |
| `/memory/search` | POST | Búsqueda FTS5 y filtros; con `query` vacío lista aplicando filtros |
| `/memory/save` | POST | Inserta una observación o actualiza por `topic_key` |
| `/memory/stats` | GET | Contadores agregados |
| `/memory/{id}` | GET | Recupera una observación o responde 404 |

Ejemplo:
```json
POST /memory/search
{
  "query": "FTS5",
  "filter": { "project_id": "battos", "type": "decision" },
  "limit": 10
}
```

### Work Board (Fase 3B)

Estos recursos requieren PostgreSQL configurado con `DATABASE_URL`. Las
colecciones de goals y tasks pueden listarse globalmente o filtrarse por
`project_id`; la creacion todavia exige proyecto para mantener trazabilidad.

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/domains`, `/domains/{id}` | GET/POST/PATCH | Areas o clientes que agrupan proyectos |
| `/projects`, `/projects/{id}` | GET/POST/PATCH | Proyectos operables por BattOS |
| `/goals`, `/goals?project_id={id}`, `/goals/{id}` | GET/POST/PATCH | Objetivos globales o filtrados por proyecto |
| `/tasks`, `/tasks?project_id={id}`, `/tasks/{id}` | GET/POST/PATCH | Tareas globales o filtradas por proyecto; crear sin `project_id` usa `inbox` |

Defaults al crear: domain/project `active`, goal `planned`, task `backlog`.

### Knowledge Center (Fase 3B)

Estos recursos requieren PostgreSQL configurado con `DATABASE_URL`. En esta
primera superficie operable, BattOS guarda el indice canonico de conocimiento:
workspaces por proyecto, journals markdown y artifacts asociados a proyecto,
tarea o futuro run.

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/knowledge/workspaces` | GET/POST | Workspaces activos de conocimiento por proyecto |
| `/journals?project_id={id}` | GET | Journals de un proyecto |
| `/journals` | POST | Crea journal; puede inferir `project_id` desde `workspace_id` |
| `/artifacts?project_id={id}` | GET | Artifacts de un proyecto |
| `/artifacts` | POST | Crea artifact markdown, link, path gestionado, diff o build report |

Defaults al crear: workspace `layout=raw_wiki_outputs`, `status=active`; journal
usa la fecha actual si `journal_date` no viene informado. Artifacts requieren al
menos uno de `content`, `managed_path` o `external_url`.

Buckets canonicos de artifacts:

| Bucket | Uso |
|---|---|
| `raw` | Briefs, referencias, inputs originales |
| `wiki` | Documentos curados para lectura humana |
| `outputs` | Entregables generados por runs o agentes |

Si se crea un artifact con `content` y sin `external_url`, BattOS escribe el
contenido bajo `knowledge.artifacts_dir` (por defecto `data/artifacts`) usando
la forma `{project_id}/{bucket}/{timestamp}-{name}.md` y guarda esa ruta en
`managed_path`. Si se informa `managed_path`, debe ser relativo y no puede salir
de `artifacts_dir`.

### Runtime Detection (Fase 4A base)

Estos endpoints requieren PostgreSQL. Detectan inventario, no ejecutan agentes
ni prompts, y no conceden permisos de ejecucion.

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/runtime-adapters` | GET | Lista runtimes registrados y su estado |
| `/runtime-adapters/detect` | POST | Detecta `claude` y `codex` en PATH y actualiza estado |
| `/cli-tools` | GET | Lista CLIs detectadas por BattOS |
| `/providers` | GET | Lista providers sin exponer secretos |
| `/providers/detect` | POST | Marca providers `configured/not_configured` segun env vars |

Estados runtime: `configured`, `detected`, `unavailable`, `blocked`,
`disabled`. `configured` significa que la CLI fue detectada y la env var de su
provider esperado existe; no implica autorizacion de ejecucion. `blocked`
aparece cuando el binario existe pero Windows/App Control o el comando de
version impiden una lectura segura. Todas las respuestas de deteccion reportan
`approved_for_execution=false`; la autorizacion real se pide por run en Fase 4B.

### Runs Supervisados (Fase 4B control plane)

Estos endpoints requieren PostgreSQL. Proponen y auditan runs; la ejecucion la
realiza el worker aislado cuando el run queda `queued` y el worker esta
corriendo.

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/runs` | GET | Lista runs; acepta `project_id` opcional |
| `/runs` | POST | Propone un run en estado `awaiting_approval` |
| `/runs/{id}` | GET | Detalle del run |
| `/runs/{id}/approvals` | POST | Registra approval `execute`, `network`, `commit` o `push` |
| `/runs/{id}/cancel` | POST | Cancela runs no terminales |
| `/runs/{id}/logs` | GET | Lista logs persistidos del run |
| `/events/runs/{id}` | GET | SSE sin timeout con `run.snapshot`, `run.log`, `run.done` y `run.error` |

`execute/approved` mueve el run a `queued`; no ejecuta nada por si solo.
`network/approved` solo habilita `network_enabled=true` si el run habia
solicitado red. `commit` y `push` quedan auditados para la fase de repositorios.
El worker puede operar en `dry_run` o `docker`; `docker` usa red `none` salvo
approval `network`, captura stdout/stderr y limpia el workspace temporal. Los
adapters `codex` y `claude-code` preparan comandos no interactivos que leen el
prompt desde `BATTOS_PROMPT_FILE`; DockerSandbox solo pasa las env keys de
provider declaradas por el adapter y redacta valores conocidos en stdout/stderr.
La imagen recomendada para esos runs es `battos-runtime-agents:dev`, construida
desde `infra/Dockerfile.runtime-agents`.

Cuando un run en DockerSandbox produce archivos dentro de `/workspace`, BattOS
los captura como artifacts del run, los guarda bajo `data/artifacts` y registra
el indice en `/artifacts` con `project_id`, `task_id` y `run_id`. El prompt
interno `BATTOS_PROMPT.md` nunca se registra como artifact.

## Superficies contratadas para completar v0.1

| Endpoint | Fase | Descripción |
|---|---|---|
| `GET/POST /agents`, `/skills`, `/providers`, `/models` | 3B | Registries; skills versionadas |
| `GET/POST /repositories` | 4C | Git local gestionado o GitHub autorizado |
| `POST /runs`, `POST /runs/{id}/approvals`, `POST /runs/{id}/cancel` | 4B base | Control plane de ejecucion supervisada |
| `GET /runs/{id}/artifacts`, `/diff` | 4B/4C | Resultado del run cuando exista worker/repos |
| `POST /novacore/chat` | 5A | Chat opcional; propone acciones/runs |
| `GET /usage/overview`, `GET /usage/runs/{id}` | 5B | Tokens/costo exacto, estimado o no reportado |
| `GET /events/system-metrics` (SSE) | 5B | Stream de metricas del dashboard |

`packages/openapi/openapi.yaml` marca cada operacion futura con
`x-battos-phase`. Los clientes generados se incorporaran al implementar la
primera superficie CRUD de Fase 3B.

## Convenciones

### Formato de errores
Todos los errores devuelven JSON con esta forma:

```json
{
  "error": {
    "message": "endpoint no encontrado",
    "code": 404
  }
}
```

### Headers comunes
- `X-Request-Id`: aceptado o generado durante el request y registrado en los logs estructurados del API.
- `Content-Type: application/json; charset=utf-8` en respuestas exitosas.

### CORS
Orígenes permitidos vienen de `config/battos.yaml` (`api.cors_origins`). En dev por defecto es `http://localhost:3000` (el frontend Next.js).

## Middleware aplicado

Orden de ejecución (de afuera hacia adentro):

1. **RequestID** → genera `X-Request-Id`.
2. **RealIP** → respeta `X-Forwarded-For` si viene de proxy confiable.
3. **Recoverer** → convierte panics en 500 sin matar el proceso.
4. **CORS** → orígenes según config.
5. **StructuredLogger** → un JSON log por request con method/path/status/duration_ms/request_id.
6. **Timeout** → 30s (no aplica a sub-routers de SSE).

## Ejemplos de uso

### Con `curl`
```bash
curl http://localhost:8000/health
curl http://localhost:8000/version
curl http://localhost:8000/status | jq
curl http://localhost:8000/memory/stats | jq
```

### Con el CLI
```powershell
battos status
battos shell
battos memory stats
battos memory search "FTS5"
battos domain create clientes --name "Clientes"
battos project create landing-acme --name "Landing Acme" --domain clientes
battos task create --title "Idea suelta"
battos task create --project landing-acme --title "Preparar brief"
battos task list --project landing-acme
battos task board --project landing-acme
battos task position <task_id> 10
battos agent create builder-web --name "Builder Web" --runtime codex --role web_builder
battos agent list
battos agent show builder-web
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

# Dentro de battos shell:
↑/↓ navegar, Enter ejecutar, / command palette, Esc volver, q salir
/status
/projects
/tasks landing-acme
/task-board landing-acme
/agents
/agent-new

# Cuando auth.mode=token:
$env:BATTOS_API_TOKEN="<token>"; battos status
```

### Desde el frontend (Fase 5)
```typescript
// apps/web/lib/api-client.ts (generado por oapi-codegen en Fase 3)
import { getStatus } from "@/lib/api-client";
const status = await getStatus();
```
