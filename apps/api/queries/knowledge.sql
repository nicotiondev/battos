-- Knowledge Center CRUD: workspaces, journals and managed artifacts.

-- name: CreateKnowledgeWorkspace :one
INSERT INTO knowledge_workspaces (id, project_id, name, layout, status, metadata)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?)
RETURNING id, project_id, name, layout, status, metadata, created_at, updated_at;

-- name: ListKnowledgeWorkspaces :many
SELECT id, project_id, name, layout, status, metadata, created_at, updated_at
FROM knowledge_workspaces
WHERE status = 'active'
ORDER BY created_at DESC;

-- name: GetKnowledgeWorkspace :one
SELECT id, project_id, name, layout, status, metadata, created_at, updated_at
FROM knowledge_workspaces WHERE id = ?;

-- name: CreateJournal :one
INSERT INTO journals (id, workspace_id, project_id, title, content, journal_date)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?)
RETURNING id, workspace_id, project_id, title, content, journal_date, created_at, updated_at;

-- name: ListJournalsByProject :many
SELECT id, workspace_id, project_id, title, content, journal_date, created_at, updated_at
FROM journals
WHERE project_id = ?
ORDER BY journal_date DESC, created_at DESC;

-- name: GetJournal :one
SELECT id, workspace_id, project_id, title, content, journal_date, created_at, updated_at
FROM journals WHERE id = ?;

-- name: CreateArtifact :one
INSERT INTO artifacts (
    id, project_id, task_id, run_id, name, kind, content, managed_path,
    external_url, metadata
)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, project_id, task_id, run_id, name, kind, content, managed_path,
          external_url, metadata, created_at, updated_at;

-- name: ListArtifactsByProject :many
SELECT id, project_id, task_id, run_id, name, kind, content, managed_path,
       external_url, metadata, created_at, updated_at FROM artifacts
WHERE project_id = ?
ORDER BY created_at DESC;

-- name: GetArtifact :one
SELECT id, project_id, task_id, run_id, name, kind, content, managed_path,
       external_url, metadata, created_at, updated_at FROM artifacts WHERE id = ?;

-- name: UpdateSkillDefinition :one
UPDATE skills
SET prompt_template = ?, lifecycle = ?, version = ?
WHERE id = ?
RETURNING id, slug, name, description, category, risk_level, inputs, outputs, steps,
          compatible_agents, compatible_runtimes, source, source_id, source_ref, version,
          status, prompt_template, lifecycle, created_at, updated_at;

-- name: GetArtifactByRunAndKind :one
SELECT id, project_id, task_id, run_id, name, kind, content, managed_path,
       external_url, metadata, created_at, updated_at FROM artifacts
WHERE run_id = ? AND kind = ?
LIMIT 1;

-- name: ListArtifactsByRun :many
SELECT id, project_id, task_id, run_id, name, kind, content, managed_path,
       external_url, metadata, created_at, updated_at FROM artifacts
WHERE run_id = ?
ORDER BY created_at DESC;
