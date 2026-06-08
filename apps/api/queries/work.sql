-- Work Board CRUD: domains, projects, goals and tasks.

-- name: CreateDomain :one
INSERT INTO domains (id, slug, name, description, status, metadata)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, slug, name, description, status, metadata, created_at, updated_at;

-- name: ListDomains :many
SELECT id, slug, name, description, status, metadata, created_at, updated_at FROM domains
WHERE status != 'archived'
ORDER BY created_at DESC;

-- name: GetDomain :one
SELECT id, slug, name, description, status, metadata, created_at, updated_at FROM domains WHERE id = ?;

-- name: UpdateDomain :one
UPDATE domains
SET name = ?, description = ?, status = ?, metadata = ?
WHERE id = ?
RETURNING id, slug, name, description, status, metadata, created_at, updated_at;

-- name: CreateProject :one
INSERT INTO projects (id, slug, name, description, domain_id, status, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, slug, name, description, status, owner_agent_id, monthly_budget_usd,
          metadata, domain_id, created_at, updated_at;

-- name: ListProjects :many
SELECT id, slug, name, description, status, owner_agent_id, monthly_budget_usd,
       metadata, domain_id, created_at, updated_at FROM projects
WHERE status != 'archived'
ORDER BY created_at DESC;

-- name: EnsureInboxProject :one
INSERT INTO projects (id, slug, name, description, status, metadata)
VALUES (
    'inbox',
    'inbox',
    'Inbox',
    'Captura temporal para tareas sin proyecto asignado',
    'active',
    '{"system":true,"purpose":"task_inbox"}'
)
ON CONFLICT (id) DO UPDATE
SET updated_at = projects.updated_at
RETURNING id, slug, name, description, status, owner_agent_id, monthly_budget_usd,
          metadata, domain_id, created_at, updated_at;

-- name: GetProject :one
SELECT id, slug, name, description, status, owner_agent_id, monthly_budget_usd,
       metadata, domain_id, created_at, updated_at FROM projects WHERE id = ?;

-- name: UpdateProject :one
UPDATE projects
SET name = ?, description = ?, domain_id = ?, status = ?, metadata = ?
WHERE id = ?
RETURNING id, slug, name, description, status, owner_agent_id, monthly_budget_usd,
          metadata, domain_id, created_at, updated_at;

-- name: CreateGoal :one
INSERT INTO goals (id, project_id, title, description, status, metadata)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?)
RETURNING id, project_id, title, description, status, metadata, created_at, updated_at;

-- name: ListGoalsByProject :many
SELECT id, project_id, title, description, status, metadata, created_at, updated_at FROM goals
WHERE project_id = ?
ORDER BY created_at DESC;

-- name: ListGoals :many
SELECT id, project_id, title, description, status, metadata, created_at, updated_at FROM goals
ORDER BY project_id, created_at DESC;

-- name: GetGoal :one
SELECT id, project_id, title, description, status, metadata, created_at, updated_at FROM goals WHERE id = ?;

-- name: UpdateGoal :one
UPDATE goals
SET title = ?, description = ?, status = ?, metadata = ?
WHERE id = ?
RETURNING id, project_id, title, description, status, metadata, created_at, updated_at;

-- name: CreateTask :one
INSERT INTO tasks (
    id, project_id, goal_id, title, description, assigned_agent_id, status,
    board_position, metadata
)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, project_id, goal_id, title, description, assigned_agent_id, status,
          board_position, metadata, created_at, updated_at;

-- name: ListTasksByProject :many
SELECT id, project_id, goal_id, title, description, assigned_agent_id, status,
       board_position, metadata, created_at, updated_at FROM tasks
WHERE project_id = ?
ORDER BY status, board_position, created_at DESC;

-- name: ListTasks :many
SELECT id, project_id, goal_id, title, description, assigned_agent_id, status,
       board_position, metadata, created_at, updated_at FROM tasks
ORDER BY project_id, status, board_position, created_at DESC;

-- name: GetTask :one
SELECT id, project_id, goal_id, title, description, assigned_agent_id, status,
       board_position, metadata, created_at, updated_at FROM tasks WHERE id = ?;

-- name: UpdateTask :one
UPDATE tasks
SET project_id = ?, goal_id = ?, title = ?, description = ?, assigned_agent_id = ?,
    status = ?, board_position = ?, metadata = ?
WHERE id = ?
RETURNING id, project_id, goal_id, title, description, assigned_agent_id, status,
          board_position, metadata, created_at, updated_at;
