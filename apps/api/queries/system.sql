-- Queries de sistema: healthchecks, logs.

-- name: PingDB :one
SELECT 1::int AS ok;

-- name: InsertSystemLog :exec
INSERT INTO system_logs (level, source, message, context)
VALUES ($1, $2, $3, $4);

-- name: RecentSystemLogs :many
SELECT * FROM system_logs
ORDER BY created_at DESC
LIMIT $1;
