-- Audit log queries para registro de acciones administrativas.

-- name: CreateAuditLog :one
INSERT INTO audit_logs (action, actor, target_type, target_id, details, ip_address)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC
LIMIT $1;

-- name: ListAuditLogsByAction :many
SELECT * FROM audit_logs
WHERE action = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListAuditLogsByActor :many
SELECT * FROM audit_logs
WHERE actor = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountAuditLogs :one
SELECT COUNT(*)::int AS total FROM audit_logs;
