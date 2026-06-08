-- Run proposal and approval queries.

-- name: CreateRun :one
INSERT INTO runs (
    id, project_id, task_id, agent_id, skill_id, runtime_adapter_id, repository_id,
    prompt, requested_network, network_enabled, host_session_enabled, status, metadata
)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 'awaiting_approval', '{}')
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: ListRuns :many
SELECT id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
       repository_id, prompt, requested_network, network_enabled, host_session_enabled,
       status, branch_name, result_summary, error_message, estimated_cost_usd,
       metadata, started_at, completed_at, created_at, updated_at FROM runs
ORDER BY created_at DESC;

-- name: ListRunsByProject :many
SELECT id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
       repository_id, prompt, requested_network, network_enabled, host_session_enabled,
       status, branch_name, result_summary, error_message, estimated_cost_usd,
       metadata, started_at, completed_at, created_at, updated_at FROM runs
WHERE project_id = ?
ORDER BY created_at DESC;

-- name: GetRun :one
SELECT id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
       repository_id, prompt, requested_network, network_enabled, host_session_enabled,
       status, branch_name, result_summary, error_message, estimated_cost_usd,
       metadata, started_at, completed_at, created_at, updated_at FROM runs WHERE id = ?;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = ?
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: ClaimNextQueuedRun :one
UPDATE runs
SET status = 'running',
    started_at = CURRENT_TIMESTAMP
WHERE id = (
    SELECT id
    FROM runs
    WHERE status = 'queued'
    ORDER BY created_at ASC
    LIMIT 1
)
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: ClaimQueuedRunByID :one
UPDATE runs
SET status = 'running',
    started_at = CURRENT_TIMESTAMP
WHERE id = ?
  AND status = 'queued'
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: EnableRunNetwork :one
UPDATE runs
SET network_enabled = 1
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: EnableRunHostSession :one
UPDATE runs
SET host_session_enabled = 1
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: CancelRun :one
UPDATE runs
SET status = 'cancelled',
    completed_at = COALESCE(completed_at, CURRENT_TIMESTAMP)
WHERE id = ?
  AND status IN ('draft', 'awaiting_approval', 'queued', 'running')
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: CompleteRun :one
UPDATE runs
SET status = 'succeeded',
    result_summary = ?,
    completed_at = CURRENT_TIMESTAMP,
    error_message = NULL
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: FailRun :one
UPDATE runs
SET status = 'failed',
    result_summary = ?,
    error_message = ?,
    completed_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;

-- name: AppendRunLog :one
INSERT INTO run_logs (run_id, stream, message)
VALUES (?, ?, ?)
RETURNING id, run_id, stream, message, created_at;

-- name: CreateRunApproval :one
INSERT INTO run_approvals (id, run_id, kind, decision, reason)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?)
RETURNING id, run_id, kind, decision, reason, decided_at;

-- name: ListRunLogs :many
SELECT id, run_id, stream, message, created_at FROM run_logs
WHERE run_id = ?
ORDER BY created_at ASC;

-- name: UpdateRunBranchAndMetadata :one
UPDATE runs
SET branch_name = ?,
    metadata = ?
WHERE id = ?
RETURNING id, project_id, task_id, agent_id, skill_id, runtime_adapter_id,
          repository_id, prompt, requested_network, network_enabled, host_session_enabled,
          status, branch_name, result_summary, error_message, estimated_cost_usd,
          metadata, started_at, completed_at, created_at, updated_at;
