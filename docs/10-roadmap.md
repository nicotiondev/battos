# 10 - Roadmap

> v1.0.0 cerrada el 2026-06-12.

## Estado v1.0

Todas las etapas del plan final están completadas. BattOS v1.0 es un control
plane self-hosted de orquestación multi-agente para coding: corre un equipo de
agentes (Claude Code, Codex, Gemini, Pi) como jobs gobernados, en paralelo, con
sandbox/tiers/approvals/dashboard/memoria/distribución.

| Etapa | Objetivo | Estado |
|-------|----------|--------|
| 1 | Runtimes reales (Gemini/Pi/Claude smoke) | ✅ Completada |
| 2 | Detectar + instalar CLIs + monitor | ✅ Completada |
| 3 | Multi-agente: delegación + RunPool + sesiones→memoria | ✅ Completada |
| 4 | Memoria via Engram (ADR-0025) | ✅ Completada |
| 5 | Nova orquestador + patrones Gentle-AI | ✅ Completada |
| 6 | Distribución (binario único) + cierre v1.0 | ✅ Completada |

## Capacidades alcanzadas en v1.0

- **Runtimes ejecutables y verificados**: adapters `claude-code`, `codex`,
  `gemini` y `pi` con smoke real por adapter; `host_session` para credenciales
  OAuth locales de Claude y Codex.
- **Detección e instalación gobernada de CLIs**: `config/cli-tools.yaml` con
  `install_command`/`install_url`, endpoint `POST /cli-tools/{id}/install` como
  mutación aprobable, métricas de sistema (CPU/MEM/NET/disco/procesos) via SSE.
- **Multi-agente con RunPool**: N goroutines concurrentes, claim atómico
  `exactly-once`, tools MCP `team_spawn_run`/`team_read_board`/`team_get_run_status`
  para que un lead delegue a sub-agentes; sesiones al cerrar auto-promovidas a
  memoria HITL.
- **Memoria via Engram**: interfaz `MemoryProvider` con `BuiltinCore` (SQLite,
  default/offline) y `EngramProvider` (HTTP `:7437`, fallback graceful). Config
  elige el provider activo. Dashboard proxya al provider elegido.
- **Nova orquestador provider-agnóstico**: tools `propose_runs`/`launch_run`;
  workflows SDD (fases design/implement/review) y Judgment-Day (review
  adversarial multi-agente); ingesta de `SKILL.md` para interop con skills de
  Claude/Gentle-AI. Provider configurable via OpenRouter/Minimax/OpenAI.
- **Binario único distribuible**: `battos serve` levanta API + worker + dashboard
  embebido (`embed.FS`), todo desde un solo binario Go. GoReleaser cross-compile
  Win/Linux/Mac. `install.sh` con `curl|sh`. GitHub Actions builds y releases.
  IDE bridge: `battos mcp install` instala team tools en Cursor y VS Code.

## Fronteras de seguridad (se mantienen en v1.0)

- Adapters ejecutables aprobados, no cualquier CLI detectada.
- Contenedor por run; terminal amplia solo dentro del contenedor.
- Red `OFF` por defecto y activación auditada.
- Secrets controlados y nunca expuestos en logs.
- Approvals separados para run, commit, push y futuros deployments.

## Post-v1 (updates siguientes)

| Versión | Alcance |
|---------|---------|
| v1.1 | Adapter ACP (Gemini nativo); proxy-inject MITM con CA propia |
| v1.2 | Engram Cloud avanzado (enroll/autosync desde BattOS) |
| v1.3 | Multi-usuario / Postgres; export Markdown/Obsidian; PR aprobado |
| v2.0+ | Odysseus runtime connected; Goal Mode limitado; ROI y skill evaluation |
