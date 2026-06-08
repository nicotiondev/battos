-- Queries de sistema: healthchecks, logs.

-- name: PingDB :one
SELECT 1 AS ok;

-- name: InsertSystemLog :exec
INSERT INTO system_logs (id, level, source, message, context)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?);

-- name: RecentSystemLogs :many
SELECT id, level, source, message, context, created_at FROM system_logs
ORDER BY created_at DESC
LIMIT ?;
