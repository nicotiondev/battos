# ADR-0022: Allowlist de egress por proxy para runs host_session

- **Status**: Accepted
- **Fecha**: 2026-06-08
- **Decidido por**: Nico + Claude Code

## Context

ADR-0020 habilito el modo `host_session`: montar la sesion OAuth del usuario
(`~/.codex` / `~/.claude`) read-only dentro del contenedor del run, para usar la
suscripcion en vez de una API key. La revision de seguridad encontro el riesgo
critico **C1**: cuando se aprueba `network`, el contenedor recibe internet
completo (`--network bridge`). Con la sesion montada y el CLI corriendo en modo
bypass, un prompt malicioso puede **leer el token de sesion y exfiltrarlo** a
cualquier servidor. ADR-0020 nombro la allowlist de egress como la mitigacion
central pero la dejo pendiente.

El obstaculo: **Docker no filtra por dominio**. `--network none|bridge` es
todo-o-nada. Filtrar por IP (iptables) es fragil porque los providers usan CDNs
con IPs cambiantes.

## Decision

Implementar una **allowlist de egress por dominio** mediante un **proxy de
egress** sobre una **red Docker interna dedicada**:

```
[contenedor del run]  --unica salida-->  [egress proxy]  --allowlist-->  internet
  red interna (sin ruta a internet)        unico peer con internet,
  HTTPS_PROXY=proxy:PORT                    filtra por hostname (CONNECT/Host)
```

Decisiones concretas (acordadas):

1. **Proxy propio en Go** (no tinyproxy/squid): un proxy HTTP/CONNECT de ~100
   lineas, sin imagen externa nueva, que encaja en el stack y es unit-testeable.
   Filtra por **hostname** del CONNECT (HTTPS) y del `Host` (HTTP) — no
   desencripta TLS, solo decide tunelar o rechazar por destino.
2. **Servicio long-lived** en Docker Compose (`battos-egress-proxy`) sobre una red
   dedicada, reusado por todos los runs. Es stateless.
3. **Atado a `host_session`**: el filtro se fuerza cuando el run monta
   credenciales del host. Los runs con API key pueden usar la red normal (el
   filtro es opcional ahi, porque no hay sesion de larga vida expuesta).
4. **La enforcement real es la red, no el env**: el run corre en una red
   **interna** (sin ruta a internet), asi que aunque un script ignore
   `HTTPS_PROXY` e intente conexion directa, **falla por falta de ruta**. El
   proxy es el unico camino, y filtra. El env solo hace que el CLI lo use.
5. **Modo `log_only` para descubrir dominios**: arranca permitiendo todo pero
   **logueando** cada destino (`WOULD BLOCK host`), para descubrir el set real de
   endpoints que codex/claude tocan (API + auth + telemetria) antes de bloquear
   duro. Una vez conocido el set minimo, se pasa a `enforce`.
6. **Allowlist conservadora inicial**: `api.openai.com`, `api.anthropic.com` y
   sus endpoints de auth conocidos; se ajusta con lo que revele `log_only`.
   Config en `execution.egress_allowlist` + `execution.egress_mode`.

## Consequences

### Positivas

- Cierra C1: aunque el agente lea el token, **no tiene a donde mandarlo** salvo
  los dominios permitidos del provider.
- Reusa el stack (Go), sin imagen/dep externa.
- El modo log-only evita romper los CLIs por una allowlist incompleta.

### Negativas

- Infra extra: un servicio proxy, una red dedicada y wiring en DockerSandbox.
- La allowlist de dominios requiere descubrimiento real (log_only) y
  mantenimiento cuando los CLIs cambien de endpoints.
- Filtrado por hostname del CONNECT: confia en el SNI/host declarado; un cliente
  que mienta el host destino y luego haga otra cosa post-CONNECT es un vector
  teorico, mitigado porque el proxy solo abre el tunel al host declarado.

## Alternatives considered

- **tinyproxy/squid off-the-shelf**: menos codigo propio, pero suma imagen y
  config externa; rechazado por simplicidad/stack.
- **iptables por IP en una red bridge dedicada**: fragil (IPs de CDN cambian);
  rechazado.
- **No hacerlo (estado actual)**: deja host_session usable solo en maquina
  dedicada single-user con prompts confiables (ya documentado en ADR-0020).

## Implementation notes (por etapas)

- **A. Proxy Go** (`apps/api/internal/egress` + `cmd/egress-proxy`): HTTP/CONNECT,
  allowlist por hostname (exacta + sufijo de dominio), modos `enforce`/`log_only`,
  unit tests. Self-contained.
- **B. DockerSandbox**: para runs con mounts host_session + network, correr en la
  red interna dedicada y setear `HTTPS_PROXY`/`HTTP_PROXY` al proxy, en vez de
  `--network bridge`.
- **C. Compose + config**: servicio `battos-egress-proxy`, red dedicada,
  `execution.egress_allowlist` / `egress_mode`, y un smoke que valide
  allow/deny.

## Seguridad operativa (IMPORTANTE)

- **`log_only` NO protege.** En `log_only` un intento de exfiltracion se tunelea
  igual (solo se loguea). Es un modo de **descubrimiento** de dominios, no de
  proteccion. Los runs `host_session` DEBEN correr el proxy en **`enforce`** —
  la Etapa B lo fuerza en codigo (no usa la red de host_session sin enforce).
- El proxy verificado (Etapa A): la allowlist no es bypasseable (matching
  anclado al borde de dominio, host chequeado == host dialed, sin TOCTOU), falla
  cerrado en casos ambiguos (FQDN con punto final), y `enforce`/`log_only` se
  comportan correctamente. Validado por review de seguridad sobre `75ee863`.
- **C1 CERRADO (Etapas B+C, `023ef23`, review aprobado):** los runs
  host_session+network corren en la red **interna** `battos-egress` (sin ruta
  directa a internet) y salen solo por el proxy en `enforce`. Aunque un script
  ignore `HTTPS_PROXY`, no tiene ruta directa -> falla cerrado. Si el proxy no
  esta configurado, el run se rechaza antes de tocar Docker. El token montado no
  puede exfiltrarse fuera de los dominios permitidos.
- **Disponibilidad (no seguridad):** las CLIs de agente son Node y `fetch`/undici
  NO honran `HTTPS_PROXY` automaticamente. Si la CLI lo ignora, el run **falla
  cerrado** (no llega al provider; no fuga). Para que host_session *funcione*,
  puede hacer falta config explicita de proxy en la CLI (`--proxy`,
  `NODE_USE_ENV_PROXY`, global agent). Pendiente de validar en el smoke real.

## Related

- ADR-0011 - ejecucion supervisada y aislamiento por contenedor.
- ADR-0020 - host_session OAuth (el riesgo C1 que esto mitiga).
- `apps/api/internal/worker/docker_sandbox.go`.
