-- +goose Up
-- +goose StatementBegin
INSERT INTO agents (id, slug, name, role, description, runtime_id, is_lead, is_meta, status)
VALUES ('novacore', 'novacore', 'NovaCore', 'System Orchestrator', 'Asistente de sistema de BattOS que te ayuda a administrar y diagnosticar el OS.', 'direct-api', true, true, 'active')
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM agents WHERE id = 'novacore';
-- +goose StatementEnd
