# 16 - Plan de Portabilidad y Memoria estilo Engram

> Plan operativo para que BattOS funcione como **una sola instalación que
> escala**: una PC sola, un pendrive con conexión, o un servidor en casa; con la
> **memoria accesible y sincronizable entre máquinas** al estilo Engram.
>
> Decisiones de fondo: `docs/adr/0021-sqlite-unificado.md` (base de datos),
> `docs/adr/0020-oauth-cli-credentials.md` (auth de CLIs) y
> `docs/adr/0017-memory-bridge-transversal.md` (memoria transversal).

## Norte

Un mismo binario de BattOS debe poder:

1. Correr **completo en una PC sola** (con Docker ejecuta agentes; sin Docker,
   modo lite: memoria, work board, knowledge, dashboard, NovaCore).
2. Usarse desde un **pendrive/laptop con conexión**, leyendo y escribiendo la
   **misma memoria** del nodo central.
3. Vivir en un **servidor en casa** como "cerebro" siempre encendido, accesible
   por Tailscale sin abrir puertos.

La memoria es la pieza central: debe comportarse como Engram — leíble desde
cualquier máquina conectada y, a futuro, sincronizable offline.

## Principios

- **Un código, un binario**, con un switch de ejecución (Docker on/off). No dos
  versiones separadas.
- **SQLite como única base de datos** (ADR-0021). Postgres queda opcional para un
  futuro multiusuario.
- **Self-hosted y privado**: el dato vive en la máquina/servidor del usuario.
- **Acceso remoto por VPN privada (Tailscale)**, nunca puertos abiertos al
  mundo.
- **HITL y aislamiento se conservan**: portabilidad no relaja la seguridad de
  runs (contenedor, red OFF por defecto, approvals).

## Modelo de despliegue

```text
        SERVIDOR EN CASA  (cerebro, siempre encendido)
        BattOS full: SQLite + Docker + agentes + OAuth logueado
                 ^
                 |  Tailscale (VPN privada)
        +--------+--------+
        |                 |
  PC de trabajo      Pendrive / laptop
  (full o lite)      (lite, con conexion -> lee la MISMA memoria)
```

## Fases

| Fase | Objetivo | Estado |
|---|---|---|
| 1 | Memoria accesible en red (Tailscale + API remota con token) | En curso |
| 2 | Memoria estilo Engram: MCP server + conflict judgment + sync | Pendiente |
| 3 | Unificación en SQLite de todos los stores (ADR-0021) | Pendiente |
| 4 | OAuth de las CLIs en runs (ADR-0020) | Pendiente |
| 5 | Sync offline del pendrive (réplica + merge estilo Engram git) | Pendiente |

### Fase 1 - Leer la memoria desde cualquier lado (quick win)

La memoria ya expone API HTTP (`/memory/*`) y la CLI acepta `--api` y
`BATTOS_API_TOKEN`. El objetivo es habilitar y documentar el acceso remoto
seguro, sin tocar el core.

Pasos:

- BattOS corre en el nodo central (PC o servidor) con `auth.mode=token` y
  `BATTOS_API_TOKEN` definido; bind a la interfaz Tailscale (o `0.0.0.0` detras
  de Tailscale).
- Instalar **Tailscale** en el nodo central y en el pendrive/laptop.
- La CLI remota apunta al nodo: `battos --api http://<tailscale-host>:8000 --token <token> memory search "..."`.
- Verificar lectura y escritura de memoria contra el nodo remoto.

Criterio de cierre:

- Desde una segunda máquina (vía Tailscale) se puede `memory recent`,
  `memory search` y `memory save` contra el nodo central.
- Sin token o con token invalido, el API rechaza con 401.
- Documentado en este plan y en el README.

Seguridad:

- Nunca exponer el API sin `auth.mode=token`.
- Preferir bind a la IP de Tailscale; no abrir el puerto en el router.

### Fase 2 - Memoria estilo Engram

Portar al Memory Core las capacidades que hoy no tiene (ver comparativa en
`docs/05-memory-core.md`):

- **MCP server** sobre el Memory Core, para que `codex`/`claude` y otras
  herramientas lean/escriban la memoria de BattOS directamente.
- **Conflict judgment**: al guardar, buscar candidatos por FTS5 y juzgar con la
  CLI del agente si hay contradiccion (inspirado en Engram `--semantic`).
- **Export/import** de memoria por proyecto (JSONL o chunks), base del sync.

### Fase 3 - Unificacion en SQLite

Migrar los stores hoy respaldados por PostgreSQL (work board, runs, usage,
novacore, repositories, audit) a SQLite, retargeteando `sqlc` al engine SQLite y
reescribiendo el dialecto de las queries. Resultado: un unico archivo de datos,
sin servidor de DB, portable. Detalle y trade-offs en ADR-0021.

### Fase 4 - OAuth de las CLIs

Implementar el modo `host_session` de ADR-0020: login una vez en el nodo,
credenciales montadas read-only en cada contenedor de run, red por allowlist al
provider. Para pendrive: credenciales en `data/` cifradas.

### Fase 5 - Sync offline del pendrive

Permitir que el pendrive lleve una **replica SQLite** y trabaje sin conexion,
sincronizando al reconectar (merge/dedup estilo Engram git-sync). Solo tras
estabilizar Fases 1-3.

## Orden recomendado

```text
1. Tailscale + memoria remota   -> usar BattOS desde el pendrive con conexion
2. MCP de memoria               -> codex/claude consumen la memoria directo
3. Conflict judgment            -> memoria que se cura sola (Engram)
4. SQLite unificado             -> pendrive 100% portable
5. OAuth (ADR-0020)             -> suscripcion en vez de API key
6. Sync offline                 -> trabajar sin internet
```

La Fase 1 entrega el grueso del objetivo ("usar BattOS desde el pendrive con
conexion y leer la memoria") casi sin desarrollo; el resto agrega las
capacidades de Engram encima.
