-- name: CreateCredential :one
INSERT INTO credentials (id, name, kind, provider_id, secret_source, secret_locator, description)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?)
RETURNING id, name, kind, provider_id, secret_source, secret_locator, description, created_at, updated_at;

-- name: GetCredentialByName :one
SELECT id, name, kind, provider_id, secret_source, secret_locator, description, created_at, updated_at
FROM credentials
WHERE name = ?;

-- name: ListCredentials :many
SELECT id, name, kind, provider_id, secret_source, secret_locator, description, created_at, updated_at
FROM credentials
ORDER BY name ASC;

-- name: DeleteCredential :exec
DELETE FROM credentials WHERE name = ?;
