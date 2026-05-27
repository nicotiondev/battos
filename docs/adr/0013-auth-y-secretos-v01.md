# ADR-0013: Autenticacion y secretos para v0.1

- **Status**: Accepted
- **Fecha**: 2026-05-27
- **Decidido por**: Nico + Codex

## Context

BattOS pasa de observar estado y memoria a administrar repositorios y ejecutar
agentes. Aunque `v0.1` sea self-hosted y single-owner, exponer endpoints de
runs, prompts o Git sin autenticacion permitiria ejecutar acciones y leer
contexto sensible desde la red.

Al mismo tiempo, guardar API keys o tokens de repositorio dentro de PostgreSQL,
logs o artifacts aumentaria el riesgo sin necesidad para la primera version.

## Decision

`v0.1` usa un modelo single-owner con token administrativo y secretos por
referencia:

- `GET /health` y `GET /version` permanecen publicos para healthchecks.
- `/status`, `/memory/*` y todas las superficies nuevas se protegen con
  `Authorization: Bearer <token>` cuando el middleware de auth se implemente.
- El token administrativo proviene de `BATTOS_API_TOKEN`; BattOS almacena solo
  un hash si en el futuro debe validarlo desde persistencia.
- En desarrollo local se permitira desactivar auth de forma explicita, solo
  para bind local; un despliegue VPS no puede arrancar en modo abierto.
- Credenciales de providers y repositorios se almacenan como referencias
  (`env_key` o `credential_ref`), nunca como secretos en tablas, logs,
  artifacts o memoria.
- El worker inyecta al contenedor un conjunto minimo de secretos por run y
  redacciona valores sensibles antes de persistir salida.
- El dashboard no expone ni persiste claves del proveedor en el navegador.

## Consequences

### Positivas

- Define un limite de seguridad antes de abrir CRUD, chats y ejecucion.
- Mantiene instalacion self-hosted simple para un unico administrador.
- Permite migrar a usuarios/roles posteriores sin cambiar los endpoints.

### Negativas

- CLI y dashboard necesitaran gestionar una sesion/token.
- Requiere middleware, redaccion de logs y validacion de configuracion antes
  de habilitar runs.

## Alternatives considered

### API sin autenticacion en v0.1

- Mas rapida de implementar.
- Inaceptable una vez que existan repositorios, prompts y ejecuciones.

### OAuth/multiusuario desde el inicio

- Adecuado para un SaaS.
- Agrega complejidad innecesaria antes de validar el producto self-hosted.

## Implementation notes

- El contrato en `packages/openapi/openapi.yaml` define `bearerAuth`.
- La implementacion del middleware entra antes de exponer endpoints mutantes.
- La configuracion debera distinguir claramente `dev` de `production`.

## Related

- ADR-0011 - ejecucion supervisada.
- ADR-0014 - ciclo de vida de runs y approvals.
- `../14-producto-final-y-roadmap.md`.
