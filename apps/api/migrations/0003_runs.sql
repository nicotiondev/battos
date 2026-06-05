-- BattOS v0.1 - supervised runs and approvals.
--
-- This migration introduces the user-facing orchestration entity from ADR-0014.
-- It does not execute workloads yet; the worker and containers come later in
-- Fase 4B.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE runs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id             TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id            TEXT NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    skill_id            TEXT REFERENCES skills(id) ON DELETE SET NULL,
    runtime_adapter_id  TEXT NOT NULL REFERENCES agent_runtimes(id) ON DELETE RESTRICT,
    repository_id       TEXT,
    prompt              TEXT NOT NULL,
    requested_network   BOOLEAN NOT NULL DEFAULT false,
    network_enabled     BOOLEAN NOT NULL DEFAULT false,
    status              TEXT NOT NULL DEFAULT 'awaiting_approval'
                        CHECK (status IN ('draft', 'awaiting_approval', 'queued', 'running', 'succeeded', 'failed', 'cancelled')),
    branch_name         TEXT,
    result_summary      TEXT,
    error_message       TEXT,
    estimated_cost_usd  NUMERIC(10, 6) NOT NULL DEFAULT 0,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runs_project_created ON runs(project_id, created_at DESC);
CREATE INDEX idx_runs_task ON runs(task_id, created_at DESC);
CREATE INDEX idx_runs_status ON runs(status, created_at DESC);
CREATE INDEX idx_runs_runtime ON runs(runtime_adapter_id, created_at DESC);

CREATE TABLE run_approvals (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id      UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('execute', 'network', 'commit', 'push')),
    decision    TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    reason      TEXT,
    decided_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_run_approvals_run ON run_approvals(run_id, decided_at DESC);
CREATE INDEX idx_run_approvals_kind ON run_approvals(kind, decided_at DESC);

CREATE TABLE run_logs (
    id          BIGSERIAL PRIMARY KEY,
    run_id      UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    stream      TEXT NOT NULL CHECK (stream IN ('system', 'stdout', 'stderr')),
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_run_logs_run_created ON run_logs(run_id, created_at);

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_run_id_fkey
    FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL;

CREATE TRIGGER set_updated_at_runs BEFORE UPDATE ON runs
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS set_updated_at_runs ON runs;
ALTER TABLE artifacts DROP CONSTRAINT IF EXISTS artifacts_run_id_fkey;
DROP TABLE IF EXISTS run_logs CASCADE;
DROP TABLE IF EXISTS run_approvals CASCADE;
DROP TABLE IF EXISTS runs CASCADE;

-- +goose StatementEnd
