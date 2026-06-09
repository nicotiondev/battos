# postgres-archive

These are the historical PostgreSQL/goose migration files from BattOS v0.1's
original Postgres-backed design. They are **no longer applied at boot** and
cannot run against SQLite.

## Current boot path

Since ADR-0021 (SQLite unificado), the live schema is bootstrapped from:

```
apps/api/internal/store/sqlite_schema.sql
```

`OpenDB` (in `apps/api/internal/store/pool.go`) embeds and applies that file
idempotently on every start — no migration runner is involved.

## Why these are kept

Historical reference only. They show the original table design before the
Postgres → SQLite migration. Do not edit, do not run them.
