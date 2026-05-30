# BattOS

> **Mission Control agentic self-hosted** para administrar proyectos, agentes,
> skills, memoria, conocimiento, modelos y ejecuciones desde dashboard y CLI.

BattOS no reemplaza Linux, Docker, GitHub, Claude Code, Codex, Obsidian ni
n8n. Los orquesta con contexto, permisos, persistencia y auditoria.

```text
Linux administra la maquina.
Docker aisla los runs.
Git conserva los cambios.
BattOS administra trabajo, agentes, memoria, ejecucion y aprobaciones.
```

## Estado actual

**v0.1.0 - en construccion.** Las Fases 0, 1, 2 y 3A estan completadas. La
Fase 3B esta en curso: el Work Board ya se puede operar y sigue Knowledge
Center.

Implementado actualmente:

- API Go con `GET /health`, `GET /version` y `GET /status`.
- CLI `battos status`.
- Schema PostgreSQL inicial y queries tipadas con sqlc.
- Memory Core propio (SQLite + FTS5) con HTTP y CLI: `recent`, `search`,
  `save`, `stats`.
- Contrato OpenAPI v0.1 y decisiones de autenticacion, secretos, runs y
  approvals.
- Middleware Bearer configurable y soporte CLI para `BATTOS_API_TOKEN`; en
  desarrollo sin token el API solo escucha en `127.0.0.1`.
- Fase 3B en curso: persistencia `sqlc` y API/CLI del Work Board para domains,
  projects, goals y tasks; persistencia preparada para knowledge workspaces,
  journals y artifacts.
- Modo interactivo `battos` / `battos shell` con comandos slash iniciales.

En Docker/VPS se debe definir `BATTOS_API_TOKEN`; el compose habilita
`auth.mode: token` al publicar el API.

Objetivo final de **v0.1**:

- Modelo de trabajo: domains, projects, goals, tasks y board.
- Knowledge Center: journals, artefactos y previews administrados.
- Agentes y skills versionadas con adapters iniciales para Claude Code y
  Codex.
- Runs aprobados en contenedores efimeros, con logs, consumo, diff y
  artefactos.
- Repositorios Git locales gestionados o GitHub autorizado; commit y push con
  aprobaciones separadas.
- NovaCore opcional para conversar con el OS, organizar trabajo y proponer
  runs.
- Dashboard completo: Command Center, Work Board, Control Room y Knowledge
  Center.

Un ejemplo: creas un proyecto para un cliente, adjuntas un diseno, pides una
landing page, eliges un agente Claude Code o Codex y apruebas el run. BattOS
lo ejecuta en un contenedor, muestra logs y consumo, guarda la entrega y te
presenta el diff antes de autorizar commit o push.

Ver [producto final](docs/14-producto-final-y-roadmap.md) y
[roadmap operativo](docs/10-roadmap.md).

## Alcance posterior

| Version | Alcance principal |
|---|---|
| v0.2 | Extension Platform con manifests/rollback, export Markdown para Obsidian, Memory export/import, SDD y pull requests aprobados |
| v0.3 | Deployment connectors aprobados, mas adapters, Ollama/routing y metricas de valor |
| v0.4+ | Hermes/OpenClaw, Goal Mode limitado y automatizacion avanzada con guardrails |

No entra en v0.1: deploy automatico, ejecucion arbitraria sobre el host,
sincronizacion bidireccional con Obsidian, autonomia indefinida ni instalacion
general de plugins.

## Persistencia y seguridad

| Necesidad | Solucion |
|---|---|
| Recursos, runs, approvals, usage y auditoria | PostgreSQL 16 |
| Memoria persistente buscable | SQLite + FTS5, Memory Core propio |
| Repositorios, journals, artefactos y snapshots | Filesystem gestionado |
| Historial entregable del codigo | Git/GitHub con aprobacion |
| Lectura humana en Obsidian | Export Markdown opcional desde v0.2 |

Los runs solo se abren mediante adapters aprobados (`claude-code` y `codex`
en v0.1), dentro de un contenedor efimero. La red esta apagada por defecto y
su activacion queda visible y auditada. Secretos no se imprimen ni se guardan
como texto plano; commit y push requieren confirmaciones independientes.

## Stack

| Capa | Tecnologia |
|---|---|
| API, worker y CLI | Go |
| Router / config / CLI | chi, viper, cobra |
| DB principal / migraciones | PostgreSQL 16, sqlc, goose |
| Memory Core | SQLite + FTS5 (`modernc.org/sqlite`) |
| Streaming | SSE |
| Contratos | OpenAPI + oapi-codegen |
| Frontend | Next.js 15 + TypeScript + shadcn/ui + Tremor |
| Aislamiento de runs | Docker container por ejecucion |

## Quickstart actual

```powershell
# Terminal 1: API; Memory Core funciona aunque Postgres no este configurado.
go run ./apps/api/cmd/api

# Terminal 2: estado y memoria
go run ./apps/cli/cmd/battos status
go run ./apps/cli/cmd/battos memory stats
go run ./apps/cli/cmd/battos project list

# Verificacion disponible
go test ./apps/api/... ./apps/cli/... ./packages/core/...
```

## CLI disponible

La terminal usa un `ASCII wordmark` propio de BattOS y la firma
`Desarrollado por [ N ] Nicotion.dev` como cabecera visual en los comandos
principales. Puedes usarla de dos formas: comandos directos o shell
interactiva.

```bash
battos
battos shell
battos status
battos memory recent
battos memory search "ficha"
battos memory save --title "..."
battos memory stats
battos domain create clientes --name "Clientes"
battos project create landing-acme --name "Landing Acme" --domain clientes
battos goal create --project landing-acme --title "Publicar landing"
battos task create --project landing-acme --title "Preparar brief"
battos task list --project landing-acme
```

Dentro de `battos shell`, escribe `/` para abrir el menu inicial o usa atajos:

```text
/status
/projects
/tasks landing-acme
/memory
/help
/exit
```

La CLI de v0.1 agregara los recursos de conocimiento, repositorios, adapters,
creacion y aprobacion de runs, logs y uso.

## Documentacion

| Doc | Contenido |
|---|---|
| [docs/14-producto-final-y-roadmap.md](docs/14-producto-final-y-roadmap.md) | Vision final, capacidades, persistencia, seguridad y versiones |
| [docs/10-roadmap.md](docs/10-roadmap.md) | Fases operativas para implementar v0.1 y posteriores |
| [packages/openapi/openapi.yaml](packages/openapi/openapi.yaml) | Contrato API fuente de verdad desde Fase 3A |
| [docs/01-architecture.md](docs/01-architecture.md) | Arquitectura por capas y flujo de ejecucion |
| [docs/03-data-model.md](docs/03-data-model.md) | Persistencia y tablas |
| [docs/05-memory-core.md](docs/05-memory-core.md) | Memory Core |
| [docs/11-agent-runtimes.md](docs/11-agent-runtimes.md) | Runtime adapters |
| [docs/12-novacore.md](docs/12-novacore.md) | Chat de administracion |
| [docs/13-comparativa-agent-os-sources.md](docs/13-comparativa-agent-os-sources.md) | Comparativa con fuentes investigadas |
| [docs/adr/0010-knowledge-workspace-opcional.md](docs/adr/0010-knowledge-workspace-opcional.md) | Obsidian/Markdown opcional |
| [docs/adr/0011-v01-ejecucion-supervisada.md](docs/adr/0011-v01-ejecucion-supervisada.md) | Ejecucion en v0.1 |
| [docs/adr/0012-extension-platform-modular.md](docs/adr/0012-extension-platform-modular.md) | Upgrades y extensiones |
| [docs/adr/0013-auth-y-secretos-v01.md](docs/adr/0013-auth-y-secretos-v01.md) | Token administrador y secretos por referencia |
| [docs/adr/0014-run-lifecycle-y-approvals.md](docs/adr/0014-run-lifecycle-y-approvals.md) | Estados de runs y aprobaciones |

## Licencia

TBD.
