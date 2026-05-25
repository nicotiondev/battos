# skills/

Esta carpeta está **vacía a propósito**.

Las skills son procesos reutilizables. BattOS no viene con un set predefinido — las creás cuando una tarea se repite dos o más veces.

## Cuándo crear una skill

Cuando notás que estás haciendo la misma cosa con el mismo patrón otra vez. Regla del doc maestro:

> Si una tarea se repite dos veces, debe convertirse en skill.

## Cómo crear una skill

### Desde el CLI
```bash
battos skill create create-landing \
  --category development \
  --risk medium
# Esto crea skills/create-landing/SKILL.md vacío para que lo edites.
```

### Desde el panel
Ir a `Skills` → `Create Skill` y completar.

## Estructura de una skill

```
skills/<slug>/
├─ SKILL.md           # frontmatter + descripción + pasos
├─ examples/          # opcional, ejemplos de inputs/outputs
└─ references/        # opcional, docs externos relevantes
```

`SKILL.md` mínimo:

```yaml
---
slug: <slug>
name: <Nombre>
category: <project | dev | marketing | analysis | infra | ...>
risk_level: low | medium | high
compatible_agents: []            # vacío = todos
compatible_runtimes: []          # opcional, restringir runtimes
inputs: []
outputs: []
status: active | draft
---

# <Título>

## Cuándo usarla
...

## Inputs requeridos
...

## Pasos
...

## Outputs esperados
...
```
