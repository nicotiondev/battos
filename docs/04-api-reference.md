# 04 - API Reference

> Fuente de verdad contractual: `packages/openapi/openapi.yaml`. Este documento
> resume el uso actual para v0.1.

Base URL en dev: `http://localhost:8000`
Base URL en prod: configurable, normalmente detras de Traefik/Nginx.

## Boot Y Estado

### `GET /health`

Latido simple para healthchecks. No toca DB ni otros subsistemas.

```json
{ "status": "ok", "timestamp": "2026-06-07T12:00:00Z" }
```

### `GET /version`

Version del binario y runtime.

```json
{
  "version": "v0.1.0-alpha",
  "commit": "dev",
  "build_date": "unknown",
  "go_version": "go1.25.x"
}
```

### `GET /status`

Estado agregado del OS. Reporta `config`, `sysmetrics`, `database`, `memory` y
metricas host. La base operacional es SQLite local; si no puede abrirse o
responder, el subsistema `database` aparece `down`.

```json
{
  "overall": "ok",
  "subsystems": [
    { "name": "config", "status": "ok", "detail": "battos.yaml cargado" },
    { "name": "database", "status": "ok", "detail": "SQLite local conectado" },
    { "name": "memory", "status": "ok", "detail": "SQLite + FTS5 listo" }
  ]
}
```

## Memory Core

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/memory/recent?limit=20` | GET | Ultimas observaciones |
| `/memory/search` | POST | Busqueda FTS5 y filtros |
| `/memory/save` | POST | Inserta o actualiza por `topic_key` |
| `/memory/stats` | GET | Contadores agregados |
| `/memory/{id}` | GET | Recupera una observacion o 404 |

## Work Board

Todos estos recursos usan la base SQLite local.

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/domains`, `/domains/{id}` | GET/POST/PATCH | Areas o clientes |
| `/projects`, `/projects/{id}` | GET/POST/PATCH | Proyectos operables |
| `/goals`, `/goals?project_id={id}`, `/goals/{id}` | GET/POST/PATCH | Objetivos globales o filtrados |
| `/tasks`, `/tasks?project_id={id}`, `/tasks/{id}` | GET/POST/PATCH | Tareas globales, por proyecto o inbox |

## Registries Y Runtimes

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/agents`, `/agents/{id}` | GET/POST/PATCH | Agentes configurados |
| `/skills` | GET/POST | Skills versionadas |
| `/runtime-adapters` | GET | Runtimes registrados y estado |
| `/runtime-adapters/detect` | POST | Detecta CLIs aprobables en PATH |
| `/cli-tools` | GET | CLIs detectadas |
| `/providers` | GET | Providers sin exponer secretos |
| `/providers/detect` | POST | Marca providers segun env vars |

Detectar un runtime o provider no concede ejecucion. La ejecucion requiere un
run propuesto y approval humano.

## Knowledge Center

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/knowledge/workspaces` | GET/POST | Workspaces por proyecto |
| `/journals?project_id={id}` | GET | Journals de un proyecto |
| `/journals` | POST | Crea journal |
| `/artifacts?project_id={id}` | GET | Artifacts de un proyecto |
| `/artifacts` | POST | Crea artifact markdown, link, path gestionado, diff o build report |

Si se crea un artifact con `content` y sin `external_url`, BattOS escribe el
archivo bajo `knowledge.artifacts_dir` (`data/artifacts` por defecto).

## Runs Supervisados

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/runs` | GET | Lista runs; acepta `project_id` opcional |
| `/runs` | POST | Propone un run en `awaiting_approval` |
| `/runs/{id}` | GET | Detalle del run |
| `/runs/{id}/approvals` | POST | Registra approval `execute`, `network`, `commit`, `push` o `remember` |
| `/runs/{id}/cancel` | POST | Cancela runs no terminales |
| `/runs/{id}/logs` | GET | Lista logs persistidos |
| `/runs/{id}/artifacts` | GET | Lista artifacts asociados al run |
| `/events/runs/{id}` | GET | SSE con snapshots, logs y estado terminal |

`execute/approved` mueve el run a `queued`; el worker procesa la cola. El worker
puede operar en `dry_run` o DockerSandbox. DockerSandbox usa red `none` salvo
approval `network`, captura stdout/stderr y limpia el workspace temporal.

## Repositories, NovaCore Y Usage

| Endpoint | Metodo | Descripcion |
|---|---|---|
| `/repositories` | GET/POST | Repos Git locales gestionados o remotos autorizados |
| `/novacore/chat` | POST | Chat opcional que propone acciones/runs |
| `/usage/overview` | GET | Tokens/costo por proyecto, agente, provider o modelo |
| `/usage/runs/{id}` | GET | Uso asociado a un run |
| `/events/system-metrics` | GET | SSE de metricas del dashboard |

## Convenciones

### Errores

```json
{
  "error": {
    "message": "endpoint no encontrado",
    "code": 404
  }
}
```

### Headers

- `X-Request-Id`: aceptado o generado durante el request.
- `Content-Type: application/json; charset=utf-8`.

### Auth

`/health` y `/version` son publicos. Las demas rutas pasan por middleware
Bearer cuando `auth.mode: token`. En desarrollo local se usa
`auth.mode: disabled` escuchando en `127.0.0.1`.

## Ejemplos CLI

```powershell
battos status
battos project list
battos task create --title "Idea suelta"
battos memory stats
battos runtime list
battos run list
```
