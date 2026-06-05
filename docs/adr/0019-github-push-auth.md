# ADR-0019: Autenticacion de clone/push a GitHub por referencia de credencial

- **Status**: Accepted
- **Fecha**: 2026-06-05
- **Decidido por**: Nico + Claude Code

## Context

La Fase 4C dejo operable el flujo Git supervisado (branch por run, diff,
approvals de `commit` y `push`), pero solo contra repositorios `managed_local`:
el worker clonaba siempre desde la carpeta local `data/repositories/<id>` y el
push usaba el remoto `origin` heredado de ese clon. La columna `credential_ref`
existia en la tabla `repositories` pero no se usaba en ninguna operacion Git, de
modo que un repositorio `kind=github` no se podia clonar ni pushear realmente.

ADR-0013 ya fijo el modelo: las credenciales de repositorio se guardan **por
referencia**, nunca como secreto en tablas, logs o artifacts. Faltaba cablear
esa referencia a las operaciones Git reales.

## Decision

Repos `kind=github` se autentican contra un remoto **https** usando el token
referenciado por `credential_ref`:

- `credential_ref` es el **nombre de una variable de entorno** del proceso
  (worker / API). El token vive en `infra/.env` o el shell, nunca en PostgreSQL.
- Un paquete dedicado `apps/api/internal/gitauth` centraliza la logica pura:
  - `Resolve(ref)` lee la env var referenciada.
  - `AuthenticatedURL(remoteURL, token)` inyecta `x-access-token:<token>@` en
    URLs `https`; deja intactas las formas `ssh`/scp y las URLs sin token.
  - `Redact(s, token)` reemplaza el token por `***` en cualquier salida.
- El **worker** clona el remoto autenticado para repos `github` y, tras clonar,
  restaura el remoto `origin` a la URL limpia para no persistir el token en el
  `.git/config` del workspace temporal.
- El **handler de approvals** vuelve a inyectar el token al vuelo en el `git
  push` (no usa `origin` para github), y redacta el token de cualquier error.
- Conectar un repo `github` exige `remote_url`. Aprobar `push` sobre un repo
  `github` cuyo `credential_ref` no resuelve un token devuelve `400` con un
  mensaje accionable, en vez de intentar un push anonimo que fallaria.

## Consequences

### Positivas

- Cierra el flujo end-to-end: ejecutar -> commit -> push a GitHub real.
- El secreto nunca toca la base de datos, los logs, los artifacts ni el
  `.git/config` persistido.
- La logica de auth es pura y testeable sin red (`gitauth_test.go`).

### Negativas

- El usuario debe definir la env var referenciada por `credential_ref` en el
  entorno del worker/API antes de aprobar push.
- Soporta `https` con token; auth por `ssh` queda implicita (llave del host) y
  sin gestion propia en v0.1.

## Alternatives considered

### Guardar el token en la tabla `repositories`

- Mas simple de cablear.
- Viola ADR-0013 (secretos por referencia) y aumenta el blast radius.

### Credential helper de Git / `.netrc`

- Estandar y reusable.
- Agrega estado en el host y archivos de credenciales a limpiar; la inyeccion al
  vuelo en URL https es mas acotada y efimera para el workspace temporal.

## Related

- ADR-0013 - autenticacion y secretos.
- ADR-0014 - ciclo de vida de runs y approvals.
- `internal/gitauth`, `internal/worker/worker.go`, `internal/handlers/runs.go`.
