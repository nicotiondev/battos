-- BattOS v0.1 - Work Board and Knowledge Center persistence.
--
-- Append-only extension of 0001_init.sql. No execution engine is introduced
-- here; runs and approvals are added in their own later migration.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE domains (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'paused', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE projects
    ADD COLUMN domain_id TEXT REFERENCES domains(id) ON DELETE SET NULL;
CREATE INDEX idx_projects_domain ON projects(domain_id);

CREATE TABLE goals (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'planned'
                    CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_goals_project ON goals(project_id, created_at DESC);
CREATE INDEX idx_goals_status ON goals(status);

CREATE TABLE tasks (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    goal_id             TEXT REFERENCES goals(id) ON DELETE SET NULL,
    title               TEXT NOT NULL,
    description         TEXT,
    assigned_agent_id   TEXT REFERENCES agents(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'backlog'
                        CHECK (status IN ('backlog', 'ready', 'in_progress', 'review', 'done', 'cancelled')),
    board_position      INTEGER NOT NULL DEFAULT 0,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status, board_position);
CREATE INDEX idx_tasks_goal ON tasks(goal_id);
CREATE INDEX idx_tasks_agent ON tasks(assigned_agent_id);

CREATE TABLE knowledge_workspaces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    layout          TEXT NOT NULL DEFAULT 'raw_wiki_outputs'
                    CHECK (layout IN ('raw_wiki_outputs')),
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE journals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES knowledge_workspaces(id) ON DELETE CASCADE,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    content         TEXT NOT NULL,
    journal_date    DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journals_project_date ON journals(project_id, journal_date DESC);
CREATE INDEX idx_journals_workspace ON journals(workspace_id, created_at DESC);

CREATE TABLE artifacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id         TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    run_id          UUID,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL
                    CHECK (kind IN ('markdown', 'image', 'link', 'diff', 'build_report')),
    content         TEXT,
    managed_path    TEXT,
    external_url    TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        content IS NOT NULL OR managed_path IS NOT NULL OR external_url IS NOT NULL
    )
);

CREATE INDEX idx_artifacts_project ON artifacts(project_id, created_at DESC);
CREATE INDEX idx_artifacts_task ON artifacts(task_id, created_at DESC);
CREATE INDEX idx_artifacts_run ON artifacts(run_id, created_at DESC);

ALTER TABLE skills
    ADD COLUMN prompt_template TEXT,
    ADD COLUMN lifecycle TEXT NOT NULL DEFAULT 'active'
               CHECK (lifecycle IN ('draft', 'active', 'deprecated', 'disabled'));

CREATE TRIGGER set_updated_at_domains BEFORE UPDATE ON domains
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_goals BEFORE UPDATE ON goals
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_tasks BEFORE UPDATE ON tasks
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_knowledge_workspaces BEFORE UPDATE ON knowledge_workspaces
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_journals BEFORE UPDATE ON journals
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_artifacts BEFORE UPDATE ON artifacts
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS set_updated_at_artifacts ON artifacts;
DROP TRIGGER IF EXISTS set_updated_at_journals ON journals;
DROP TRIGGER IF EXISTS set_updated_at_knowledge_workspaces ON knowledge_workspaces;
DROP TRIGGER IF EXISTS set_updated_at_tasks ON tasks;
DROP TRIGGER IF EXISTS set_updated_at_goals ON goals;
DROP TRIGGER IF EXISTS set_updated_at_domains ON domains;

ALTER TABLE skills
    DROP COLUMN IF EXISTS lifecycle,
    DROP COLUMN IF EXISTS prompt_template;

DROP TABLE IF EXISTS artifacts CASCADE;
DROP TABLE IF EXISTS journals CASCADE;
DROP TABLE IF EXISTS knowledge_workspaces CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;
DROP TABLE IF EXISTS goals CASCADE;

DROP INDEX IF EXISTS idx_projects_domain;
ALTER TABLE projects DROP COLUMN IF EXISTS domain_id;
DROP TABLE IF EXISTS domains CASCADE;

-- +goose StatementEnd
