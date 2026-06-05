-- Knowledge Center CRUD: workspaces, journals and managed artifacts.

-- name: CreateKnowledgeWorkspace :one
INSERT INTO knowledge_workspaces (project_id, name, layout, status, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListKnowledgeWorkspaces :many
SELECT * FROM knowledge_workspaces
WHERE status = 'active'
ORDER BY created_at DESC;

-- name: GetKnowledgeWorkspace :one
SELECT * FROM knowledge_workspaces WHERE id = $1;

-- name: CreateJournal :one
INSERT INTO journals (workspace_id, project_id, title, content, journal_date)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListJournalsByProject :many
SELECT * FROM journals
WHERE project_id = $1
ORDER BY journal_date DESC, created_at DESC;

-- name: GetJournal :one
SELECT * FROM journals WHERE id = $1;

-- name: CreateArtifact :one
INSERT INTO artifacts (
    project_id, task_id, run_id, name, kind, content, managed_path,
    external_url, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListArtifactsByProject :many
SELECT * FROM artifacts
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: GetArtifact :one
SELECT * FROM artifacts WHERE id = $1;

-- name: UpdateSkillDefinition :one
UPDATE skills
SET prompt_template = $2, lifecycle = $3, version = $4
WHERE id = $1
RETURNING *;

-- name: GetArtifactByRunAndKind :one
SELECT * FROM artifacts
WHERE run_id = $1 AND kind = $2
LIMIT 1;
