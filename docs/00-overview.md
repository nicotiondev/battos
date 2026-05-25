# 00 — Overview

## Qué es BattOS

BattOS es una **capa agentic self-hosted** instalable sobre Linux/VPS (o Windows local en dev) que administra desde un único panel:

- Proyectos
- Agentes de IA
- Skills (procesos reutilizables)
- Modelos y proveedores LLM
- Memoria persistente
- Conexiones MCP
- Herramientas CLI externas (Claude Code, Codex, OpenCode, Gemini CLI, GitHub CLI, etc.)
- Workflows (n8n y otros)
- Logs de ejecución y auditoría

## Qué NO es

- **No es un chatbot.** Es una capa de orquestación.
- **No reemplaza** Notion, Obsidian, n8n, Claude Code, Codex, EasyPanel ni GitHub. Los **conecta**.
- **No es un agente único.** Es un sistema operativo que orquesta agentes especializados.

## Tesis central

> Usar el agente correcto, con la skill correcta, el modelo correcto, la memoria correcta y la herramienta correcta, para cada proyecto.

```text
Linux administra la máquina.
Docker administra contenedores.
EasyPanel/Coolify administran apps.
BattOS administra inteligencia, agentes, proyectos, memoria, modelos y ejecución.
```

## Flujo operativo

```text
Solicitud del usuario
   ↓
Clasificación de tarea (proyecto + tipo)
   ↓
Selección de agente
   ↓
Selección de skill
   ↓
Model Advisor (qué modelo usar y por qué)
   ↓
Selección de herramienta (CLI / MCP / workflow n8n)
   ↓
Ejecución con permisos controlados
   ↓
Logs + memoria + métricas
   ↓
Aprendizaje del router
```

## Lienzo en blanco

BattOS arranca **vacío** en cuanto a contenido. El usuario crea:

- Sus **proyectos** (contenedores de contexto).
- Sus **agentes**, eligiendo el **runtime** que los ejecuta (Claude Code CLI, Codex, OpenCode, OpenClaw, Hermes Agent, MCP, n8n-webhook, manual…).
- Sus **skills**, cuando una tarea se repite.
- Sus **conexiones MCP**.

Las **plantillas de agentes** del ecosistema personal del autor (Zeus CEO, Iris Research, Midas, etc.) viven en `examples/agents/` como referencia copiable, no como seeds.

Ver `docs/adr/0008-lienzo-en-blanco.md` y `docs/11-agent-runtimes.md`.

## Agent Runtimes (la pieza clave)

BattOS no implementa agentes — los **enruta** a runtimes externos. Un agente en BattOS es identidad + permisos + skills + scope, y un campo `runtime_id` decide quién lo ejecuta:

| Runtime | Cómo se ejecuta |
|---|---|
| `claude-code` | Subprocess de Claude Code CLI |
| `codex` | Subprocess de Codex CLI |
| `opencode` | Subprocess de OpenCode |
| `gemini-cli` | Subprocess de Gemini CLI |
| `openclaw` | HTTP a gateway OpenClaw |
| `hermes` | HTTP a runtime Hermes |
| `mcp` | Cliente MCP a server externo |
| `n8n-webhook` | POST a workflow n8n |
| `manual` | Sin automation; ejecución a mano |

## Fuente conceptual completa

El documento maestro original (4310 líneas) vive en `G:\Mi unidad\BattOS\battOS.md`. Este repo es la implementación parcial de esa visión, comenzando por v0.1.
