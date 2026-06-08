-- Repositories CRUD

-- name: CreateRepository :one
INSERT INTO repositories (id, project_id, kind, name, remote_url, credential_ref, default_branch, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, project_id, kind, name, remote_url, credential_ref, default_branch,
          metadata, created_at, updated_at;

-- name: ListRepositoriesByProject :many
SELECT id, project_id, kind, name, remote_url, credential_ref, default_branch,
       metadata, created_at, updated_at FROM repositories
WHERE project_id = ?
ORDER BY created_at DESC;

-- name: ListRepositories :many
SELECT id, project_id, kind, name, remote_url, credential_ref, default_branch,
       metadata, created_at, updated_at FROM repositories
ORDER BY created_at DESC;

-- name: GetRepository :one
SELECT id, project_id, kind, name, remote_url, credential_ref, default_branch,
       metadata, created_at, updated_at FROM repositories WHERE id = ?;

-- name: DeleteRepository :one
DELETE FROM repositories WHERE id = ?
RETURNING id, project_id, kind, name, remote_url, credential_ref, default_branch,
          metadata, created_at, updated_at;
