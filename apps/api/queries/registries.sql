-- Queries para registries básicos.
-- Solo lectura + counts en Fase 2. CRUD completo viene en Fase 3.

-- name: CountProjects :one
SELECT COUNT(*)::int AS total FROM projects WHERE status != 'archived';

-- name: CountAgents :one
SELECT COUNT(*)::int AS total FROM agents WHERE status = 'active';

-- name: CountSkills :one
SELECT COUNT(*)::int AS total FROM skills WHERE status = 'active';

-- name: CountActiveMCPConnections :one
SELECT COUNT(*)::int AS total FROM mcp_connections WHERE status = 'active';

-- name: CountDetectedCLITools :one
SELECT COUNT(*)::int AS total FROM cli_tools WHERE status = 'detected';

-- name: CountAvailableRuntimes :one
SELECT COUNT(*)::int AS total FROM agent_runtimes WHERE status = 'available';

-- name: ListAgentRuntimes :many
SELECT * FROM agent_runtimes ORDER BY id;

-- name: GetAgentRuntime :one
SELECT * FROM agent_runtimes WHERE id = $1;

-- name: ListProviders :many
SELECT * FROM providers ORDER BY id;

-- name: GetProvider :one
SELECT * FROM providers WHERE id = $1;

-- name: UpdateProviderStatus :exec
UPDATE providers SET status = $2, last_check_at = NOW() WHERE id = $1;
