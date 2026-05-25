# 12 — NovaCore (Asistente del OS)

> **Estado**: Diseñado en Fase 0. Implementado a partir de v0.3.

## Qué es NovaCore

El asistente conversacional integrado en BattOS. Te ayuda a usar el OS:
te sugiere comandos, explica conceptos, recomienda skills/MCPs/agentes según
lo que necesitás, diagnostica problemas y guía el onboarding.

NovaCore **no** opera tus proyectos. Para eso están los agentes que vos creás
(Zeus, Builder, Iris, etc.). NovaCore **opera el OS**.

## Cómo aparece en el mockup

En el panel derecho del Command Center está la sección:

```
Lead System Agent     [View All]
🤖 NovaCore
   System Orchestrator
   ● Online
```

Es exactamente ese.

## Cómo se usa

### Desde el panel (Fase 5+)

Burbuja flotante en la esquina inferior derecha. Click → panel de chat lateral.

### Desde el CLI (Fase 4+)

```bash
battos ask "necesito conectar Notion"
battos ask "qué runtimes tengo disponibles"
battos chat nova                          # modo conversación continua
```

### Atajos especiales

```bash
battos help <comando>     # NovaCore explica el comando con ejemplos
battos suggest            # NovaCore propone qué hacer ahora según tu estado
battos diagnose           # NovaCore revisa subsistemas y sugiere fixes
```

## Qué puede y qué no puede

| Puede | No puede |
|---|---|
| Listar lo que tenés (proyectos, agentes, skills, MCPs, runtimes) | Ejecutar acciones sin tu confirmación |
| Explicar qué es un MCP, qué es un Agent Runtime, etc. | Decidir por vos en algo crítico |
| Sugerir comandos exactos para lo que querés hacer | Acceder a APIs externas que no estén configuradas |
| Recomendar skills/runtimes según tu caso | Bypassear permisos (ALLOW_CLI_EXECUTION, etc.) |
| Diagnosticar por qué algo no funciona | Modificar config sin confirmación |
| Recordar el contexto de tu conversación | Compartir info entre usuarios (en multi-tenant futuro) |

## Cómo funciona internamente

NovaCore es **un agente más** dentro de BattOS, con dos flags especiales en la tabla `agents`:

- `is_lead = true` — el panel le da el slot destacado.
- `is_meta = true` — tiene tools para administrar el OS (los agentes normales no las tienen).

Su runtime es `direct-api`: llama a Claude/GPT/Gemini API directamente (sin
necesidad de tener el CLI de esos modelos instalado).

```
Vos ────→ NovaCore (Haiku/4o-mini vía API)
              │
              ├─ tools: list_*, get_*, search_*
              ├─ tools: install_*, create_* (con HITL)
              ↓
            BattOS API (endpoints internos)
              ↓
            Acción ejecutada con tu confirmación
```

## Configuración

```yaml
# config/novacore.yaml
novacore:
  enabled: true                   # apagar con false → BattOS funciona sin NovaCore
  provider: anthropic
  model: claude-haiku-4-5         # barato y rápido — recomendado para chat
  budget:
    daily_usd: 0.50
    monthly_usd: 10.00
```

Ver `docs/adr/0009-novacore-meta-agent.md` para la lista completa de opciones.

## Apagar NovaCore

Si preferís usar BattOS solo con CLI/panel directo:

```yaml
# config/novacore.yaml
novacore:
  enabled: false
```

O en runtime:
```bash
battos novacore disable
```

BattOS sigue funcionando al 100% sin él. Lo único que perdés es la asistencia
conversacional.

## Costo

NovaCore es la única parte de BattOS que **consume tokens de un LLM externo**.

Con Claude Haiku 4.5 y uso moderado (~50 mensajes/día), el costo estimado es:

```
Input  promedio:  ~800 tokens/mensaje  × $0.0008/1K = $0.00064
Output promedio:  ~400 tokens/mensaje  × $0.004/1K  = $0.00160
                                                       --------
Total:                                               $0.00224 por mensaje
50 mensajes/día × $0.00224 = $0.11/día ≈ $3.30/mes
```

El budget config corta automáticamente cuando se excede el límite diario/mensual.

## Cuándo NO usar NovaCore

- En entornos completamente sin internet (no puede llamar API).
- Cuando querés cero costo de operación (apagalo).
- Para acciones críticas que querés revisar línea por línea — usá el CLI directo.

## Cuándo SÍ usar NovaCore

- Onboarding inicial: te guía paso a paso.
- "No sé qué comando usar" → preguntale.
- "No sé qué runtime conviene para este agente" → te recomienda con razones.
- Diagnóstico: "algo no funciona" → te pregunta y te ayuda a aislar.
- Discovery: "¿qué puedo hacer con BattOS?" → te muestra opciones.

## Roadmap

- **v0.3** — Modo discovery-only (lista, explica, sugiere; no ejecuta).
- **v0.4** — HITL ejecutivo (ejecuta con confirmación).
- **v0.5** — Proactivo (avisos, alertas).
- **v0.6** — Aprende patrones del usuario y sugiere atajos personalizados.
