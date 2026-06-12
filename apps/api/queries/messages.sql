-- name: CreateAgentMessage :one
INSERT INTO agent_messages (id, project_id, from_agent_id, to_agent_id, run_id, subject, body)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?)
RETURNING id, project_id, from_agent_id, to_agent_id, run_id, subject, body, read_at, created_at;

-- name: ListInboxForAgent :many
SELECT id, project_id, from_agent_id, to_agent_id, run_id, subject, body, read_at, created_at
FROM agent_messages
WHERE to_agent_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListUnreadInboxForAgent :many
SELECT id, project_id, from_agent_id, to_agent_id, run_id, subject, body, read_at, created_at
FROM agent_messages
WHERE to_agent_id = ? AND read_at IS NULL
ORDER BY created_at ASC;

-- name: MarkAgentMessageRead :one
UPDATE agent_messages
SET read_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, project_id, from_agent_id, to_agent_id, run_id, subject, body, read_at, created_at;

-- name: CountUnreadForAgent :one
SELECT COUNT(*) FROM agent_messages
WHERE to_agent_id = ? AND read_at IS NULL;
