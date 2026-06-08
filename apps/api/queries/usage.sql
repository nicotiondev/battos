-- Queries para eventos de consumo y uso (usage_events)

-- name: CreateUsageEvent :one
INSERT INTO usage_events (
    id, run_id, provider_id, model_id, project_id, agent_id, skill_id,
    input_tokens, output_tokens, cached_tokens, request_count, estimated_cost_usd
)
VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, run_id, provider_id, model_id, project_id, agent_id, skill_id,
          input_tokens, output_tokens, cached_tokens, request_count,
          estimated_cost_usd, created_at;

-- name: GetUsageOverview :many
SELECT 
    ue.project_id,
    COALESCE(p.name, ue.project_id) AS project_name,
    p.monthly_budget_usd AS project_monthly_budget_usd,
    ue.agent_id,
    ue.model_id,
    ue.provider_id,
    SUM(ue.input_tokens) AS total_input_tokens,
    SUM(ue.output_tokens) AS total_output_tokens,
    SUM(ue.cached_tokens) AS total_cached_tokens,
    SUM(ue.request_count) AS total_requests,
    COALESCE(SUM(ue.estimated_cost_usd), 0) AS total_cost_usd
FROM usage_events ue
LEFT JOIN projects p ON p.slug = ue.project_id OR p.id = ue.project_id
GROUP BY ue.project_id, p.name, p.monthly_budget_usd, ue.agent_id, ue.model_id, ue.provider_id
ORDER BY total_cost_usd DESC;

-- name: GetUsageByRun :many
SELECT id, run_id, provider_id, model_id, project_id, agent_id, skill_id,
       input_tokens, output_tokens, cached_tokens, request_count,
       estimated_cost_usd, created_at
FROM usage_events WHERE run_id = ? ORDER BY created_at DESC;
