# ADR-0001: Stack principal Go + TypeScript

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

Hay que elegir lenguaje primario para BattOS. Las opciones serias:

1. **Python + TS** (FastAPI + Next.js) — ecosistema AI maduro, iteración rápida.
2. **Go + TS** (chi + Next.js) — single binary, concurrencia nativa, footprint bajo.
3. **Node + TS** (NestJS + Next.js) — un solo lenguaje.
4. **Rust + TS** — performance máxima.
5. **Híbrido Go + Python sidecar + TS** — lo mejor de cada uno, complejidad triple.

Factores que inclinan la decisión:

- BattOS se describe como **self-hosted instalable en VPS** (§23 del doc maestro).
- El mockup del Command Center exige streaming concurrente de ~30 series + watch de procesos CLI.
- El usuario priorizó "**lo más optimizado posible**" explícitamente.
- El usuario va a aprender Go por primera vez — el plan debe absorber esa curva.

## Decision

**Go + TypeScript/Next.js.** El workspace/API usa actualmente Go 1.25; módulos auxiliares pueden conservar una versión mínima menor mientras compilen dentro del workspace.

- Backend (API + CLI + workers): Go.
- Frontend (panel): TypeScript + Next.js 15.
- Memoria, registries, detection: todo Go embebido en el binario del API.
- Python no entra como dependencia core. Si en el futuro un skill necesita librerías Python (LiteLLM avanzado, instructor), corre como sidecar opcional o subprocess, no como parte del binario principal.

## Consequences

### Positivas

- **Distribución como single binary**. `scp battos user@vps && ./battos` funciona.
- **Footprint chico**: ~30–80 MB RAM por proceso. Importante en VPS modestos.
- **Concurrencia natural** para el dashboard streaming, watch de CLIs, polling de providers.
- **Startup instantáneo** del CLI (<100 ms) — `battos status` se siente nativo.
- **Tipado fuerte end-to-end** con OpenAPI compartido entre Go y TS.
- **Ecosistema infra de primer nivel**: SDKs oficiales de Anthropic, OpenAI, Google, MCP y Docker en Go.

### Negativas

- **Curva de aprendizaje** para el usuario (primera vez con Go). Mitigación: `docs/go-primer.md` con conceptos en línea + arranque con scope chico en Fase 1.
- **Iteración en AI features más lenta** que Python en el primer mes. Mitigación: para tuning de prompts/skills/Model Advisor se usa Python aparte en `/research` (notebooks).
- **Menos código de ejemplo público** que Python para flujos agentic. Mitigación: los SDKs oficiales documentan bien lo principal.

### Neutrales

- Frontend queda libre de toda decisión Go. TanStack + shadcn + Tremor son estándar.
- Si v0.5 necesita un componente Python específico, se agrega como servicio separado sin tocar el core.

## Alternatives considered

### Python + TS (FastAPI)
- ✅ Ecosistema AI más maduro y código de ejemplo abundante.
- ✅ Iteración rápida en prompts/skills.
- ❌ Footprint y startup mucho peores.
- ❌ Concurrencia con asyncio + GIL es más frágil bajo carga (30+ SSE streams, watch CLI, polling).
- ❌ Distribución compleja (Docker obligatorio o PyInstaller).
- ❌ No se siente como producto cuando se compara con herramientas Go puro de la misma categoría.

### Node + TS (NestJS)
- ✅ Un solo lenguaje (TS) front y back.
- ❌ Performance y memoria peor que Go.
- ❌ Ecosistema AI más débil que Python.
- ❌ Concurrencia event-loop no tan eficiente como goroutines para watch+streaming pesado.

### Rust
- ✅ Performance top.
- ❌ Curva de aprendizaje brutal para un proyecto donde la mayor parte del valor es orquestación, no perf submilisegundo.
- ❌ Ecosistema AI aún inmaduro.
- ❌ Compile times largos lastran iteración.

### Híbrido (Go + Python sidecar + TS)
- ✅ Lo mejor de cada mundo en teoría.
- ❌ Tres lenguajes desde día 1 = tres contextos, tres pipelines de CI, tres formatos de error.
- ❌ Aceptable como evolución futura (v0.5+), no como starting point.

## Implementation notes

- `go.work` en la raíz con `apps/api`, `apps/cli`, `packages/core`, `packages/openapi` (cuando exista).
- Cada `apps/*` es un módulo Go separado con su `go.mod`.
- Frontend `apps/web` es un workspace npm independiente (no monorepo TS).
- CI futuro: dos pipelines paralelos, uno Go (`go test ./apps/api/... ./apps/cli/... ./packages/core/...`), uno Node (`npm run build && npm run typecheck`).

## Related

- `docs/02-stack-decisions.md` — vista resumida.
- `docs/go-primer.md` — onboarding Go para el usuario (Fase 1).
- `adr/0005-sqlc-vs-orm.md` — capa DB sin ORM (consecuencia de elegir Go).
