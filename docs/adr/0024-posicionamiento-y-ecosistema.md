# ADR-0024 — Posicionamiento de BattOS + integración del ecosistema como tiers

- **Estado**: Accepted
- **Fecha**: 2026-06-11
- **Decidido por**: Nico + Claude
- **Relacionado**: ADR-0023 (broker), [11-trust-tiers.md], ADR-0025 (memoria), plan `la-idea-es-que-snoopy-truffle.md`.

## Context

Surgió la pregunta de fondo: ¿qué ES BattOS, tiene objetivo mayor, y cómo se
relaciona con el ecosistema existente (Gentle-AI, Engram, Pi, Hermes, OpenClaw,
Odysseus)? Se investigaron esos proyectos en profundidad (junio 2026).

## Decision

**BattOS es un "control plane" self-hosted de orquestación multi-agente para
coding: corre un *equipo* de agentes como jobs gobernados, observables y en
paralelo — usando las *suscripciones* del usuario, con sandbox, tiers de
confianza, approvals, memoria compartida y dashboard.**

Analogía: lo que CI/Kubernetes es para deployar código, BattOS es para **operar
agentes**. BattOS **orquesta** runtimes externos; **no los reinventa**.

### Los runtimes externos se mapean a los tiers (ADR trust-tiers)

| Proyecto | Qué es | Licencia | Rol en BattOS | Integración |
|---|---|---|---|---|
| **Pi** (earendil-works) | harness CLI minimalista (`-p`, JSON, RPC stdio, SDK), Agent Skills, sin MCP | MIT | runtime *sandbox/direct* | adapter `pi` (CommandAdapter, como codex) |
| **Hermes** (NousResearch) | agente autónomo, sub-agentes, memoria, gateway always-on | MIT | runtime *connected* | `connected_runtimes` por config (A3), local-cli o http |
| **OpenClaw** | gateway local-first, WebSocket, sandbox backends (Docker/SSH) | MIT | runtime *connected* + primo arquitectónico | `connected_runtimes` http/ws por config |
| **Odysseus** | workspace self-hosted single-agent, REST | **AGPL-3.0** | solo *arm's length* (HTTP) | ⚠️ nunca vendorizar (copyleft de red) |
| Claude Code / Codex / Gemini | CLIs oficiales | — | runtimes *sandbox/direct* (host_session) | adapters existentes |

El `ConnectedSandbox` + adapter dirigido por config de **Fase A3** ya habilita
Hermes/OpenClaw como **trabajo de config, no de arquitectura**.

### Relación con Gentle-AI: peer, no fundación

Gentle-AI es un *configurador de ecosistema* (inyecta memoria/SDD/skills/persona
en la config de agentes existentes y usa su delegación nativa). Su modelo de
ejecución es **opuesto** al de BattOS (config-injection vs job gobernado), y sus
artefactos están **acoplados a su CLI** (`state.json`, `.atl/`). Por lo tanto:

- **NO** construir BattOS sobre Gentle-AI ni vendorizar sus artefactos.
- **SÍ** adoptar sus *patrones* nativamente sobre el modelo de runs de BattOS:
  - **SDD** (fases design/review/apply) → workflow sobre runs, cada fase con
    runtime/tier/modelo elegido.
  - **Judgment-Day** (jueces adversariales + fix-agent) → el review adversarial
    multi-agente de Fase B/D.
  - **Skills** → adoptar el formato `SKILL.md` (interop: un skill de
    Claude/Gentle-AI cae directo en BattOS).
  - **Modelo-por-fase** → asignación de runtime/tier por task.
- **Coexistir**: Gentle-AI tunea los agentes locales; BattOS los orquesta.

## Consequences

- **Diferenciación nítida**: governance + dashboard + inter-comunicación
  multi-agente + suscripciones. Ningún proyecto del ecosistema combina eso
  (OpenClaw = asistente/mensajería; Hermes/Odysseus/Pi = single-agent o harness).
- **Categoría validada**: OpenClaw confirma que "agent gateway local-first" es
  real; BattOS se diferencia por el foco coding-team + governance.
- **Watch de licencias**: Pi/Hermes/OpenClaw = MIT (integración libre);
  Odysseus = AGPL (solo HTTP arm's length, nunca embeber).
- **Disciplina anti-platform-build-infinito**: dogfood temprano. El primer
  "team run" real (p.ej. Claude lead + Codex + Pi comunicándose por el mailbox)
  es la prueba de fuego del objetivo mayor antes de gold-platear B/C/D.

## Definición de "objetivo mayor logrado"
Desde el dashboard o el IDE asignás una tarea eligiendo agente + tier + tu
suscripción; varios agentes (incluidos runtimes externos como Pi/Hermes) la
trabajan en paralelo comunicándose; todo queda en memoria persistente; y Nova
puede orquestarlo bajo tu aprobación — con sandbox/egress/approvals en los tres
tiers.
