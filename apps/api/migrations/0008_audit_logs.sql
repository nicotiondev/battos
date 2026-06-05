-- +goose Up
-- +goose StatementBegin
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action      TEXT NOT NULL,       -- e.g. 'run_approved', 'push_approved', 'backup_created', 'repo_connected'
    actor       TEXT NOT NULL,       -- e.g. 'admin', 'novacore', 'system'
    target_type TEXT NOT NULL DEFAULT '',  -- e.g. 'run', 'repository', 'project', 'backup'
    target_id   TEXT NOT NULL DEFAULT '',  -- e.g. run UUID, project slug
    details     JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_action    ON audit_logs(action, created_at DESC);
CREATE INDEX idx_audit_logs_actor     ON audit_logs(actor, created_at DESC);
CREATE INDEX idx_audit_logs_target    ON audit_logs(target_type, target_id);
CREATE INDEX idx_audit_logs_created   ON audit_logs(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
-- +goose StatementEnd
