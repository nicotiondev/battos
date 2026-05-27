# ADR-0014: Ciclo de vida de runs y approvals

- **Status**: Accepted
- **Fecha**: 2026-05-27
- **Decidido por**: Nico + Codex

## Context

`v0.1` debe ejecutar agentes reales sin convertir BattOS en una terminal
remota sin control. El API, el dashboard y el futuro worker necesitan compartir
una semantica estable para aprobar, ejecutar, cancelar y versionar trabajo.

El schema inicial ya posee `executions`, pero un run supervisado incluye
estado previo a la ejecucion, permisos, contenedor, red, diff y varias
aprobaciones. Es una entidad de orquestacion mas amplia.

## Decision

`runs` sera la entidad canonica para un trabajo supervisado. `executions`
permanecera como registro tecnico de invocaciones y podra asociarse a un run
mediante migracion append-only.

Estados del run:

```text
draft -> awaiting_approval -> queued -> running -> succeeded
                                  |          |  -> failed
                                  |          |  -> cancelled
                                  -> cancelled
```

Reglas:

- Crear un run desde dashboard, CLI o NovaCore produce
  `awaiting_approval`; nunca ejecuta inmediatamente.
- Una aprobacion `execute` encola el run. El worker solo reclama runs
  `queued`.
- Cada run obtiene contenedor efimero y workspace controlado.
- La red comienza `OFF`; habilitarla requiere approval `network` registrado.
- Finalizado el trabajo, `commit` y `push` son approvals separados, aun cuando
  el run haya terminado exitosamente.
- `cancel` es permitido mientras el run este pendiente, en cola o ejecutando;
  el worker debe detener y marcar la terminacion.
- Logs, artifacts, diff, parametros, permisos y approvals quedan auditables.

## Consequences

### Positivas

- Evita que chat o CRUD impliquen ejecucion accidental.
- Permite representar exactamente lo que muestra Control Room.
- Conserva telemetria tecnica detallada con `executions` sin forzarla a ser
  el workflow de usuario.

### Negativas

- Requiere nuevas tablas y migracion aditiva en Fase 3B/4B.
- Hay mas estados y casos de recuperacion que en un subprocess directo.

## Implementation notes

- OpenAPI introduce `/runs`, `/runs/{id}/approvals`, `/logs`, `/artifacts` y
  `/diff`.
- La futura migracion crea `runs`, `run_approvals`, `run_logs`,
  `run_artifacts` y agrega `run_id` nullable donde corresponda.
- Los kinds iniciales de approval son `execute`, `network`, `commit` y
  `push`.

## Related

- ADR-0011 - ejecucion supervisada en contenedores.
- ADR-0013 - autenticacion y secretos.
- `../10-roadmap.md`.
