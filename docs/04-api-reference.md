# 04 — API Reference

> Fuente de verdad provisoria: este doc. En Fase 3 se reemplaza por OpenAPI generado.

Base URL en dev: `http://localhost:8000`
Base URL en prod: configurable, normalmente detrás de Traefik/Nginx.

## Endpoints disponibles (v0.1)

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
  "go_version": "go1.26.3"
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
  "version": { "version": "v0.1.0-alpha", "commit": "dev", "build_date": "unknown", "go_version": "go1.26.3" },
  "overall": "ok",
  "subsystems": [
    { "name": "config", "status": "ok", "detail": "battos.yaml cargado" },
    { "name": "sysmetrics", "status": "ok" },
    { "name": "database", "status": "unknown", "detail": "Fase 2" },
    { "name": "memory", "status": "unknown", "detail": "Fase 2" }
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

## Endpoints planeados (Fase 2+)

| Endpoint | Fase | Descripción |
|---|---|---|
| `GET /projects`, `POST /projects`, ... | 3 | Project registry |
| `GET /agents`, `POST /agents`, ... | 3 | Agent registry |
| `GET /agent-runtimes` | 3 | Runtimes disponibles |
| `GET /skills`, `POST /skills` | 3 | Skill registry |
| `GET /providers` | 3 | Status de providers (configured/not_configured) |
| `GET /models` | 3 | Registry de modelos |
| `GET /cli/tools`, `POST /cli/detect` | 4 | CLI Manager |
| `GET /connections` (MCP) | 3 | MCP Registry |
| `GET /memory/recent`, `POST /memory/search`, `POST /memory/save` | 3 | Memory Core |
| `GET /usage/overview`, `GET /usage/providers/{id}` | 3 | Usage tracker (stub) |
| `GET /events/system-metrics` (SSE) | 5 | Stream CPU/MEM/NET para dashboard |
| `GET /events/logs` (SSE) | 5 | Stream del terminal del Command Center |

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
- `X-Request-Id`: agregado por middleware en todas las responses; útil para correlar con logs estructurados del API.
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
```

### Con el CLI
```bash
battos status
```

### Desde el frontend (Fase 5)
```typescript
// apps/web/lib/api-client.ts (generado por oapi-codegen en Fase 3)
import { getStatus } from "@/lib/api-client";
const status = await getStatus();
```
