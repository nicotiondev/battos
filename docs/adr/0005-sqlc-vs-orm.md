# ADR-0005: sqlc para acceso a Postgres (no GORM, ent, SQLBoiler)

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Claude

## Context

BattOS necesita acceder a Postgres desde Go con ~14 tablas y un puñado de queries por tabla (CRUD + agregaciones para el dashboard). Opciones serias en Go:

1. **`database/sql` pelado** + `pgx`.
2. **GORM** (ORM full-feature).
3. **ent** (de Facebook, codegen-based).
4. **SQLBoiler** (codegen desde DB existente).
5. **sqlc** (codegen desde queries SQL).

## Decision

**sqlc.**

`apps/api/sqlc.yaml` configura sqlc para leer `migrations/*.sql` (schema) y `queries/*.sql`, y generar Go tipado en `internal/store/`.

## Consequences

### Positivas

- **SQL puro como fuente de verdad.** Escribimos SQL real, no DSL. Cuando una query falla, el plan es transparente; cuando aprendemos algo nuevo de Postgres, lo aplicamos directo.
- **Tipado fuerte sin runtime cost.** El código generado es lo que escribirías a mano. Sin reflection, sin magia.
- **Refactor seguro.** Cambiar un schema rompe el codegen en compile-time, no en runtime con queries que retornan campos incorrectos.
- **Pedagógico para Go-newcomer.** Aprender Go + SQL es más útil que aprender Go + un ORM mágico que oculta el SQL.
- **Performance.** Sin overhead de ORM, sin "N+1 hidden" por lazy loading mágico.
- **Migraciones separadas.** Schema (migraciones goose) y queries (sqlc) viven en archivos distintos — claridad.

### Negativas

- **Ciclo de cambio explícito.** Editar schema → `sqlc generate` → revisar diffs → commit. Más pasos que un ORM auto-magic. Mitigación: script `scripts/generate.ps1` automatiza.
- **Sin "active record" patterns.** Si querés `user.Save()` mágico, sqlc no te lo da. Hay que escribir `queries.UpdateUser(ctx, params)`. Aceptable.
- **Codegen errors a veces crípticos.** Bug encontrado en Fase 2: override de `jsonb` nullable producía Go inválido. Fixeado con `db_type: "pg_catalog.jsonb"` (notación cualificada).

### Neutrales

- El código generado vive en repo (`internal/store/`) y se commitea — facilita code review.
- Mocks de tests: sqlc emite una `Querier` interface, fácil de mockear.

## Alternatives considered

### `database/sql` pelado + pgx
- ✅ Cero codegen, control absoluto.
- ❌ Boilerplate enorme: cada query manual con `rows.Scan(&a, &b, &c, ...)`.
- ❌ Drift entre schema y código: un campo que se renombra solo se detecta en runtime.
- ❌ Para 14 tablas se vuelve tedioso fast.

### GORM
- ✅ Más popular en Go, mucha documentación.
- ❌ Active-record pattern oculta el SQL — los N+1 son fáciles de generar sin notarlo.
- ❌ Migration system propio compite con goose / fly.
- ❌ Acopla los modelos al ORM (hooks, lifecycle, etc.).
- ❌ Tags `gorm:""` en los structs ensucian el modelo.
- **Lo descartamos** porque queremos SQL explícito y modelos limpios.

### ent
- ✅ Codegen muy completo, modelo de grafo bonito.
- ✅ Lo usa Facebook a escala.
- ❌ Curva de aprendizaje alta — su DSL propio para definir entidades.
- ❌ Querer hacer una query custom requiere bajar a SQL pelado igual.
- ❌ Para un proyecto chico/medio es overkill.
- **Lo descartamos** porque sqlc cubre nuestro caso con menos curva.

### SQLBoiler
- ✅ Codegen desde DB existente — similar a sqlc al revés.
- ❌ Menos mantenido que sqlc actualmente.
- ❌ Genera más código (CRUD completo) — más código a leer.
- **Lo descartamos** principalmente por menos momentum del proyecto.

## Implementation notes

### Layout

```
apps/api/
├─ migrations/        ← goose: schema evolution
│  └─ 0001_init.sql
├─ queries/           ← sqlc: queries tipadas
│  ├─ registries.sql
│  └─ system.sql
├─ sqlc.yaml          ← config del generador
└─ internal/store/    ← código generado (commiteado al repo)
    ├─ db.go
    ├─ models.go
    ├─ querier.go     ← interface para mockear
    ├─ registries.sql.go
    └─ system.sql.go
```

### Ciclo de cambio

```powershell
# 1. Editar schema (nueva tabla, columna, índice)
code apps/api/migrations/0002_add_foo.sql

# 2. Editar queries si hace falta
code apps/api/queries/foo.sql

# 3. Regenerar
cd apps/api
sqlc generate

# 4. Compilar — falla si las queries usan columnas que no existen
go build ./...

# 5. Aplicar migración (en dev)
goose -dir migrations postgres "$DATABASE_URL" up
```

### Gotchas conocidos

1. **JSONB override**: usar `pg_catalog.jsonb` (no solo `jsonb`) y formato string corto:
   ```yaml
   overrides:
     - db_type: "pg_catalog.jsonb"
       go_type: "encoding/json.RawMessage"
   ```
   Si se intenta con `import:`/`package:` objects → genera código con duplicate imports.

2. **`COUNT(*) FILTER (WHERE ...)`**: sqlite no lo soporta, Postgres sí. Para Memory Core (SQLite) usar `COUNT(CASE WHEN ... THEN 1 END)`.

3. **`SELECT 1::int AS ok`** para ping: sin cast explícito, sqlc emite `interface{}` por la column type ambigua.

## Cuándo se reevalúa

- Si en algún momento necesitamos modelos extremadamente complejos con grafos (multi-tabla con join automáticos) → considerar ent.
- Si sqlc deja de mantenerse o tiene bugs bloqueantes → migrar a una alternativa con código generado equivalente.

## Related

- `apps/api/sqlc.yaml` — config concreto.
- `docs/03-data-model.md` — schema completo.
- ADR-0003 — chi como router (mismo espíritu de "stdlib-first, magia mínima").
