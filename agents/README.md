# agents/

Esta carpeta está **vacía a propósito**.

BattOS arranca como un lienzo en blanco. Los agentes se crean desde el panel o el CLI, no desde archivos del repo.

## Cómo crear un agente

### Desde el CLI
```bash
battos agent create my-agent \
  --name "My Agent" \
  --runtime claude-code \
  --risk low
```

### Desde el panel
Ir a `Agents` → `Create Agent`, completar el formulario, asignar runtime y permisos.

### Desde una plantilla
```bash
battos agent create --from-template examples/agents/builder.md --runtime claude-code
```

Plantillas en [`examples/agents/`](../examples/agents/).

## Por qué no precargamos agentes

La visión de BattOS es **un OS personal/profesional**, no un producto opinionado con personajes predefinidos. Cada usuario crea los agentes que necesita y los conecta al runtime que prefiera (Claude Code CLI, Codex, OpenClaw, Hermes Agent, MCP, etc.).

Ver `docs/adr/0008-lienzo-en-blanco.md`.
