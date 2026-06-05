-- BattOS v0.1 - Repositories migration.
--
-- This migration introduces the repositories configuration.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE repositories (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('managed_local', 'github')),
    name            TEXT NOT NULL,
    remote_url      TEXT,
    credential_ref  TEXT,
    default_branch  TEXT NOT NULL DEFAULT 'master',
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_repositories_project ON repositories(project_id, created_at DESC);
CREATE INDEX idx_repositories_kind ON repositories(kind);

-- Modificar tabla runs para añadir la clave foránea
ALTER TABLE runs
    ADD CONSTRAINT runs_repository_id_fkey
    FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE SET NULL;

CREATE TRIGGER set_updated_at_repositories BEFORE UPDATE ON repositories
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS set_updated_at_repositories ON repositories;
ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_repository_id_fkey;
DROP TABLE IF EXISTS repositories CASCADE;

-- +goose StatementEnd
