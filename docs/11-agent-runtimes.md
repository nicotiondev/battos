# 11 — Agent Runtimes

## Concepto

En BattOS, un **agente** es identidad + permisos + skills + scope de proyectos.
Un **agent runtime** es el **motor que efectivamente ejecuta** las tareas de ese agente.

```text
┌──────────────┐         ┌────────────────────────┐
│   Agente     │ ─runs─▶ │     Agent Runtime      │
│  "Zeus CEO"  │         │   (Claude Code CLI)    │
│              │         │                        │
│ permisos     │         │ comando: claude        │
│ skills       │         │ workspace: ./projects  │
│ proyectos    │         │ mcp_servers: [...]     │
└──────────────┘         └────────────────────────┘
```

El mismo agente puede mover de runtime sin perder identidad/memoria/permisos. Esto desacopla **quién es** el agente de **cómo corre**.

## Runtimes soportados (target v0.1–v0.2)

### `claude-code`
**Qué es**: Claude Code CLI ejecutándose como subprocess controlado.
**Cómo conecta BattOS**: lanza `claude` con workspace dedicado, parsea I/O, captura logs.
**Cuándo usarlo**: trabajo de desarrollo, refactors, análisis de código, ejecución multi-step con tools.
**v0.1**: solo detecta presencia y registra. v0.2 ejecuta con aprobación.

### `codex`
**Qué es**: Codex CLI (OpenAI) ejecutándose como subprocess.
**Cómo conecta BattOS**: idem `claude-code` pero con comando `codex`.
**Cuándo usarlo**: generación de código, repo editing, tests.

### `opencode`
**Qué es**: OpenCode (agente local/self-hosted) como subprocess.
**Cuándo usarlo**: workflows locales sin enviar código a APIs externas.

### `gemini-cli`
**Qué es**: Gemini CLI (Google).
**Cuándo usarlo**: tareas que aprovechen context window grande (2M tokens).

### `openclaw`
**Qué es**: agente self-hosted con gateway local, skills y memoria. Corre fuera de BattOS (en su propio proceso/contenedor).
**Cómo conecta BattOS**: vía HTTP/MCP al gateway OpenClaw. BattOS le envía tareas y recibe resultados.
**Cuándo usarlo**: agentes "always-on" que reciben triggers de mensajería (Telegram/WhatsApp) y BattOS los coordina.

### `hermes`
**Qué es**: Hermes Agent — agente auto-mejorable con learning loop, skills emergentes, memoria personal. Vive en su propio proceso/VPS/serverless.
**Cómo conecta BattOS**: vía API o webhook al runtime Hermes.
**Cuándo usarlo**: agentes personales always-on con learning, accesibles por mensajería.

### `mcp`
**Qué es**: cualquier MCP server externo. El "agente" de BattOS es en realidad un cliente MCP que invoca tools de ese server.
**Cómo conecta BattOS**: stdio/HTTP transport según el server. Config en `runtime_config`.
**Cuándo usarlo**: cuando ya existe un MCP server que hace lo que necesitás (Engram, Notion, Linear, custom).

### `n8n-webhook`
**Qué es**: el agente se "ejecuta" disparando un workflow n8n.
**Cómo conecta BattOS**: POST a un webhook URL configurado. BattOS pasa el contexto, n8n hace su lógica y responde.
**Cuándo usarlo**: agentes que son en realidad orquestaciones n8n bajo el capó.

### `manual`
**Qué es**: no hay automation. El "agente" es un placeholder con identidad y permisos; las tareas se ejecutan a mano.
**Cuándo usarlo**: agentes con alto riesgo (Midas trading, decisiones financieras) o cuando todavía no decidiste qué runtime usar.

## Modelo de datos

Tablas relevantes (v0.1):

```sql
-- Registry de runtimes disponibles
agent_runtimes (
  id TEXT PRIMARY KEY,           -- 'claude-code' | 'codex' | 'openclaw' | ...
  name TEXT NOT NULL,            -- 'Claude Code CLI'
  kind TEXT NOT NULL,            -- 'subprocess' | 'http' | 'mcp' | 'webhook' | 'manual'
  status TEXT NOT NULL,          -- 'available' | 'unavailable' | 'disabled'
  health_check JSONB,            -- cómo verificar que está vivo
  schema JSONB,                  -- JSON schema de los campos de runtime_config válidos
  detected_at TIMESTAMPTZ
);

-- Agentes creados por el usuario apuntan a un runtime
agents (
  ...
  runtime_id TEXT REFERENCES agent_runtimes(id),
  runtime_config JSONB,          -- params específicos: command, args, endpoint, mcp_transport, ...
  ...
);
```

## Cómo se "detecta" un runtime disponible

Al boot, BattOS corre **CLI Detector** (Fase 4) que busca:
- Binarios en `PATH` (`claude`, `codex`, `opencode`, `gemini`, etc.) → runtimes subprocess.
- MCP servers configurados en `config/mcp.yaml` → runtime `mcp`.
- Endpoints HTTP configurados (OpenClaw URL, Hermes URL, n8n URL) → runtimes `http` / `webhook`.

El resultado se persiste en `agent_runtimes` con `status='available'` o `'unavailable'`.

## UI en el panel

Cuando el usuario crea un agente:

1. Elige **runtime** de un dropdown alimentado por `agent_runtimes` con `status='available'`.
2. Según el runtime, el panel muestra el form específico de `runtime_config` (validado contra el schema).
3. Guarda agente con `runtime_id` + `runtime_config`.

## Cambio de runtime sin perder identidad

Un agente puede cambiar de runtime:
- Hoy "Zeus CEO" corre en `claude-code`.
- Mañana lo movés a `hermes` para que sea always-on.

Lo que se mantiene: identidad, permisos, memoria asociada (en Memory Core), historial de ejecuciones.
Lo que cambia: `runtime_id` + `runtime_config`.

## Roadmap

| Versión | Capacidad |
|---|---|
| v0.1 | Detecta runtimes + registra. NO ejecuta. |
| v0.2 | Ejecuta `claude-code`, `codex`, `manual`. Workspace dedicado por proyecto. HITL para riesgo alto. |
| v0.3 | Suma `mcp` y `n8n-webhook`. |
| v0.4 | Suma `openclaw` y `hermes` (always-on, mensajería). |
| v0.5 | Routing dinámico: un mismo agente puede saltar de runtime según la tarea (delegado por Model Advisor). |
