-- Repositories CRUD

-- name: CreateRepository :one
INSERT INTO repositories (id, project_id, kind, name, remote_url, credential_ref, default_branch, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListRepositoriesByProject :many
SELECT * FROM repositories
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY created_at DESC;

-- name: GetRepository :one
SELECT * FROM repositories WHERE id = $1;

-- name: DeleteRepository :one
DELETE FROM repositories WHERE id = $1
RETURNING *;
