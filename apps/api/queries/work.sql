-- Work Board CRUD: domains, projects, goals and tasks.

-- name: CreateDomain :one
INSERT INTO domains (id, slug, name, description, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDomains :many
SELECT * FROM domains
WHERE status != 'archived'
ORDER BY created_at DESC;

-- name: GetDomain :one
SELECT * FROM domains WHERE id = $1;

-- name: UpdateDomain :one
UPDATE domains
SET name = $2, description = $3, status = $4, metadata = $5
WHERE id = $1
RETURNING *;

-- name: CreateProject :one
INSERT INTO projects (id, slug, name, description, domain_id, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListProjects :many
SELECT * FROM projects
WHERE status != 'archived'
ORDER BY created_at DESC;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, description = $3, domain_id = $4, status = $5, metadata = $6
WHERE id = $1
RETURNING *;

-- name: CreateGoal :one
INSERT INTO goals (project_id, title, description, status, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGoalsByProject :many
SELECT * FROM goals
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: GetGoal :one
SELECT * FROM goals WHERE id = $1;

-- name: UpdateGoal :one
UPDATE goals
SET title = $2, description = $3, status = $4, metadata = $5
WHERE id = $1
RETURNING *;

-- name: CreateTask :one
INSERT INTO tasks (
    project_id, goal_id, title, description, assigned_agent_id, status,
    board_position, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListTasksByProject :many
SELECT * FROM tasks
WHERE project_id = $1
ORDER BY status, board_position, created_at DESC;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: UpdateTask :one
UPDATE tasks
SET goal_id = $2, title = $3, description = $4, assigned_agent_id = $5,
    status = $6, board_position = $7, metadata = $8
WHERE id = $1
RETURNING *;
