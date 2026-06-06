# ADR-0021: SQLite como base de datos unica (portabilidad)

- **Status**: Accepted
- **Fecha**: 2026-06-06
- **Decidido por**: Nico + Claude Code

## Context

BattOS hoy usa **dos motores de datos**: SQLite + FTS5 para el Memory Core
(embebido en el binario) y **PostgreSQL** para el resto (work board, runs,
approvals, usage, novacore, repositories, audit), via `sqlc` + `pgx`
(ADR-0005).

El objetivo de producto evoluciono hacia un BattOS **portable y single-user**:
una sola instalacion que corre en una PC, un pendrive con conexion o un servidor
en casa, con la misma data viajando o sincronizandose entre maquinas. PostgreSQL
es el obstaculo: es un servidor con su propio directorio de datos, no un archivo;
complica el pendrive, suma RAM y mantenimiento, y hace el sync entre maquinas mas
dificil.

Para un unico usuario, las ventajas de Postgres (concurrencia de muchos
escritores, acceso por red, escala) no se usan. SQLite con WAL cubre de sobra el
caso: 1 escritor + N lectores, escrituras cortas, runs ocasionales en paralelo.

## Decision

Unificar toda la persistencia en **SQLite** (con FTS5 donde aplique), dejando
PostgreSQL como opcion futura solo para un escenario multiusuario.

- El Memory Core ya es SQLite; se mantiene.
- Los stores hoy en Postgres se migran a SQLite: se **retargetea `sqlc` al engine
  SQLite** y se reescribe el dialecto de las queries (tipos, `RETURNING`,
  `JSONB` -> `TEXT`/`JSON`, UUID -> `TEXT`, etc.).
- Un unico archivo de datos (o pocos archivos) bajo `data/`, portable por copia.
- WAL + `busy_timeout` + pool acotado, como ya hace el Memory Core.
- BattOS deja de **requerir** un servicio Postgres para arrancar; los endpoints
  ya no devuelven 503 "configura DATABASE_URL" en el camino por defecto.

`sqlc` se conserva (soporta SQLite como engine); no se vuelve a SQL inline ni se
adopta un ORM (sigue vigente el espiritu de ADR-0005).

## Consequences

### Positivas

- **Portabilidad real**: toda la base es un archivo; pendrive, backup y sync se
  vuelven "copiar archivos".
- **Cero dependencia de servicio**: un binario + una carpeta `data/` y BattOS
  corre en cualquier maquina, sin instalar ni mantener Postgres.
- **Menos RAM** y arranque mas simple (sin contenedor de DB).
- Habilita el sync de datos entre maquinas estilo Engram (Fase 5 del plan 16).

### Negativas

- **Migracion grande**: reescribir queries y regenerar stores; revisar features
  Postgres usadas (JSONB, tipos, transacciones, FK).
- Menor concurrencia de escritura: aceptable para single-user, **no** para
  multiusuario/equipo.
- Riesgo de rendimiento de SQLite sobre medios lentos (USB/SD); recomendado SSD
  para uso intensivo, pendrive para transporte de datos.

## Alternatives considered

### Mantener Postgres y resolver portabilidad llevando su volumen

- No requiere reescribir nada.
- Rechazada como base: volumen Postgres en pendrive es fragil y lento, y el sync
  entre instancias Postgres es complejo. Postgres no desaparece como opcion, pero
  no es el camino por defecto.

### Doble backend (SQLite en lite, Postgres en server)

- Mantiene Postgres donde rinde.
- Rechazada: obliga a mantener **dos capas de datos** y a sincronizar entre
  motores distintos (SQLite <-> Postgres), lo mas dificil de todo.

## Implementation notes

- Migracion por fases junto al plan 16; no bloquea las Fases 1-2 (memoria
  remota y MCP), que funcionan con el estado actual.
- Revisar uso de `pgtype`, `pgx.ErrNoRows` (-> `sql.ErrNoRows`), columnas
  `JSONB`/`metadata`, UUIDs y `RETURNING` al portar.
- Mantener un camino de import desde un Postgres existente para no perder datos
  de quien ya lo use.

## Related

- ADR-0004 - Memory Core propio (SQLite + FTS5).
- ADR-0005 - sqlc en vez de ORM (se conserva, retargeteado a SQLite).
- ADR-0017 - Memory Bridge transversal.
- `docs/16-plan-portabilidad-memoria.md`.
