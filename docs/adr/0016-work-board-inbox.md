# ADR-0016 - Inbox Del Work Board

## Estado

Aceptada.

## Contexto

BattOS necesita capturar tareas sueltas antes de que el usuario decida a que
proyecto pertenecen. El esquema actual exige `project_id` en `tasks` para
mantener trazabilidad, indices simples y compatibilidad con artifacts/runs.

## Decision

Usar un proyecto especial `inbox` como contenedor de captura. BattOS no lo crea
en el boot inicial; lo crea de forma idempotente cuando el usuario crea una task
sin `project_id`.

## Consecuencias

- `battos task create --title "Idea suelta"` queda permitido.
- La API `POST /tasks` solo exige `title`; si no llega `project_id`, usa
  `inbox`.
- `task assign <task_id> <project_id>` mueve la tarea a un proyecto real.
- No se vuelve nullable `tasks.project_id`, evitando ambiguedad en Knowledge
  Center, runs futuros y tablero Kanban.
- El principio de lienzo en blanco se mantiene para contenido personal: `inbox`
  es infraestructura de captura creada por accion del usuario, no un proyecto
  de ejemplo precargado.

## Referencias

- `../10-roadmap.md`
- `../15-plan-de-objetivos.md`
- ADR-0008: lienzo en blanco.
