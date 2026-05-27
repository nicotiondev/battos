# 04 - API Reference

> Fuente de verdad contractual desde Fase 3A:
> `packages/openapi/openapi.yaml`. Este documento explica cobertura y uso.
> Que un endpoint este contratado no significa que ya este implementado.

Base URL en dev: `http://localhost:8000`
Base URL en prod: configurable, normalmente detrás de Traefik/Nginx.

## Endpoints disponibles actualmente (Fases 1-2)

Los endpoints actuales todavia corren sin middleware Bearer. El contrato de
seguridad de ADR-0013 se implementara antes de exponer operaciones mutantes o
runs; `/health` y `/version` permaneceran publicos.

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

## Superficies contratadas para completar v0.1

| Endpoint | Fase | Descripción |
|---|---|---|
| `GET/POST /projects`, `/domains`, `/goals`, `/tasks` | 3B | Modelo de trabajo y board |
| `GET/POST /agents`, `/skills`, `/providers`, `/models` | 3B | Registries; skills versionadas |
| `GET/POST /knowledge/workspaces`, `/journals`, `/artifacts` | 3B | Knowledge Center canonico |
| `GET /runtime-adapters`, `POST /runtime-adapters/detect` | 4A | Adapters permitidos para Claude Code/Codex |
| `GET/POST /repositories` | 4C | Git local gestionado o GitHub autorizado |
| `POST /runs`, `POST /runs/{id}/approve`, `POST /runs/{id}/cancel` | 4B | Ejecucion supervisada |
| `POST /runs/{id}/network`, `/commit`, `/push` | 4B/4C | Aprobaciones y acciones auditadas |
| `GET /runs/{id}/logs`, `/artifacts`, `/diff` | 4B/4C | Resultado del run |
| `POST /novacore/chat` | 5A | Chat opcional; propone acciones/runs |
| `GET /usage/overview`, `GET /usage/runs/{id}` | 5B | Tokens/costo exacto, estimado o no reportado |
| `GET /events/system-metrics`, `/events/runs/{id}` (SSE) | 5B | Streams del dashboard |

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
```bash
battos status
battos memory stats
battos memory search "FTS5"
```

### Desde el frontend (Fase 5)
```typescript
// apps/web/lib/api-client.ts (generado por oapi-codegen en Fase 3)
import { getStatus } from "@/lib/api-client";
const status = await getStatus();
```
