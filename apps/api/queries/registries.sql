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
SELECT COUNT(*)::int AS total FROM agent_runtimes WHERE status IN ('detected', 'configured');

-- name: ListAgentRuntimes :many
SELECT * FROM agent_runtimes ORDER BY id;

-- name: GetAgentRuntime :one
SELECT * FROM agent_runtimes WHERE id = $1;

-- name: UpdateAgentRuntimeDetection :one
UPDATE agent_runtimes
SET status = $2, binary_path = $3, version = $4, detected_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListCLITools :many
SELECT * FROM cli_tools ORDER BY id;

-- name: UpsertCLIToolDetection :one
INSERT INTO cli_tools (
    id, name, command, kind, detected_path, version, runtime_id, status,
    risk_level, requires_auth, capabilities, last_detected_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    command = EXCLUDED.command,
    kind = EXCLUDED.kind,
    detected_path = EXCLUDED.detected_path,
    version = EXCLUDED.version,
    runtime_id = EXCLUDED.runtime_id,
    status = EXCLUDED.status,
    risk_level = EXCLUDED.risk_level,
    requires_auth = EXCLUDED.requires_auth,
    capabilities = EXCLUDED.capabilities,
    last_detected_at = NOW()
RETURNING *;

-- name: ListProviders :many
SELECT * FROM providers ORDER BY id;

-- name: GetProvider :one
SELECT * FROM providers WHERE id = $1;

-- name: UpdateProviderStatus :exec
UPDATE providers SET status = $2, last_check_at = NOW() WHERE id = $1;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY id;

-- name: CreateAgent :one
INSERT INTO agents (
    id, slug, name, role, description, runtime_id, runtime_config,
    system_prompt, allowed_tools, allowed_projects, risk_level,
    is_lead, is_meta, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false, false, $12)
RETURNING *;

-- name: ListSkills :many
SELECT * FROM skills ORDER BY id;
