-- Queries para eventos de consumo y uso (usage_events)

-- name: CreateUsageEvent :one
INSERT INTO usage_events (
    run_id, provider_id, model_id, project_id, agent_id, skill_id,
    input_tokens, output_tokens, cached_tokens, request_count, estimated_cost_usd
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetUsageOverview :many
SELECT 
    ue.project_id,
    COALESCE(p.name, ue.project_id) AS project_name,
    p.monthly_budget_usd AS project_monthly_budget_usd,
    ue.agent_id,
    ue.model_id,
    ue.provider_id,
    SUM(ue.input_tokens)::bigint AS total_input_tokens,
    SUM(ue.output_tokens)::bigint AS total_output_tokens,
    SUM(ue.cached_tokens)::bigint AS total_cached_tokens,
    SUM(ue.request_count)::bigint AS total_requests,
    COALESCE(SUM(ue.estimated_cost_usd), 0)::numeric(10, 6) AS total_cost_usd
FROM usage_events ue
LEFT JOIN projects p ON p.slug = ue.project_id OR p.id::text = ue.project_id
GROUP BY ue.project_id, p.name, p.monthly_budget_usd, ue.agent_id, ue.model_id, ue.provider_id
ORDER BY total_cost_usd DESC;

-- name: GetUsageByRun :many
SELECT * FROM usage_events WHERE run_id = $1 ORDER BY created_at DESC;
