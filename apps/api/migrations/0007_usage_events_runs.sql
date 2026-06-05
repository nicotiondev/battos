-- +goose Up
-- +goose StatementBegin
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_execution_id_fkey;
ALTER TABLE usage_events RENAME COLUMN execution_id TO run_id;
ALTER TABLE usage_events ADD CONSTRAINT usage_events_run_id_fkey FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_run_id_fkey;
ALTER TABLE usage_events RENAME COLUMN run_id TO execution_id;
ALTER TABLE usage_events ADD CONSTRAINT usage_events_execution_id_fkey FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE;
-- +goose StatementEnd
