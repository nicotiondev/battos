# ADR-0012 - Extension Platform modular y upgrades reversibles

**Estado**: Aceptado como direccion de producto
**Fecha**: 2026-05-26

## Contexto

BattOS debe crecer con nuevos adapters, skills, conectores y vistas sin
convertir cada mejora en una modificacion riesgosa del nucleo. La revision de
Gentle-AI refuerza el valor de componentes declarativos, instalacion guiada y
rollback de configuracion.

## Decision

BattOS adopta una arquitectura modular por contratos versionados:

- En v0.1, los recursos extensibles incluyen version, origen, estado de
  lifecycle y snapshots basicos de configuracion critica.
- En v0.2, una **Extension Platform** administra manifests, dependencias,
  instalacion, actualizacion, desactivacion y rollback.
- Una extension puede declarar adapters de runtime, skills, conectores MCP,
  exporters o integraciones de dashboard, sin saltarse las politicas de runs.
- Detectar o descargar una extension no habilita automaticamente secretos,
  red, ejecucion o escritura en repositorios; las acciones sensibles requieren
  aprobacion explicita.

## Consecuencias

- El nucleo se mantiene estable mientras se incorporan capacidades.
- Las mejoras instaladas tienen trazabilidad y vuelta atras.
- v0.1 reserva metadatos y snapshots, pero no necesita un marketplace o
  updater automatico completo.
- Toda extension futura queda sujeta a la ejecucion supervisada de ADR-0011.

## Fuera de alcance inicial

- Marketplace publico.
- Auto-update silencioso.
- Plugins arbitrarios con permisos de host.
- Resolucion compleja de dependencias de terceros.

## Relacionados

- `adr/0011-v01-ejecucion-supervisada.md`
- `../10-roadmap.md`
- `../14-producto-final-y-roadmap.md`
