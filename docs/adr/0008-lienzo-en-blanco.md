# ADR-0008: BattOS arranca como lienzo en blanco (sin agentes/skills/proyectos seed)

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

Inicialmente el plan de v0.1 incluía **seeds** de:
- 9 agentes (Zeus CEO, Iris Research, Midas, OjoDera, Automator, Builder, Wiki Keeper, Project Manager, Module Connector).
- 7 skills (`create-project`, `create-agent`, etc.).
- 5 proyectos (RED NBL, Partshop, Nikkos Capital, SIDETH, BattOS Core).

Estos venían del doc maestro como **ejemplos del ecosistema personal de Nico**, no como parte intrínseca de BattOS.

El usuario aclaró durante la implementación que prefiere:

> "La idea es tener un lienzo en blanco, o sea, con todos los módulos y todo, pero en cuanto a los agentes, los voy creando dentro. Y ahí voy conectándolos, ya sea a través de alguna CLI o lo voy conectando, por ejemplo, un agente a través de OpenClaw o Hermes Agent."

## Decision

BattOS arranca **completamente vacío** en términos de contenido:
- `agents/` vacío (solo README explicando cómo crear).
- `skills/` vacío (idem).
- DB sin proyectos seed.
- DB sin agentes seed.
- DB sin skills seed.

El **objetivo de infraestructura** de v0.1 incluye API, panel, registries, schema, Memory Core, CLI Manager, MCP Registry, Model Advisor (stub), Usage tracker, Providers, Agent Runtimes registry y Docker. Al cierre de Fase 2 están listos API base, schema y Memory Core; el resto se entrega en fases posteriores.

Los **templates** de los agentes que se habían escrito viven en `examples/agents/` como referencia copiable. **No se cargan al boot.**

## Consequences

### Positivas

- **Reutilizable**: BattOS sirve para cualquier usuario, no solo para el ecosistema personal del autor.
- **Pedagógico**: el usuario aprende el sistema creando sus primeros agentes en lugar de heredar opiniones.
- **Mantiene el modelo mental**: "BattOS orquesta agentes externos (Claude Code, OpenClaw, Hermes, MCP, etc.), no los reemplaza."
- **Más fácil de testear**: estado inicial determinista (vacío) en lugar de seeds que pueden volverse stale.

### Negativas

- **Onboarding más largo**: el primer login al panel muestra todo vacío. Requiere guiar al usuario (wizard o docs claros).
- **Mitigación**: Fase 5 incluye un "Getting Started" panel que sugiere los primeros pasos: detectar runtimes → crear primer agente → crear primer proyecto.

### Neutrales

- Los `examples/agents/` se mantienen como referencia. No suman peso al runtime.
- Las plantillas pueden cargarse con `battos agent create --from-template examples/agents/<slug>.md`.

## Implementation notes

- Eliminar la tarea de seed de proyectos/agentes/skills del scope de Fase 2.
- Schema SQL sí mantiene todas las tablas — solo arrancan vacías.
- Agregar tabla `agent_runtimes` al schema (decisión relacionada — ver `docs/11-agent-runtimes.md`).
- Frontend muestra estados vacíos prolijos con CTAs claros ("Create your first agent", "Detect runtimes", etc.).

## Related

- `docs/11-agent-runtimes.md` — concepto de Agent Runtimes (consecuencia directa).
- `examples/agents/README.md` — cómo usar las plantillas.
- `agents/README.md` y `skills/README.md` — explican el lienzo vacío.
