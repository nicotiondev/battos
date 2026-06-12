-- Queries for basic registries.
-- Solo lectura + counts en Fase 2. CRUD completo viene en Fase 3.

-- name: CountProjects :one
SELECT COUNT(*) AS total FROM projects WHERE status != 'archived';

-- name: CountAgents :one
SELECT COUNT(*) AS total FROM agents WHERE status = 'active';

-- name: CountSkills :one
SELECT COUNT(*) AS total FROM skills WHERE status = 'active';

-- name: CountActiveMCPConnections :one
SELECT COUNT(*) AS total FROM mcp_connections WHERE status = 'active';

-- name: CountDetectedCLITools :one
SELECT COUNT(*) AS total FROM cli_tools WHERE status = 'detected';

-- name: CountAvailableRuntimes :one
SELECT COUNT(*) AS total FROM agent_runtimes WHERE status IN ('detected', 'configured');

-- name: ListAgentRuntimes :many
SELECT id, name, kind, status, binary_path, version, endpoint_url, risk_level,
       requires_auth, capabilities, config_schema, detected_at, created_at, updated_at
FROM agent_runtimes ORDER BY id;

-- name: GetAgentRuntime :one
SELECT id, name, kind, status, binary_path, version, endpoint_url, risk_level,
       requires_auth, capabilities, config_schema, detected_at, created_at, updated_at
FROM agent_runtimes WHERE id = ?;

-- name: UpdateAgentRuntimeDetection :one
UPDATE agent_runtimes
SET status = ?, binary_path = ?, version = ?, detected_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, name, kind, status, binary_path, version, endpoint_url, risk_level,
          requires_auth, capabilities, config_schema, detected_at, created_at, updated_at;

-- name: ListCLITools :many
SELECT id, name, command, kind, detected_path, version, runtime_id, status,
       risk_level, requires_auth, capabilities, install_command, install_url,
       last_detected_at, created_at, updated_at
FROM cli_tools ORDER BY id;

-- name: GetCLITool :one
SELECT id, name, command, kind, detected_path, version, runtime_id, status,
       risk_level, requires_auth, capabilities, install_command, install_url,
       last_detected_at, created_at, updated_at
FROM cli_tools WHERE id = ?;

-- name: UpsertCLIToolDetection :one
INSERT INTO cli_tools (
    id, name, command, kind, detected_path, version, runtime_id, status,
    risk_level, requires_auth, capabilities, install_command, install_url, last_detected_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
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
    install_command = EXCLUDED.install_command,
    install_url = EXCLUDED.install_url,
    last_detected_at = CURRENT_TIMESTAMP
RETURNING id, name, command, kind, detected_path, version, runtime_id, status,
          risk_level, requires_auth, capabilities, install_command, install_url,
          last_detected_at, created_at, updated_at;

-- Instalacion gobernada de CLIs (Etapa 2): el request queda pending_approval y
-- solo se ejecuta tras un approve explicito (mismo modelo HITL que run_approvals).
-- NOTA: sin acentos en comentarios de queries -- sqlc desplaza offsets con
-- caracteres multibyte y trunca el SQL generado.

-- name: CreateCLIToolInstall :one
INSERT INTO cli_tool_installs (id, cli_tool_id, install_command)
VALUES (lower(hex(randomblob(16))), ?, ?)
RETURNING id, cli_tool_id, install_command, status, reason, output,
          requested_at, decided_at, completed_at;

-- name: GetCLIToolInstall :one
SELECT id, cli_tool_id, install_command, status, reason, output,
       requested_at, decided_at, completed_at
FROM cli_tool_installs WHERE id = ?;

-- name: ListCLIToolInstalls :many
SELECT id, cli_tool_id, install_command, status, reason, output,
       requested_at, decided_at, completed_at
FROM cli_tool_installs
WHERE cli_tool_id = ?
ORDER BY requested_at DESC, id;

-- name: DecideCLIToolInstall :one
UPDATE cli_tool_installs
SET status = ?, reason = ?, decided_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 'pending_approval'
RETURNING id, cli_tool_id, install_command, status, reason, output,
          requested_at, decided_at, completed_at;

-- name: FinishCLIToolInstall :one
UPDATE cli_tool_installs
SET status = ?, output = ?, completed_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 'running'
RETURNING id, cli_tool_id, install_command, status, reason, output,
          requested_at, decided_at, completed_at;

-- name: ListProviders :many
SELECT id, name, kind, env_key, docs_url, status, monthly_budget_usd,
       monthly_spend_usd, last_check_at, created_at, updated_at
FROM providers ORDER BY id;

-- name: GetProvider :one
SELECT id, name, kind, env_key, docs_url, status, monthly_budget_usd,
       monthly_spend_usd, last_check_at, created_at, updated_at
FROM providers WHERE id = ?;

-- name: UpdateProviderStatus :exec
UPDATE providers SET status = ?, last_check_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ListAgents :many
SELECT id, slug, name, role, description, runtime_id, runtime_config, system_prompt,
       allowed_tools, allowed_projects, risk_level, is_lead, is_meta, status,
       created_at, updated_at
FROM agents ORDER BY id;

-- name: CreateAgent :one
INSERT INTO agents (
    id, slug, name, role, description, runtime_id, runtime_config,
    system_prompt, allowed_tools, allowed_projects, risk_level,
    is_lead, is_meta, status
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?)
RETURNING id, slug, name, role, description, runtime_id, runtime_config, system_prompt,
          allowed_tools, allowed_projects, risk_level, is_lead, is_meta, status,
          created_at, updated_at;

-- name: ListSkills :many
SELECT id, slug, name, description, category, risk_level, inputs, outputs, steps,
       compatible_agents, compatible_runtimes, source, source_id, source_ref, version,
       status, prompt_template, lifecycle, created_at, updated_at
FROM skills ORDER BY id;
