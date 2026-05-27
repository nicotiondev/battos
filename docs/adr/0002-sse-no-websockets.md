# ADR-0002: SSE en vez de WebSockets para el streaming del dashboard

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

El Command Center necesita ~30 streams concurrentes para mostrar:
- CPU/MEM/NET en vivo (top bar).
- Sparklines de tokens por proyecto.
- Status de CLIs en tiempo real.
- Health gauge consolidado.
- Terminal tail.

Las opciones técnicas son **SSE** (Server-Sent Events), **WebSockets** o **polling HTTP**.

## Decision

**SSE para todos los streams del dashboard.**

Implementación: endpoints `GET /events/<tipo>` que devuelven `Content-Type: text/event-stream` y escriben mensajes cada N segundos hasta que el cliente cierra la conexión.

## Consequences

### Positivas

- **HTTP plano** — sin handshake adicional, sin librería cliente exótica. `EventSource` es estándar en todos los navegadores modernos.
- **Reconexión automática** del browser cuando se pierde la conexión.
- **Funciona con HTTP/2** sin configuración especial — multiplexa muchos streams sobre una sola conexión TCP.
- **Compatible con Traefik/Nginx/Cloudflare** sin tocar config (es solo HTTP largo).
- **Fácil de testear** con `curl -N` desde la línea de comandos.
- **Encaja con el patrón request-response** — un endpoint = un stream temático.

### Negativas

- **Solo server→client.** No hay canal de input streaming. Para v0.1 esto no es problema porque el dashboard solo consume datos.
- **Límite de conexiones por dominio** en algunos navegadores antiguos (6 por hostname). Mitigación: HTTP/2 multiplexea, así que en la práctica no se siente.
- **Sin compresión nativa** del payload — para muchos streams chicos es overhead vs WebSockets binarios.

### Neutrales

- En Go el patrón es trivial: tener un `http.ResponseWriter`, hacer `flusher, _ := w.(http.Flusher)` y escribir `event: x\ndata: {...}\n\n` periódicamente.
- El timeout del server para SSE debe ser 0 (no cortar) — el `http.Server` de Fase 1 ya está configurado así.

## Alternatives considered

### WebSockets
- ✅ Bidireccional, payload binario eficiente.
- ❌ Requiere librería cliente (`socket.io` o `ws` nativo) que complica el frontend.
- ❌ Más complejo de proxiear correctamente.
- ❌ No necesitamos bidireccional en el dashboard v0.1.

### Polling HTTP (sin streaming)
- ✅ Lo más simple posible — el frontend pregunta cada N segundos.
- ❌ Latencia alta (mínimo 1 ciclo de poll).
- ❌ Carga server alta con 30 endpoints siendo polled cada segundo desde muchas pestañas.
- ❌ Inconsistente visualmente (saltos en sparklines).

### Long-polling
- Solo lo elegiríamos si tuviéramos un proxy hostil que rompa SSE. Hoy no es el caso.

## Cuándo se reevalúa

Si v0.5 expone un terminal interactivo real (input desde el browser hacia un agente corriendo en el server), ahí sí WebSockets es la opción correcta. Mientras tanto, SSE.

## Implementation notes

- Endpoint `GET /events/system-metrics` se implementa en Fase 5 (no es necesario hasta que esté el frontend).
- Helper `server.SSEWriter(w)` que envuelve el `ResponseWriter` y expone `WriteEvent(name, payload)` — se agrega en Fase 5.
- El `http.Server` en `apps/api/cmd/api/main.go` ya está con `WriteTimeout: 0` para no cortar streams.

## Related

- `docs/04-api-reference.md` — lista de endpoints SSE planeados.
- `docs/07-frontend-architecture.md` (a crear en Fase 5) — hook `useSSE` del lado React.
