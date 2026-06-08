-- Audit log queries para registro de acciones administrativas.

-- name: CreateAuditLog :one
INSERT INTO audit_logs (id, action, actor, target_type, target_id, details, ip_address)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?)
RETURNING id, action, actor, target_type, target_id, details, ip_address, created_at;

-- name: ListAuditLogs :many
SELECT id, action, actor, target_type, target_id, details, ip_address, created_at
FROM audit_logs
ORDER BY created_at DESC
LIMIT ?;

-- name: ListAuditLogsByAction :many
SELECT id, action, actor, target_type, target_id, details, ip_address, created_at
FROM audit_logs
WHERE action = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListAuditLogsByActor :many
SELECT id, action, actor, target_type, target_id, details, ip_address, created_at
FROM audit_logs
WHERE actor = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: CountAuditLogs :one
SELECT COUNT(*) AS total FROM audit_logs;
