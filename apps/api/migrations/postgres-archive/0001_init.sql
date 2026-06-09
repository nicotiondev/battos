-- BattOS v0.1 — Schema inicial.
--
-- Decisiones:
--   - IDs como TEXT (slug-friendly) en tablas user-facing (projects, agents, skills);
--     UUID/SERIAL en tablas técnicas (executions, usage_events).
--   - JSONB para campos flexibles (runtime_config, allowed_tools, capabilities).
--   - TIMESTAMPTZ para todo lo temporal (sin sufrir timezones).
--   - Sin foreign keys "duras" entre projects/agents/skills al inicio (slugs son lookups);
--     SÍ FKs entre tablas de logs/executions/usage para mantener consistencia.
--
-- Lienzo en blanco (ADR-0008): NO seedeamos projects, agents ni skills.
-- Solo poblamos agent_runtimes (catálogo conocido) e idempotent rows.

-- +goose Up
-- +goose StatementBegin

-- ===========================================================================
-- 1. AGENT RUNTIMES (catálogo de motores donde corren los agentes)
-- ===========================================================================
CREATE TABLE agent_runtimes (
    id              TEXT PRIMARY KEY,        -- 'claude-code' | 'codex' | 'openclaw' | ...
    name            TEXT NOT NULL,           -- 'Claude Code CLI'
    kind            TEXT NOT NULL,           -- 'subprocess' | 'http' | 'mcp' | 'webhook' | 'manual' | 'direct-api'
    status          TEXT NOT NULL DEFAULT 'unavailable',  -- 'available' | 'unavailable' | 'disabled'
    binary_path     TEXT,                    -- path al ejecutable (si kind=subprocess)
    version         TEXT,
    endpoint_url    TEXT,                    -- si kind=http/webhook
    risk_level      TEXT NOT NULL DEFAULT 'medium',
    requires_auth   BOOLEAN NOT NULL DEFAULT false,
    capabilities    JSONB NOT NULL DEFAULT '[]'::jsonb,
    config_schema   JSONB NOT NULL DEFAULT '{}'::jsonb,   -- JSON schema de runtime_config válido
    detected_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed catálogo: lista los runtimes conocidos, todos arrancan 'unavailable'
-- hasta que CLI Detector confirme. NOTA: idempotente vía ON CONFLICT DO NOTHING.
INSERT INTO agent_runtimes (id, name, kind, risk_level, requires_auth, capabilities) VALUES
  ('claude-code',  'Claude Code CLI',  'subprocess', 'high',   true,  '["code_editing","file_reading","terminal_commands","mcp"]'::jsonb),
  ('codex',        'Codex CLI',        'subprocess', 'high',   true,  '["code_generation","repo_editing","tests"]'::jsonb),
  ('opencode',     'OpenCode',         'subprocess', 'medium', true,  '["code_editing","local_agent","terminal_workflows"]'::jsonb),
  ('gemini-cli',   'Gemini CLI',       'subprocess', 'medium', true,  '["long_context","multimodal"]'::jsonb),
  ('kimi-cli',     'Kimi CLI',         'subprocess', 'medium', true,  '["long_context"]'::jsonb),
  ('qwen-cli',     'Qwen Code',        'subprocess', 'medium', true,  '["code_generation","local"]'::jsonb),
  ('aider',        'Aider',            'subprocess', 'high',   true,  '["code_editing","git_integration"]'::jsonb),
  ('openclaw',     'OpenClaw Gateway', 'http',       'high',   true,  '["always_on","messaging","skills","memory"]'::jsonb),
  ('hermes',       'Hermes Agent',     'http',       'high',   true,  '["always_on","learning","skills","memory","messaging"]'::jsonb),
  ('mcp',          'MCP Server',       'mcp',        'medium', true,  '["tool_calling"]'::jsonb),
  ('n8n-webhook',  'n8n Webhook',      'webhook',    'medium', false, '["workflow_automation"]'::jsonb),
  ('direct-api',   'Direct LLM API',   'direct-api', 'medium', true,  '["chat","tool_calling","streaming"]'::jsonb),
  ('manual',       'Manual',           'manual',     'low',    false, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- ===========================================================================
-- 2. PROVIDERS (de modelos LLM)
-- ===========================================================================
CREATE TABLE providers (
    id              TEXT PRIMARY KEY,        -- 'openai' | 'anthropic' | 'google' | 'openrouter'
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL,           -- 'api' | 'gateway' | 'local'
    env_key         TEXT NOT NULL,           -- nombre de la env var con la API key
    docs_url        TEXT,
    status          TEXT NOT NULL DEFAULT 'not_configured',  -- 'configured' | 'not_configured' | 'down'
    monthly_budget_usd  NUMERIC(10, 2),
    monthly_spend_usd   NUMERIC(10, 4) NOT NULL DEFAULT 0,
    last_check_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO providers (id, name, kind, env_key, docs_url) VALUES
  ('openai',     'OpenAI',         'api',     'OPENAI_API_KEY',     'https://platform.openai.com/docs/api-reference'),
  ('anthropic',  'Anthropic',      'api',     'ANTHROPIC_API_KEY',  'https://docs.anthropic.com/en/api/overview'),
  ('google',     'Google Gemini',  'api',     'GOOGLE_API_KEY',     'https://ai.google.dev/gemini-api/docs'),
  ('openrouter', 'OpenRouter',     'gateway', 'OPENROUTER_API_KEY', 'https://openrouter.ai/docs')
ON CONFLICT (id) DO NOTHING;

-- ===========================================================================
-- 3. MODELS (registry de modelos LLM por tier)
-- ===========================================================================
CREATE TABLE models (
    id              TEXT PRIMARY KEY,        -- 'claude-opus-4-7' | 'gpt-4o' | ...
    provider_id     TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    tier            SMALLINT NOT NULL,       -- 0..5 (ver §21.4 doc maestro)
    context_window  INTEGER NOT NULL DEFAULT 0,
    supports_tools   BOOLEAN NOT NULL DEFAULT false,
    supports_vision  BOOLEAN NOT NULL DEFAULT false,
    supports_code    BOOLEAN NOT NULL DEFAULT false,
    input_price_per_1m   NUMERIC(10, 4),     -- USD por 1M tokens input
    output_price_per_1m  NUMERIC(10, 4),
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_models_provider ON models(provider_id);
CREATE INDEX idx_models_tier ON models(tier);

-- ===========================================================================
-- 4. PROJECTS (contenedores de contexto)
-- ===========================================================================
CREATE TABLE projects (
    id              TEXT PRIMARY KEY,        -- mismo que slug por simplicidad
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'paused' | 'archived'
    owner_agent_id  TEXT,                    -- FK lógica a agents.id (no hard ref para evitar deps circular)
    monthly_budget_usd  NUMERIC(10, 2),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_status ON projects(status);

-- ===========================================================================
-- 5. AGENTS
-- ===========================================================================
CREATE TABLE agents (
    id                TEXT PRIMARY KEY,
    slug              TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    role              TEXT,
    description       TEXT,
    runtime_id        TEXT REFERENCES agent_runtimes(id),
    runtime_config    JSONB NOT NULL DEFAULT '{}'::jsonb,
    system_prompt     TEXT,                  -- inyectado al runtime al invocar
    allowed_tools     JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_projects  JSONB NOT NULL DEFAULT '[]'::jsonb,    -- [] = todos
    risk_level        TEXT NOT NULL DEFAULT 'medium',
    is_lead           BOOLEAN NOT NULL DEFAULT false,        -- NovaCore = true
    is_meta           BOOLEAN NOT NULL DEFAULT false,        -- opera el OS, no proyectos
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agents_runtime ON agents(runtime_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_is_lead ON agents(is_lead) WHERE is_lead = true;

-- ===========================================================================
-- 6. SKILLS
-- ===========================================================================
CREATE TABLE skill_sources (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    trust_level     TEXT NOT NULL DEFAULT 'community',  -- 'official' | 'community' | 'personal'
    last_synced_at  TIMESTAMPTZ,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE skills (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT,
    risk_level      TEXT NOT NULL DEFAULT 'medium',
    inputs          JSONB NOT NULL DEFAULT '[]'::jsonb,
    outputs         JSONB NOT NULL DEFAULT '[]'::jsonb,
    steps           TEXT,                    -- markdown
    compatible_agents    JSONB NOT NULL DEFAULT '[]'::jsonb,
    compatible_runtimes  JSONB NOT NULL DEFAULT '[]'::jsonb,
    source          TEXT NOT NULL DEFAULT 'local',  -- 'local' | 'imported'
    source_id       TEXT REFERENCES skill_sources(id) ON DELETE SET NULL,
    source_ref      TEXT,                    -- commit SHA / URL exacta
    version         TEXT,
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_skills_category ON skills(category);
CREATE INDEX idx_skills_status ON skills(status);

-- ===========================================================================
-- 7. CLI TOOLS (registro de CLIs detectadas en el host)
-- ===========================================================================
CREATE TABLE cli_tools (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    command         TEXT NOT NULL,           -- 'claude' | 'codex' | 'gh'
    kind            TEXT NOT NULL,           -- 'coding_agent' | 'dev_tool' | 'infra' | 'runtime'
    detected_path   TEXT,
    version         TEXT,
    runtime_id      TEXT REFERENCES agent_runtimes(id) ON DELETE SET NULL,
    status          TEXT NOT NULL DEFAULT 'not_detected',  -- 'detected' | 'not_detected' | 'broken'
    risk_level      TEXT NOT NULL DEFAULT 'medium',
    requires_auth   BOOLEAN NOT NULL DEFAULT false,
    capabilities    JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_detected_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ===========================================================================
-- 8. MCP CONNECTIONS
-- ===========================================================================
CREATE TABLE mcp_connections (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    transport       TEXT NOT NULL,           -- 'stdio' | 'http' | 'sse'
    command         TEXT,                    -- si stdio
    args            JSONB NOT NULL DEFAULT '[]'::jsonb,
    env             JSONB NOT NULL DEFAULT '{}'::jsonb,
    url             TEXT,                    -- si http/sse
    status          TEXT NOT NULL DEFAULT 'inactive',  -- 'active' | 'inactive' | 'error'
    health_score    SMALLINT NOT NULL DEFAULT 0,       -- 0..100
    last_sync_at    TIMESTAMPTZ,
    source_url      TEXT,
    project_scope   JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [] = todos
    permissions     JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_level      TEXT NOT NULL DEFAULT 'medium',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mcp_status ON mcp_connections(status);

-- ===========================================================================
-- 9. EXECUTIONS (log estructurado de cada invocación de agente)
-- ===========================================================================
CREATE TABLE executions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT REFERENCES projects(id) ON DELETE SET NULL,
    agent_id        TEXT REFERENCES agents(id) ON DELETE SET NULL,
    skill_id        TEXT REFERENCES skills(id) ON DELETE SET NULL,
    model_id        TEXT REFERENCES models(id) ON DELETE SET NULL,
    runtime_id      TEXT REFERENCES agent_runtimes(id) ON DELETE SET NULL,
    user_request    TEXT,
    result_summary  TEXT,
    status          TEXT NOT NULL DEFAULT 'pending',  -- 'pending' | 'running' | 'success' | 'failed' | 'cancelled'
    error_message   TEXT,
    input_tokens    INTEGER NOT NULL DEFAULT 0,
    output_tokens   INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd  NUMERIC(10, 6) NOT NULL DEFAULT 0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_executions_project ON executions(project_id, created_at DESC);
CREATE INDEX idx_executions_agent ON executions(agent_id, created_at DESC);
CREATE INDEX idx_executions_status ON executions(status);
CREATE INDEX idx_executions_created ON executions(created_at DESC);

-- ===========================================================================
-- 10. USAGE EVENTS (tokens/costo agregable)
-- ===========================================================================
CREATE TABLE usage_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id    UUID REFERENCES executions(id) ON DELETE CASCADE,
    provider_id     TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model_id        TEXT REFERENCES models(id) ON DELETE SET NULL,
    project_id      TEXT REFERENCES projects(id) ON DELETE SET NULL,
    agent_id        TEXT REFERENCES agents(id) ON DELETE SET NULL,
    skill_id        TEXT REFERENCES skills(id) ON DELETE SET NULL,
    input_tokens    INTEGER NOT NULL DEFAULT 0,
    output_tokens   INTEGER NOT NULL DEFAULT 0,
    cached_tokens   INTEGER NOT NULL DEFAULT 0,
    request_count   INTEGER NOT NULL DEFAULT 1,
    estimated_cost_usd  NUMERIC(10, 6) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_provider ON usage_events(provider_id, created_at DESC);
CREATE INDEX idx_usage_project ON usage_events(project_id, created_at DESC);
CREATE INDEX idx_usage_created ON usage_events(created_at DESC);

-- ===========================================================================
-- 11. SYSTEM LOGS (eventos de sistema, no requests HTTP)
-- ===========================================================================
CREATE TABLE system_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    level           TEXT NOT NULL,           -- 'debug' | 'info' | 'warn' | 'error'
    source          TEXT NOT NULL,           -- 'detector' | 'sysmetrics' | 'memory' | ...
    message         TEXT NOT NULL,
    context         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_system_logs_level ON system_logs(level, created_at DESC);
CREATE INDEX idx_system_logs_source ON system_logs(source, created_at DESC);

-- ===========================================================================
-- 12. NOVACORE — conversaciones y mensajes del asistente meta
-- ===========================================================================
CREATE TABLE novacore_conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         TEXT,                    -- v0.1 single-user; futuro multi
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMPTZ,
    message_count   INTEGER NOT NULL DEFAULT 0,
    total_input_tokens   INTEGER NOT NULL DEFAULT 0,
    total_output_tokens  INTEGER NOT NULL DEFAULT 0,
    total_cost_usd  NUMERIC(10, 6) NOT NULL DEFAULT 0
);

CREATE TABLE novacore_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES novacore_conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,           -- 'user' | 'assistant' | 'tool'
    content         TEXT,
    tool_calls      JSONB,
    tool_result     JSONB,
    tokens_in       INTEGER NOT NULL DEFAULT 0,
    tokens_out      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_novacore_msgs_conv ON novacore_messages(conversation_id, created_at);

-- ===========================================================================
-- Trigger genérico de updated_at
-- ===========================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_updated_at_agent_runtimes BEFORE UPDATE ON agent_runtimes
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_providers BEFORE UPDATE ON providers
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_models BEFORE UPDATE ON models
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_projects BEFORE UPDATE ON projects
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_agents BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_skills BEFORE UPDATE ON skills
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_skill_sources BEFORE UPDATE ON skill_sources
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_cli_tools BEFORE UPDATE ON cli_tools
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER set_updated_at_mcp_connections BEFORE UPDATE ON mcp_connections
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS novacore_messages CASCADE;
DROP TABLE IF EXISTS novacore_conversations CASCADE;
DROP TABLE IF EXISTS system_logs CASCADE;
DROP TABLE IF EXISTS usage_events CASCADE;
DROP TABLE IF EXISTS executions CASCADE;
DROP TABLE IF EXISTS mcp_connections CASCADE;
DROP TABLE IF EXISTS cli_tools CASCADE;
DROP TABLE IF EXISTS skills CASCADE;
DROP TABLE IF EXISTS skill_sources CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS models CASCADE;
DROP TABLE IF EXISTS providers CASCADE;
DROP TABLE IF EXISTS agent_runtimes CASCADE;
DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;
-- +goose StatementEnd
