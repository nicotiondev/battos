# ADR-0023 — Credential broker (Bóveda funcional) y `auth_mode` por runtime

- **Estado**: Propuesto (Fase A4 del plan "siguiente nivel")
- **Fecha**: 2026-06-11
- **Contexto del plan**: `la-idea-es-que-snoopy-truffle.md` → Fase A, paso A4.
- **Relacionado**: [ADR-0013] secretos por referencia · [ADR-0020] OAuth CLI / host_session · [ADR-0022] egress allowlist proxy.

---

## Contexto

Hoy BattOS maneja credenciales por **referencia** (ADR-0013): la DB guarda
`credential_ref`, que es el **nombre de una variable de entorno**; el token real
vive en el entorno del proceso (`infra/.env` o shell) y `gitauth.Resolve` hace
`os.Getenv(ref)`. Dos modos de auth conviven en los adapters:

- **`host_session`** (ADR-0020): se montan los directorios de credenciales del
  host (`~/.codex`, `~/.claude`) read-only en el contenedor; el agente usa su
  **suscripción**. No hay token plano en ninguna parte.
- **byok / default**: una API key referenciada por `ProviderEnv`
  (`OPENAI_API_KEY`, etc.) se pasa al contenedor con `docker -e KEY` leyendo del
  env del host.

El plan A4 pedía un **broker de credenciales** donde "el egress proxy INYECTE el
token por request (header `Authorization`) en vez de montarlo — el modelo de
Copilot/Antigravity", con `auth_mode` por runtime (`oauth-login` | `byok` |
`proxy-inject`).

## El problema con "inyectar en el proxy"

El egress proxy (ADR-0022, `internal/egress/proxy.go`) procesa **CONNECT**: abre
un túnel TCP y **copia bytes en crudo** entre cliente y upstream
(`io.Copy` bidireccional). El TLS es **end-to-end entre el agente y el provider**;
el proxy nunca ve el texto plano de la request, así que **no puede agregar un
header `Authorization`**.

Inyectar un header en una conexión HTTPS exige **terminar el TLS en el proxy**
(MITM): el proxy presenta un cert firmado por una **CA propia** que el agente
debe **confiar**, descifra, agrega el header, y re-origina el TLS hacia el
provider. Eso implica:

- Generar y gestionar una CA; instalar el cert en cada contenedor/imagen.
- Romper deliberadamente la garantía e2e de TLS (el proxy ve todo el tráfico en
  claro) — superficie de ataque y de fuga mucho mayor.
- Manejar pinning/HSTS de algunos providers.

Es factible (es lo que hace mitmproxy / algunos brokers enterprise), pero es un
**salto de complejidad y de modelo de confianza** que no corresponde a un primer
corte.

## Decisión

Partir A4 en dos:

### A4 v1 — Broker de credenciales (este ADR, se implementa ahora)

Hacer la **Bóveda funcional** sin MITM:

1. **Store de credenciales** (`credentials` en SQLite): `id`, `name`,
   `kind` (`api_key` | `oauth_token` | `git_token`), `provider_id` (opcional),
   `secret_source` (`env` | `keychain` | `inline_encrypted`), `secret_locator`
   (nombre de env var, o id de keychain, o blob cifrado), `created_at`. El secreto
   **nunca se guarda en claro**: o es una referencia (`env`/`keychain`) o un blob
   cifrado at-rest con una clave maestra del host.
2. **Resolución unificada** (`internal/credstore`): `Resolve(ctx, ref) (secret, error)`
   que generaliza `gitauth.Resolve`. Orden: lookup en `credentials` por `name`;
   según `secret_source` → leer env var / leer keychain del SO / descifrar blob.
   **Fallback compatible**: si `ref` no está en la tabla, cae a `os.Getenv(ref)`
   (comportamiento actual, no rompe nada existente).
3. **`auth_mode` por runtime** explícito (ya hay un campo `AuthMode` en el adapter):
   - `oauth-login` ≙ el actual `host_session` (monta creds, suscripción).
   - `byok` — key resuelta por el broker e inyectada como env del sandbox.
   - `proxy-inject` — **reservado**, lo implementa A4 v2 (abajo). Por ahora el
     handler lo acepta pero el worker lo trata como `byok` y **loguea** que la
     inyección en proxy aún no está activa (no se silencia el downgrade).
4. **Inyección desde el broker, no desde `infra/.env`**: hoy la key tiene que
   estar en el env del proceso worker. Con el broker, el worker la resuelve del
   store y la pasa al sandbox sólo en el momento del run. Para `DirectSandbox`
   (A2) y `DockerSandbox` el secreto entra como env efímero; el broker es la
   **fuente de verdad** de dónde vive.

El **tier `sandbox`+egress sigue siendo el default seguro**; el broker no cambia
el aislamiento, sólo de **dónde** salen las credenciales.

### A4 v2 — Inyección en el proxy (MITM) — DIFERIDO

Cuando haga falta el modelo "el token nunca entra al contenedor": el proxy
termina TLS con una CA de BattOS confiada por las imágenes, y agrega
`Authorization` desde el broker por request. Va en su propio ADR cuando se
priorice (post-v0.2). Requiere: CA management, distribución del cert a las
imágenes, allowlist de hosts MITM-eables, y auditoría reforzada porque el proxy
pasa a ver tráfico en claro.

## Consecuencias

**A favor:**
- La Bóveda deja de ser decorativa: hay un punto único para credenciales con
  cifrado at-rest / keychain, no secretos sueltos en `infra/.env`.
- `auth_mode` queda modelado para los 3 tiers; el camino a `proxy-inject` está
  documentado y no se finge implementado.
- Cero ruptura: el fallback a `os.Getenv` mantiene todo lo que ya funciona
  (incluido `gitauth`).

**En contra / deuda:**
- `proxy-inject` real (token fuera del contenedor) queda para v2; hasta entonces
  `byok` mete la key como env del sandbox (sigue siendo el modelo de hoy, sólo
  que la fuente es el broker).
- Cifrado at-rest necesita una clave maestra del host (otro secreto a gestionar);
  el primer corte puede limitarse a `secret_source` = `env`/`keychain` y dejar
  `inline_encrypted` detrás de esa pieza.

## Validación

- Un run `byok` resuelve la key vía el broker (no directo de `os.Getenv` en el
  adapter) y ejecuta OK; quitar la key del broker hace fallar el run con un error
  claro de credencial faltante.
- `auth_mode: proxy-inject` se acepta, corre como `byok`, y deja un log explícito
  de que la inyección en proxy no está activa todavía.
- Los tests de `gitauth` siguen verdes (fallback intacto).
