# ADR-0018 - Dashboard Next.js 16

## Estado

Aceptado.

## Contexto

La documentacion inicial indicaba Next.js 15 para el dashboard. Durante el
avance paralelo de la Fase 5B se creo `apps/web` con Next.js 16.2.7, React
19.2.4 y ESLint 9. El build de produccion fue verificado correctamente fuera
del sandbox de Codex.

## Decision

BattOS v0.1 acepta Next.js 16 como version actual del dashboard, manteniendo:

- App Router.
- TypeScript.
- SSE para streaming.
- Cliente API generado desde OpenAPI como objetivo de hardening.
- Estados offline/degraded obligatorios antes de cerrar la Fase 5B.

## Consecuencias

- La documentacion de stack debe reflejar Next.js 16 donde describa el estado
  implementado.
- La deuda actual de lint queda permitida solo como warnings durante la base de
  dashboard; antes del release se debe reducir usando tipos compartidos y
  cliente OpenAPI generado.
- No se hara downgrade a Next.js 15 salvo que aparezca un bloqueo tecnico real.

## Verificacion

- `npm run lint`: 0 errores, warnings conocidos.
- `npm run build`: pasa fuera del sandbox; dentro del sandbox falla con
  `spawn EPERM` al ejecutar TypeScript por politica de procesos de Windows.
