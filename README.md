# BattOS

> **AI Operating System** self-hosted para administrar proyectos, agentes, skills, modelos, memoria, MCP, herramientas CLI, workflows y logs desde un único panel.

BattOS no reemplaza Linux, Docker, n8n, Notion, Obsidian, Claude Code, Codex ni OpenCode. **Los orquesta**.

```text
Linux administra la máquina.
Docker administra contenedores.
EasyPanel/Coolify administran apps.
BattOS administra inteligencia, agentes, proyectos, memoria, modelos y ejecución.
```

---

## Estado actual

**v0.1.0 — en construcción** (Fase 0: bootstrap completado).

Lo que **sí** entra en v0.1:
- Panel web con dashboard tipo Command Center (lectura).
- API Go con registries de projects/agents/skills/providers/models/MCPs **(arrancan vacíos)**.
- CLI `battos` con `status`, `project list/create`, `agent list/create`, `skill list/create`, `cli detect`, etc.
- Memory Core propio (SQLite + FTS5) embebido en el API.
- Detección de CLIs externas y MCP servers configurados → registry de **Agent Runtimes**.
- Schema completo en Postgres + migraciones. **DB arranca vacía.**
- Docker Compose para levantar todo.

BattOS arranca como **lienzo en blanco**: vos creás los agentes, proyectos, skills y conexiones desde el panel/CLI. Los agentes se enrutan a runtimes externos (Claude Code, Codex, OpenClaw, Hermes, MCP, etc.) — BattOS no los reimplementa, los orquesta.

Lo que **no** entra en v0.1 (va para v0.2):
- Ejecución de tareas / llamadas a LLM / workers.
- Model Advisor con políticas reales (solo placeholder).
- Usage tracking real (solo schema).
- MCP server propio expuesto.
- Aprobación humana (HITL).

Ver [docs/10-roadmap.md](docs/10-roadmap.md) para detalle.

---

## Stack

| Capa | Tecnología |
|---|---|
| API + workers + CLI | Go 1.23 |
| Router | chi |
| ORM | sqlc (genera Go tipado desde SQL) |
| DB principal | PostgreSQL 16 |
| Memory Core | SQLite + FTS5 (modernc.org/sqlite, puro Go, sin CGo) |
| Migraciones | goose |
| Realtime | SSE (Server-Sent Events) |
| Config | viper + YAML |
| CLI | cobra + lipgloss |
| Frontend | Next.js 15 + TypeScript + Tailwind + shadcn/ui + Tremor |
| Tipos compartidos | oapi-codegen (desde OpenAPI) |
| Deploy | Docker Compose + binario único `battos` |

Detalle y razones en [docs/02-stack-decisions.md](docs/02-stack-decisions.md) y los ADRs en [docs/adr/](docs/adr/).

---

## Quickstart (cuando v0.1 esté lista)

```powershell
git clone https://github.com/nicotion/battos.git
cd battos

# Configurar
cp infra/.env.example infra/.env
# editar .env con POSTGRES_PASSWORD y BATTOS_SECRET_KEY

# Levantar todo
docker compose -f infra/docker-compose.yml up -d

# Seed inicial
./scripts/seed.ps1

# Verificar
.\scripts\build-cli.ps1   # compila battos.exe
.\bin\battos status
```

Esperado:
```text
BattOS Core: running
API: running
Database: connected
Memory: connected
CLIs detected: claude, codex, gh, docker, node
Providers configured: OpenAI, Anthropic
```

Abrir el panel: http://localhost:3000

---

## Estructura del repo

```
battos/
├─ apps/
│  ├─ api/         # Backend Go (chi + sqlc + SSE)
│  ├─ cli/         # Binario `battos` (cobra)
│  └─ web/         # Next.js dashboard
├─ packages/
│  ├─ core/        # Tipos compartidos Go
│  └─ openapi/     # openapi.yaml + tipos generados
├─ agents/         # Markdown de los 9 agentes iniciales
├─ skills/         # Markdown de las 7 skills iniciales
├─ config/         # battos.yaml, providers.yaml, models.yaml, ...
├─ docs/           # Documentación viva + ADRs
├─ infra/          # docker-compose, Dockerfiles, .env.example
├─ scripts/        # PowerShell helpers (dev-up, seed, generate)
└─ data/           # DBs locales, logs, workspaces
```

---

## CLI principal (cuando esté lista)

```bash
battos status                    # estado general del OS
battos runtime list              # runtimes de agentes disponibles
battos project create <slug>     # crear un proyecto
battos project list              # listar proyectos
battos agent create <slug> \     # crear un agente apuntando a un runtime
  --runtime claude-code
battos agent list                # listar agentes
battos skill create <slug>       # crear skill
battos skill list                # listar skills
battos cli detect                # detectar CLIs instaladas
battos cli list                  # listar CLIs registradas
battos mcp list                  # listar conexiones MCP
battos memory recent             # últimas memorias guardadas
battos memory search "ficha"     # búsqueda FTS
battos usage status              # uso de tokens/budget (placeholder en v0.1)
```

---

## Principios de seguridad

- Ningún secreto en texto plano (todos vienen de env vars).
- Sin ejecución de CLIs externas todavía (`ALLOW_CLI_EXECUTION=false` por defecto).
- Sin llamadas a APIs LLM en v0.1 (solo registro de providers).
- Todo evento queda en logs JSONL.
- Las API keys solo se validan por presencia, nunca se loguean.

Detalle en [docs/09-security.md](docs/09-security.md).

---

## Documentación

| Doc | Contenido |
|---|---|
| [00-overview.md](docs/00-overview.md) | Visión general |
| [01-architecture.md](docs/01-architecture.md) | Arquitectura por capas |
| [02-stack-decisions.md](docs/02-stack-decisions.md) | Por qué cada elección |
| [03-data-model.md](docs/03-data-model.md) | Schema SQL y ER |
| [04-api-reference.md](docs/04-api-reference.md) | Endpoints REST + SSE |
| [05-memory-core.md](docs/05-memory-core.md) | Memory Core (SQLite+FTS5) |
| [06-cli-detection.md](docs/06-cli-detection.md) | Detección de CLIs |
| [07-frontend-architecture.md](docs/07-frontend-architecture.md) | Frontend Next.js |
| [08-install-vps.md](docs/08-install-vps.md) | Instalación en VPS |
| [09-security.md](docs/09-security.md) | Modelo de seguridad |
| [10-roadmap.md](docs/10-roadmap.md) | Qué viene después |
| [11-agent-runtimes.md](docs/11-agent-runtimes.md) | Cómo se enrutan agentes a CLIs/MCPs/Hermes/OpenClaw |
| [go-primer.md](docs/go-primer.md) | Primer de Go para retomar el repo |
| [adr/](docs/adr/) | Decisiones arquitecturales registradas |

---

## Licencia

TBD.
