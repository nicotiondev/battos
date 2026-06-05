# ADR-0017: Memory Bridge transversal para agentes y herramientas

- **Status**: Accepted
- **Fecha**: 2026-06-01
- **Decidido por**: Nico + Codex

## Context

BattOS ya tiene un Memory Core propio inspirado en Engram: SQLite + FTS5,
HTTP y CLI, con `project_id`, `agent_id`, `scope` y `topic_key`.

El valor principal no es solo guardar recuerdos dentro de BattOS. El valor es
que la memoria sobreviva al cambio de herramienta: Claude Code, Codex,
Antigravity, NovaCore u otros agentes deben poder leer y escribir contexto del
mismo proyecto sin empezar desde cero.

## Decision

Crear una capa **Memory Bridge** encima del Memory Core.

Memory Bridge es la superficie comun para:

- Buscar memoria relevante antes de un run o una conversacion.
- Generar context packs por proyecto/agente.
- Guardar resumenes, decisiones, bugfixes, preferencias y aprendizajes.
- Exponer la memoria como HTTP, CLI y futuro MCP server propio.
- Mantener politicas de privacidad, confirmacion y deduplicacion.

BattOS no dependera de Engram como proceso externo en v0.1. Engram sigue siendo
referencia de diseno; las capacidades utiles se portan selectivamente.

## Consequences

### Positivas

- La memoria queda en BattOS, no encerrada en una CLI especifica.
- Cambiar de Claude Code a Codex no pierde contexto del proyecto.
- NovaCore puede operar con el mismo conocimiento que los runs.
- El dashboard puede mostrar y curar memoria sin depender de herramientas
  externas.
- El Memory Core sigue siendo local-first y respaldable.

### Negativas

- Hay que definir politicas para no guardar ruido, secretos o preferencias mal
  inferidas.
- MCP server, deduplicacion y conflict judgment agregan superficie de seguridad.
- La memoria automatica debe ser supervisada al principio para evitar
  acumulacion desordenada.

## Alcance inicial

Para v0.1:

- CLI `battos memory context` para generar un paquete Markdown/JSON filtrado por
  proyecto, agente y scope.
- Context pack antes de runs de Codex/Claude Code.
- Guardado de resumen de run como memoria `learning` o `decision` cuando el
  usuario lo apruebe.
- Dashboard: vista de memoria asociada a proyecto/run.

Para v0.2+:

- MCP server de BattOS Memory Core.
- Export/import JSONL por proyecto.
- Dedupe y judgment de conflictos inspirado en Engram.
- Politicas configurables de auto-captura.

## Guardrails

- Nada de secretos en memoria.
- La captura automatica empieza como sugerencia aprobable.
- Memorias personales usan `scope=personal`; memorias del proyecto usan
  `scope=project`.
- `topic_key` se usa para preferencias/decisiones estables y evitar duplicados.
- El agente puede proponer guardar memoria, pero BattOS decide segun politica y
  aprobacion.

## Related

- ADR-0004 - Memory Core propio.
- ADR-0011 - Ejecucion supervisada en v0.1.
- ADR-0013 - Auth y secretos.
- ADR-0014 - Runs y approvals auditables.
- `docs/05-memory-core.md`
- `docs/15-plan-de-objetivos.md`
