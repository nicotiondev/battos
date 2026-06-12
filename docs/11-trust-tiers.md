# Trust tiers de ejecución (Fase A — "siguiente nivel")

Estado: **implementado y testeado** (A1–A5 + gate + broker). Smokes con agentes
reales (A6) dependen del entorno del operador. Plan completo:
`la-idea-es-que-snoopy-truffle.md`.

## La abstracción

Un **run = (runtime) + (trust tier) + (auth mode)**. El tier se elige **por
run** vía `execution_mode` y decide *dónde* se ejecuta:

| tier | dónde | aislamiento | egress | cuándo |
|---|---|---|---|---|
| `sandbox` (default) | contenedor Docker efímero | fuerte | allowlist proxy (ADR-0022) | código no confiable |
| `direct` | proceso en el host | ninguno | red del host (sin filtro) | trabajo confiable, veloz, warm |
| `connected` | servicio always-on (Hermes/OpenClaw) | el del servicio | el del servicio | reusar un agente ya corriendo |

`sandbox` sigue siendo el **default seguro**; `direct` y `connected` son opt-in
con **aprobación explícita y separada**.

## Piezas (todas en `apps/api`)

- **A1 — `execution_mode` por run.** Columna en `runs`
  (`sandbox|direct|connected`, default `sandbox`); el handler lo acepta/valida en
  el propose; el worker elige la impl de `Sandbox` por run vía
  `Worker.SandboxFor(execution_mode)` (`cmd/worker/main.go`), no por config global.
  `dry_run` global sigue siendo el master off-switch.
- **A1b — gate de approval `execution_mode`.** Un run direct/connected no pasa a
  `queued` (approval `execute`) sin aprobar antes `kind=execution_mode`
  (`CountApprovedRunApproval`). Sandbox no lo requiere. Mismo patrón que
  `host_session`.
- **A2 — `DirectSandbox`** (`internal/worker/sandbox_direct.go`). Corre el CLI del
  agente en el host (sin Docker) reusando workspace/prompt-file/streaming-redactado/
  scan-de-artifacts (`host_exec.go`, compartido con connected). **Sin egress
  control**: la red es la del host. Tradeoff consciente del tier confiable.
- **A3 — `ConnectedSandbox`** (`internal/worker/sandbox_connected.go`). Reenvía el
  run a un servicio always-on: `local-cli` (corre el CLI del servicio en host con
  placeholders `{{prompt}}`/`{{prompt_file}}`) o `http` (POST JSON al endpoint).
  Config en `execution.connected_runtimes` (`config/battos.yaml`); `ConnectedAdapter`
  dirigido por config; `validatePlan` permite command vacío en este tier.
- **A4 — broker de credenciales** (`internal/credstore`, ADR-0023). Resuelve
  `credential_ref` vía `env` / `inline_encrypted` (AES-256-GCM con
  `BATTOS_MASTER_KEY`) / `keychain` (futuro), con fallback a `os.Getenv` (no rompe
  `gitauth`). Ya es el resolvedor unificado de git tokens (clone + push). El
  **proxy-inject (MITM)** —token que nunca entra al contenedor— es A4 v2/post-v0.2
  (el proxy hace CONNECT/túnel, no puede inyectar headers sin terminar TLS).
- **A5 — UI** (`apps/web`). Selector de trust tier en propose-run, badge de
  `execution_mode` en los runs, panel de runtimes vivos.

## Flujo de aprobación de un run no-sandbox

```
propose (draft, execution_mode=direct)
  → approve kind=execution_mode      (consentimiento del tier)
  → approve kind=execute             (gate: exige execution_mode ya aprobado)
  → queued → worker → SandboxFor("direct") → DirectSandbox en host
```

## A6 — smokes reales (gate de cierre, depende del entorno)

La mecánica de los sandboxes está probada con procesos host reales en los tests
(`sandbox_direct_test.go`, `sandbox_connected_test.go`: exec real de `cmd`/`sh`,
`httptest`, asserts de output+artifacts). El smoke **end-to-end con agentes
reales** (codex direct, claude sandbox, hermes connected) se corre en la máquina
del operador con los servicios + Docker arriba, igual que se validó codex
`host_session` en su momento. Cada uno descubre su egress con `log_only` si filtra.
