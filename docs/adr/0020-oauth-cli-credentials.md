# ADR-0020: Ejecutar las CLIs de agente con su sesion OAuth, no con API key

- **Status**: Proposed
- **Fecha**: 2026-06-05
- **Decidido por**: Nico + Claude Code (pendiente de aprobacion)

## Context

El objetivo de producto de BattOS es invocar agentes corriendo **la CLI real**
de cada runtime (`codex`, `claude-code`, `opencode`, ...), no una
reimplementacion. Eso ya esta hecho: los adapters ejecutan `codex exec` y
`claude --bare --print` dentro del contenedor efimero
(`apps/api/internal/worker/adapters.go`).

Hoy esas CLIs se autentican con una **API key** inyectada por variable de
entorno (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`). Ese camino es **pago por
token**: descuenta de una cuenta de API separada de la suscripcion.

La intencion del usuario es distinta y mejor para su caso: que la CLI use la
**sesion OAuth/login que el usuario ya creo** (`codex login`, `claude login`),
es decir, su **suscripcion** (ChatGPT Pro / Claude Pro/Max). Asi el costo es el
plan fijo que ya paga, sin gasto extra por token.

El obstaculo es arquitectonico: la sesion OAuth vive en **archivos del host**
(`~/.codex/`, `~/.claude/`). Pero el run corre en un **contenedor efimero,
aislado y con la red apagada por defecto** (ADR-0011), que no tiene acceso ni a
esos archivos ni a la red del provider. Por diseno, hoy no puede usar el OAuth.

Hay una tension real con el modelo de seguridad: montar una sesion de larga
duracion dentro de un sandbox donde se ejecuta **codigo generado por un agente**
crea un vector de exfiltracion del token de sesion.

## Decision (propuesta)

Habilitar un modo de autenticacion **"sesion del host"** para CLIs oficiales,
**opt-in y por run**, con guardrails que acoten el riesgo:

1. **Adapter variante** (p. ej. `codex` con `auth_mode=host_session`) que **no**
   inyecta API key. En su lugar, el worker monta la carpeta de credenciales del
   CLI dentro del contenedor.
2. **Montaje read-only y minimo**: solo el path de credenciales necesario
   (`~/.codex`, `~/.claude`), nunca el home completo, y en modo `:ro`.
3. **Red restringida por allowlist**, no `bridge` abierto: el contenedor solo
   puede salir a los dominios del provider (`api.openai.com`,
   `api.anthropic.com`, endpoints de auth). Esta es la mitigacion central: aunque
   el agente leyera el token, no tiene a donde mandarlo.
4. **Solo CLIs oficiales** aprobadas (`codex`, `claude-code`). Clientes terceros
   no entran en este modo.
5. **Approval explicito**: exponer la sesion del host requiere una autorizacion
   consciente del usuario (flag/approval propio), separada de `execute` y
   `network`, y queda auditada (prompt, runtime, montaje, red, resultado).
6. **Redaccion**: cualquier token que aparezca en logs/stdout se redacta
   (mecanismo ya existente en `gitauth`/`docker_sandbox`).

Por defecto BattOS sigue en modo API key / `dry_run`; el modo sesion del host es
una eleccion explicita para una maquina/VPS dedicada del propio usuario.

## Consequences

### Positivas

- Cumple el objetivo de producto: usar la suscripcion ya pagada, sin costo por
  token.
- Reusa la CLI oficial tal como el usuario la usa en su terminal.
- La red por allowlist es un guardrail mas fuerte que el `bridge` abierto actual,
  util tambien para el modo API key.

### Negativas

- Introduce un vector de exfiltracion de la sesion: un prompt malicioso podria
  intentar leer las credenciales montadas. Se acota con read-only + allowlist de
  red, pero no se elimina.
- Acopla el run a credenciales del host: pierde algo de la pureza
  "todo efimero, nada del host".
- La allowlist de red por dominio requiere infra extra (proxy/egress filter o
  red Docker dedicada); `--network none/bridge` no alcanza para filtrar por
  dominio.
- Depende de formatos de credenciales y rutas de cada CLI, que pueden cambiar
  entre versiones.

## Alternatives considered

### Seguir solo con API key (estado actual)

- Mas simple y mejor aislado (ningun secreto de larga duracion en el sandbox).
- No cumple el objetivo del usuario y agrega costo por token.
- Queda como modo por defecto y fallback.

### Correr el agente en el host (sin contenedor) usando el OAuth directo

- Trivial de autenticar (usa la sesion tal cual).
- **Rechazada**: rompe el principio nuclear de BattOS de aislar la ejecucion;
  un agente con bug o prompt malicioso operaria sobre el host.

### Broker de credenciales / token efimero por run

- BattOS pediria un token de corta vida y lo inyectaria, sin montar la sesion
  completa.
- Ideal en seguridad, pero los providers de suscripcion no exponen hoy emision
  de tokens efimeros de delegacion para terceros. Queda como evolucion futura.

## Implementation notes

- Nuevo campo de adapter `AuthMode` (`api_key` | `host_session`) y variantes en
  `ApprovedDryRunAdapters`.
- `DockerSandbox`: soporte de mounts read-only adicionales y de una politica de
  red por allowlist (red Docker dedicada o proxy de egress), no solo
  `none`/`bridge`.
- Config: paths de credenciales por runtime y switch global
  (`execution.host_session_enabled=false` por defecto).
- Nuevo approval `host_session` (o extension del de `network`) con su registro
  de auditoria.
- Primer runtime objetivo: **Codex** (OAuth de ChatGPT ya validado en el proyecto
  Hermes Agent). Luego `claude-code`.
- Documentar claramente que este modo es para una maquina dedicada del usuario,
  no para multi-tenant.

## Related

- ADR-0011 - ejecucion supervisada y aislamiento por contenedor.
- ADR-0013 - autenticacion y secretos por referencia.
- ADR-0014 - ciclo de vida de runs y approvals.
- ADR-0019 - auth de push a GitHub por referencia de credencial.
- `apps/api/internal/worker/adapters.go`, `docker_sandbox.go`.
