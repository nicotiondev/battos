# ADR-0011: v0.1 incorpora ejecucion supervisada en contenedores

- **Status**: Accepted
- **Fecha**: 2026-05-26
- **Decidido por**: Nico + Codex
- **Supersedes**: ADR-0006

## Context

El alcance inicial de `v0.1` era observacion y configuracion sin ejecutar
agentes. Tras revisar los informes Agent OS, Gentle-AI y la experiencia
deseada, el valor principal de BattOS requiere completar un ciclo real:
crear una tarea, asignar un agente, ejecutar codigo bajo control, revisar el
resultado y aprobar su versionado.

Ejecutar CLIs sin aislamiento convertiria el panel en acceso peligroso al
servidor. Ejecutar cualquier binario detectado tambien confundiria inventario
con permisos.

## Decision

`v0.1` sera un **Mission Control ejecutor supervisado**.

- Ejecutara sólo runtime adapters aprobados; los adapters iniciales son
  `claude-code` y `codex`.
- Cada run requerira confirmacion explicita y correra en un contenedor efimero
  con workspace aislado.
- Un run podra editar, instalar dependencias y ejecutar build/tests dentro del
  contenedor.
- La red estara apagada por defecto y podra habilitarse por run con auditoria.
- Projects podran vincular repos GitHub autorizados o repos Git locales
  gestionados por BattOS.
- BattOS mostrara diff y artifacts; commit y push requeriran aprobaciones
  independientes.
- NovaCore entra como asistente opcional de `v0.1`: administra recursos y
  propone runs, pero nunca ejecuta sin aprobacion.

## No Incluye En v0.1

- Deploy automatico o conectores de produccion.
- Pull requests automaticos.
- Adapters para cualquier herramienta del `PATH`.
- Runtimes always-on como Hermes/OpenClaw.
- Goal Mode, ejecucion continua o Dreaming auto-mutante.

## Consequences

- BattOS entrega valor productivo antes: puede producir codigo real y
  conservar memoria, artifacts, diffs y medicion del run.
- `v0.1` requiere execution engine, contenedores, approvals, auditoria,
  streaming de logs y manejo de repositories; el release sera mayor al plan
  inicial.
- ADR-0006 queda supersedido; la frontera ya no es "no ejecucion", sino
  "ejecucion sólo aislada, aprobada y auditable".

## Related

- `docs/14-producto-final-y-roadmap.md`
- `docs/10-roadmap.md`
- `docs/11-agent-runtimes.md`
- `docs/12-novacore.md`
- ADR-0004 - Memory Core propio.
- ADR-0010 - Knowledge Workspace opcional.
