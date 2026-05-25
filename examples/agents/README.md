# examples/agents/

Plantillas de agentes para copiar cuando los necesites.

**Estos archivos NO se cargan al boot de BattOS.** Son solo referencia.

## Cómo usar una plantilla

### Desde el panel
1. Ir a `Agents` → `Create Agent`.
2. Pegar el contenido del MD que quieras como base.
3. Asignar **runtime** (Claude Code, Codex, OpenCode, OpenClaw, Hermes, MCP, n8n, manual…).
4. Asignar proyectos permitidos y skills disponibles.

### Desde el CLI
```bash
battos agent create --from-template examples/agents/zeus-ceo.md --runtime claude-code
```

## Plantillas incluidas

| Slug | Rol sugerido | Runtime sugerido |
|---|---|---|
| `zeus-ceo` | Estrategia y priorización | claude-code o manual |
| `iris-research` | Investigación y análisis | claude-code |
| `midas-trading` | Análisis financiero | manual (alto riesgo) |
| `ojodera-security` | Seguridad y revisión | claude-code |
| `automator` | Automatización n8n / APIs | codex o claude-code |
| `builder` | Desarrollo de software | claude-code o codex |
| `wiki-keeper` | Documentación | claude-code |
| `project-manager` | Tareas y deadlines | manual o n8n |
| `module-connector` | Integraciones internas del OS | claude-code |

## Estructura mínima de un agente

```yaml
---
slug: <slug-unico>            # ej: my-agent
name: <Nombre humano>
role: <una línea>
runtime: <runtime-id>         # claude-code | codex | opencode | openclaw | hermes | mcp | n8n | manual
runtime_config: {}            # params específicos del runtime (opcional)
risk_level: low | medium | high
allowed_tools: []             # lista de tools que puede usar
allowed_projects: []          # vacío = todos
status: active | draft | disabled
---

# <Título>

Descripción libre del agente: qué hace, cuándo usarlo, cuándo no.
```

Ver `docs/11-agent-runtimes.md` para detalle de cada runtime disponible.
