-- +goose Up
ALTER TABLE run_approvals DROP CONSTRAINT IF EXISTS run_approvals_kind_check;
ALTER TABLE run_approvals ADD CONSTRAINT run_approvals_kind_check CHECK (kind IN ('execute', 'network', 'commit', 'push', 'remember'));

-- +goose Down
ALTER TABLE run_approvals DROP CONSTRAINT IF EXISTS run_approvals_kind_check;
ALTER TABLE run_approvals ADD CONSTRAINT run_approvals_kind_check CHECK (kind IN ('execute', 'network', 'commit', 'push'));
