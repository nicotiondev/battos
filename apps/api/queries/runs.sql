-- Run proposal and approval queries.

-- name: CreateRun :one
INSERT INTO runs (
    project_id, task_id, agent_id, skill_id, runtime_adapter_id, repository_id,
    prompt, requested_network, network_enabled, status, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, 'awaiting_approval', '{}'::jsonb)
RETURNING *;

-- name: ListRuns :many
SELECT * FROM runs
ORDER BY created_at DESC;

-- name: ListRunsByProject :many
SELECT * FROM runs
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: GetRun :one
SELECT * FROM runs WHERE id = $1;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = $2
WHERE id = $1
RETURNING *;

-- name: ClaimNextQueuedRun :one
UPDATE runs
SET status = 'running',
    started_at = NOW()
WHERE id = (
    SELECT id
    FROM runs
    WHERE status = 'queued'
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: ClaimQueuedRunByID :one
UPDATE runs
SET status = 'running',
    started_at = NOW()
WHERE id = $1
  AND status = 'queued'
RETURNING *;

-- name: EnableRunNetwork :one
UPDATE runs
SET network_enabled = true
WHERE id = $1
RETURNING *;

-- name: CancelRun :one
UPDATE runs
SET status = 'cancelled',
    completed_at = COALESCE(completed_at, NOW())
WHERE id = $1
  AND status IN ('draft', 'awaiting_approval', 'queued', 'running')
RETURNING *;

-- name: CompleteRun :one
UPDATE runs
SET status = 'succeeded',
    result_summary = $2,
    completed_at = NOW(),
    error_message = NULL
WHERE id = $1
RETURNING *;

-- name: FailRun :one
UPDATE runs
SET status = 'failed',
    result_summary = $2,
    error_message = $3,
    completed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: AppendRunLog :one
INSERT INTO run_logs (run_id, stream, message)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateRunApproval :one
INSERT INTO run_approvals (run_id, kind, decision, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListRunLogs :many
SELECT * FROM run_logs
WHERE run_id = $1
ORDER BY created_at ASC;

-- name: UpdateRunBranchAndMetadata :one
UPDATE runs
SET branch_name = $2,
    metadata = $3
WHERE id = $1
RETURNING *;
