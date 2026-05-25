# ADR-0009: NovaCore — agente meta que ayuda a usar BattOS

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

BattOS arranca como **lienzo en blanco** (ADR-0008): sin proyectos, agentes, skills ni MCPs pre-cargados. Esto resuelve el problema de "opiniones impuestas" pero crea otro: el primer login enfrenta al usuario con un panel vacío y un CLI con muchos comandos pero ninguna pista de por dónde empezar.

Además, el modelo de Agent Runtimes (ADR-0008, `docs/11-agent-runtimes.md`) y los marketplaces de skills/MCPs introducen muchas opciones que requieren saber qué elegir:

- "¿Qué runtime le pongo a este agente?"
- "¿Esta skill me sirve?"
- "¿Cómo conecto Notion?"
- "¿Por qué claude-code no aparece como detectado?"

Sin un asistente integrado, la única respuesta es leer docs o adivinar.

## Decision

**Agregar NovaCore: un agente "meta" que vive dentro de BattOS y ayuda al usuario a usar BattOS.**

NovaCore NO es:
- Un agente que opera proyectos (eso lo hacen Zeus, Builder, Iris, etc., que son agentes que vos creás).
- Un competidor de Claude Code / Codex (esos siguen siendo runtimes invocables).
- Un componente externo (vive en el binario `battos-api`).

NovaCore SÍ es:
- Un **agente más** dentro de la tabla `agents`, con dos flags especiales: `is_lead=true` (el panel le da slot destacado) e `is_meta=true` (tiene acceso a tools de administración del OS).
- Un asistente conversacional accesible desde el panel (burbuja inferior derecha, ya prevista en el mockup como "Lead System Agent · NovaCore") y desde el CLI (`battos ask "..."`, `battos chat nova`).
- Un agente con **autonomía de razonamiento pero no de acción**: sugiere comandos, pide confirmación, ejecuta solo con HITL.

## Consequences

### Positivas

- **Resuelve el problema del lienzo en blanco.** El primer mensaje que ve el usuario es NovaCore presentándose y sugiriendo qué hacer.
- **Documentación viva.** NovaCore lee `docs/` y responde preguntas adaptadas al estado actual del OS (ej: "no tenés runtimes detectados — corrá `battos cli detect`").
- **Discovery natural** de features sin tener que leer changelogs.
- **Reduce fricción del CLI.** Si no recordás el comando, lo preguntás.
- **Punto único de soporte.** Reemplaza la necesidad de un FAQ extenso o un Discord para usuarios.
- **Trazabilidad.** Cada interacción queda en `executions` + `memory_items`. Aprendés dónde se traba la gente.

### Negativas

- **Consume tokens** — cada conversación tiene costo. Mitigación: modelo barato por defecto (Haiku/4o-mini), budget configurable, cache de respuestas frecuentes, opción de apagarlo.
- **Más superficie de seguridad** — un LLM con acceso a tools podría sugerir mal. Mitigación: HITL estricto, NovaCore nunca ejecuta sin confirmación, allowlist de tools acotada.
- **Más complejidad para v0.3+** — function calling, gestión de conversaciones, contexto. Mitigación: arranca en modo "discovery-only" (lista y explica, no ejecuta) y va sumando capacidades por fase.

### Neutrales

- Si el usuario lo apaga (`config/novacore.yaml: enabled: false`), BattOS funciona al 100% sin él vía CLI/panel directo.
- Cada usuario puede usar su propia API key, modelo y proveedor.

## Arquitectura

```
┌──────────────────────────────────────────────┐
│ User                                         │
│   ↓                                          │
│   • Panel: burbuja chat (esquina inf. der.)  │
│   • CLI:   battos ask "..." / battos chat    │
└──────────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────────┐
│ NovaCore (agente con is_lead=true)           │
│   - runtime: direct-api                      │
│   - provider: anthropic | openai | google    │
│   - model: claude-haiku-4-5 (default)        │
│   - system_prompt: "Sos NovaCore, ..."       │
│   - tools: [list_*, get_*, install_*, ...]   │
└──────────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────────┐
│ BattOS API (endpoints internos)              │
│   /projects /agents /skills /runtimes        │
│   /mcps /memory /providers /cli              │
│   ← NovaCore los invoca como tool calls      │
└──────────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────────┐
│ Usuario confirma (HITL)                      │
│   y → BattOS ejecuta acción                  │
│   n → NovaCore explica/ajusta                │
└──────────────────────────────────────────────┘
```

## Schema

Tablas existentes que se extienden:

```sql
-- agents: agregamos 2 flags
ALTER TABLE agents ADD COLUMN is_lead   BOOLEAN DEFAULT false;
ALTER TABLE agents ADD COLUMN is_meta   BOOLEAN DEFAULT false;

-- Nueva tabla para la conversación con NovaCore (largo plazo)
CREATE TABLE novacore_conversations (
  id           TEXT PRIMARY KEY,
  user_id      TEXT,                -- v0.1 single-user, futuro multi
  started_at   TIMESTAMP,
  ended_at     TIMESTAMP,
  message_count INT,
  total_tokens INT,
  total_cost_usd NUMERIC(10, 4)
);

CREATE TABLE novacore_messages (
  id              TEXT PRIMARY KEY,
  conversation_id TEXT REFERENCES novacore_conversations(id),
  role            TEXT,             -- user | assistant | tool
  content         TEXT,
  tool_calls      JSONB,            -- si role=assistant y llamó tools
  tool_result     JSONB,            -- si role=tool
  tokens_in       INT,
  tokens_out      INT,
  created_at      TIMESTAMP
);
```

## Tools disponibles para NovaCore

**Read-only (discovery)** — disponibles desde v0.3:
- `list_projects`, `list_agents`, `list_skills`, `list_runtimes`, `list_mcps`, `list_providers`
- `get_status`, `get_health`, `detect_clis`, `test_mcp`
- `search_memory`, `recent_memory`
- `explain_concept` (lee docs/), `show_example`

**Mutating (HITL obligatorio)** — disponibles desde v0.4:
- `install_skill_from_url`
- `install_mcp_from_url`
- `create_project`, `create_agent`, `update_agent`
- `register_runtime`

**Proactive (solo v0.5+)**:
- `notify_user` (avisos de novedades, alertas de budget)
- `suggest_improvement` (basado en patrones de uso)

## System prompt base

Vive en `config/novacore.system.md`. Resumido:

```
Sos NovaCore, asistente de BattOS.

Tu rol:
1. Ayudar al usuario a entender y usar BattOS.
2. Sugerir comandos y configuraciones concretas.
3. Educar sobre conceptos (MCP, Agent Runtime, Skill, etc.).
4. Recomendar skills/MCPs/agentes según necesidad.
5. Diagnosticar problemas.
6. Onboarding cuando el OS está vacío.

Reglas inquebrantables:
- NUNCA ejecutes acciones sin confirmación explícita del usuario.
- Siempre mostrá el comando exacto antes de ejecutar.
- Si dudás, preguntá.
- Adaptá profundidad técnica al contexto.
- Respondé en el idioma del usuario.

Tools: [...lista cargada dinámicamente]
Memoria del usuario: [...contexto inyectado por turno]
```

## Configuración

```yaml
# config/novacore.yaml
novacore:
  enabled: true
  runtime: direct-api
  provider: anthropic           # anthropic | openai | google | openrouter
  model: claude-haiku-4-5
  fallback:
    provider: openai
    model: gpt-4o-mini
  budget:
    daily_usd: 0.50
    monthly_usd: 10.00
    when_exceeded: degrade      # degrade | pause | switch_to_local
  cache:
    enabled: true
    ttl_hours: 24
  system_prompt_path: config/novacore.system.md
  tools_allowlist:              # subset de tools habilitadas (default = all)
    - list_*
    - get_*
    - search_*
    - explain_concept
    - suggest_command
  hitl_required_for:            # tools que SIEMPRE requieren confirmación
    - install_*
    - create_*
    - update_*
    - register_*
```

## Alternatives considered

### No tener asistente — solo docs y CLI/panel
- ✅ Más simple, sin costo de tokens.
- ❌ Curva de adopción mucho más alta.
- ❌ El lienzo en blanco se siente abrumador.

### Asistente externo (sidekick aparte)
- ✅ Desacoplado.
- ❌ Otra instalación, otra config.
- ❌ Pierde acceso directo a la DB y memoria de BattOS.

### Hardcodear un "wizard" de onboarding sin LLM
- ✅ Determinístico, sin costo.
- ❌ No escala — cada feature nueva pide otro paso del wizard.
- ❌ No responde preguntas ambiguas.

### Integrar Claude Code/Codex como asistente (sin agente propio)
- ✅ Reaprovecha tools existentes.
- ❌ No tiene contexto de BattOS automáticamente — sería otro agente más, no el "host".
- ❌ Sin posibilidad de UI especial (la burbuja inferior derecha).

## Roadmap

| Fase BattOS | Capacidad de NovaCore |
|---|---|
| **v0.1-0.2** | No existe (sistema en desarrollo) |
| **v0.3** | Modo **discovery-only**: lista, explica, sugiere comandos. Sin ejecutar. Function calling básico. |
| **v0.4** | **HITL ejecutivo**: puede ejecutar acciones con confirmación. Memoria conversacional. |
| **v0.5** | **Proactivo**: avisa de novedades, alertas de budget, sugerencias. |
| **v0.6** | **Aprende del usuario**: nota patrones, sugiere atajos personalizados. |

## Implementation notes

- En la tabla `agents` se inserta una fila con `slug='novacore'`, `is_lead=true`, `is_meta=true` automáticamente al seed inicial (Fase 2).
- El panel detecta `is_lead=true` y renderiza la burbuja en esquina inferior derecha.
- El CLI agrega comandos `battos ask <prompt>` y `battos chat nova` solo si NovaCore está enabled.
- Toda interacción se loguea en `novacore_conversations` + `novacore_messages` + `executions` + `usage_events`.
- HITL: para acciones mutantes, el flujo es: NovaCore llama `suggest_command` (read-only) → muestra al usuario → usuario confirma → BattOS ejecuta `install_*` (server-side).

## Related

- `docs/11-agent-runtimes.md` — runtime `direct-api` que usa NovaCore.
- `docs/12-novacore.md` — documentación de usuario (cómo activar, configurar, usar).
- ADR-0008 — lienzo en blanco (motiva la existencia de NovaCore).
