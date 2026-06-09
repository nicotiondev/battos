PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS agent_runtimes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unavailable',
    binary_path TEXT,
    version TEXT,
    endpoint_url TEXT,
    risk_level TEXT NOT NULL DEFAULT 'medium',
    requires_auth INTEGER NOT NULL DEFAULT 0,
    capabilities TEXT NOT NULL DEFAULT '[]',
    config_schema TEXT NOT NULL DEFAULT '{}',
    detected_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    env_key TEXT NOT NULL,
    docs_url TEXT,
    status TEXT NOT NULL DEFAULT 'not_configured',
    monthly_budget_usd REAL,
    monthly_spend_usd REAL NOT NULL DEFAULT 0,
    last_check_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS models (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    tier INTEGER NOT NULL,
    context_window INTEGER NOT NULL DEFAULT 0,
    supports_tools INTEGER NOT NULL DEFAULT 0,
    supports_vision INTEGER NOT NULL DEFAULT 0,
    supports_code INTEGER NOT NULL DEFAULT 0,
    input_price_per_1m REAL,
    output_price_per_1m REAL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    owner_agent_id TEXT,
    monthly_budget_usd REAL,
    metadata TEXT NOT NULL DEFAULT '{}',
    domain_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT,
    description TEXT,
    runtime_id TEXT REFERENCES agent_runtimes(id),
    runtime_config TEXT NOT NULL DEFAULT '{}',
    system_prompt TEXT,
    allowed_tools TEXT NOT NULL DEFAULT '[]',
    allowed_projects TEXT NOT NULL DEFAULT '[]',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    is_lead INTEGER NOT NULL DEFAULT 0,
    is_meta INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skill_sources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    trust_level TEXT NOT NULL DEFAULT 'community',
    last_synced_at DATETIME,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    category TEXT,
    risk_level TEXT NOT NULL DEFAULT 'medium',
    inputs TEXT NOT NULL DEFAULT '[]',
    outputs TEXT NOT NULL DEFAULT '[]',
    steps TEXT,
    compatible_agents TEXT NOT NULL DEFAULT '[]',
    compatible_runtimes TEXT NOT NULL DEFAULT '[]',
    source TEXT NOT NULL DEFAULT 'local',
    source_id TEXT REFERENCES skill_sources(id) ON DELETE SET NULL,
    source_ref TEXT,
    version TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    prompt_template TEXT,
    lifecycle TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cli_tools (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    kind TEXT NOT NULL,
    detected_path TEXT,
    version TEXT,
    runtime_id TEXT REFERENCES agent_runtimes(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'not_detected',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    requires_auth INTEGER NOT NULL DEFAULT 0,
    capabilities TEXT NOT NULL DEFAULT '[]',
    last_detected_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_connections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    transport TEXT NOT NULL,
    command TEXT,
    args TEXT NOT NULL DEFAULT '[]',
    env TEXT NOT NULL DEFAULT '{}',
    url TEXT,
    status TEXT NOT NULL DEFAULT 'inactive',
    health_score INTEGER NOT NULL DEFAULT 0,
    last_sync_at DATETIME,
    source_url TEXT,
    project_scope TEXT NOT NULL DEFAULT '[]',
    permissions TEXT NOT NULL DEFAULT '[]',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS domains (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'archived')),
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS goals (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    goal_id TEXT REFERENCES goals(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT,
    assigned_agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'backlog' CHECK (status IN ('backlog', 'ready', 'in_progress', 'review', 'done', 'cancelled')),
    board_position INTEGER NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS knowledge_workspaces (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    layout TEXT NOT NULL DEFAULT 'raw_wiki_outputs' CHECK (layout IN ('raw_wiki_outputs')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS journals (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES knowledge_workspaces(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    journal_date DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    run_id TEXT,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('markdown', 'image', 'link', 'diff', 'build_report')),
    content TEXT,
    managed_path TEXT,
    external_url TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content IS NOT NULL OR managed_path IS NOT NULL OR external_url IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS repositories (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('managed_local', 'github')),
    name TEXT NOT NULL,
    remote_url TEXT,
    credential_ref TEXT,
    default_branch TEXT NOT NULL DEFAULT 'master',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    skill_id TEXT REFERENCES skills(id) ON DELETE SET NULL,
    runtime_adapter_id TEXT NOT NULL REFERENCES agent_runtimes(id) ON DELETE RESTRICT,
    repository_id TEXT REFERENCES repositories(id) ON DELETE SET NULL,
    prompt TEXT NOT NULL,
    requested_network INTEGER NOT NULL DEFAULT 0,
    network_enabled INTEGER NOT NULL DEFAULT 0,
    host_session_enabled INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'awaiting_approval' CHECK (status IN ('draft', 'awaiting_approval', 'queued', 'running', 'succeeded', 'failed', 'cancelled')),
    branch_name TEXT,
    result_summary TEXT,
    error_message TEXT,
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    metadata TEXT NOT NULL DEFAULT '{}',
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS run_approvals (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('execute', 'network', 'host_session', 'commit', 'push', 'remember')),
    decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    reason TEXT,
    decided_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS run_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    stream TEXT NOT NULL CHECK (stream IN ('system', 'stdout', 'stderr')),
    message TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS usage_events (
    id TEXT PRIMARY KEY,
    run_id TEXT REFERENCES runs(id) ON DELETE CASCADE,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model_id TEXT REFERENCES models(id) ON DELETE SET NULL,
    project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
    skill_id TEXT REFERENCES skills(id) ON DELETE SET NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 1,
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS system_logs (
    id TEXT PRIMARY KEY,
    level TEXT NOT NULL,
    source TEXT NOT NULL,
    message TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS novacore_conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    message_count INTEGER NOT NULL DEFAULT 0,
    total_input_tokens INTEGER NOT NULL DEFAULT 0,
    total_output_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_usd REAL NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS novacore_messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES novacore_conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT,
    tool_calls TEXT,
    tool_result TEXT,
    tokens_in INTEGER NOT NULL DEFAULT 0,
    tokens_out INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '{}',
    ip_address TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL DEFAULT 'manual',
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    topic_key TEXT,
    project_id TEXT,
    agent_id TEXT,
    scope TEXT NOT NULL DEFAULT 'project',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS memory_items_fts USING fts5(
    title, content, topic_key,
    content='memory_items',
    content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS memory_items_ai AFTER INSERT ON memory_items BEGIN
    INSERT INTO memory_items_fts(rowid, title, content, topic_key)
    VALUES (new.id, new.title, new.content, COALESCE(new.topic_key, ''));
END;

CREATE TRIGGER IF NOT EXISTS memory_items_ad AFTER DELETE ON memory_items BEGIN
    INSERT INTO memory_items_fts(memory_items_fts, rowid, title, content, topic_key)
    VALUES ('delete', old.id, old.title, old.content, COALESCE(old.topic_key, ''));
END;

CREATE TRIGGER IF NOT EXISTS memory_items_au AFTER UPDATE ON memory_items BEGIN
    INSERT INTO memory_items_fts(memory_items_fts, rowid, title, content, topic_key)
    VALUES ('delete', old.id, old.title, old.content, COALESCE(old.topic_key, ''));
    INSERT INTO memory_items_fts(rowid, title, content, topic_key)
    VALUES (new.id, new.title, new.content, COALESCE(new.topic_key, ''));
END;

CREATE INDEX IF NOT EXISTS idx_models_provider ON models(provider_id);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_agents_runtime ON agents(runtime_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_skills_status ON skills(status);
CREATE INDEX IF NOT EXISTS idx_goals_project ON goals(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project_id, status, board_position);
CREATE INDEX IF NOT EXISTS idx_artifacts_project ON artifacts(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_run ON artifacts(run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_repositories_project ON repositories(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_project_created ON runs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_run_approvals_run ON run_approvals(run_id, decided_at DESC);
CREATE INDEX IF NOT EXISTS idx_run_logs_run_created ON run_logs(run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_project ON usage_events(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_novacore_msgs_conv ON novacore_messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_memory_items_topic ON memory_items(topic_key) WHERE topic_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memory_items_project ON memory_items(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memory_items_type ON memory_items(type);
CREATE INDEX IF NOT EXISTS idx_memory_items_created ON memory_items(created_at DESC);

INSERT INTO agent_runtimes (id, name, kind, risk_level, requires_auth, capabilities) VALUES
  ('claude-code',  'Claude Code CLI',  'subprocess', 'high',   1, '["code_editing","file_reading","terminal_commands","mcp"]'),
  ('claude-code-host-session', 'Claude Code CLI (host session)', 'subprocess', 'high', 1, '["code_editing","file_reading","terminal_commands","mcp","host_session"]'),
  ('codex',        'Codex CLI',        'subprocess', 'high',   1, '["code_generation","repo_editing","tests"]'),
  ('codex-host-session', 'Codex CLI (host session)', 'subprocess', 'high', 1, '["code_generation","repo_editing","tests","host_session"]'),
  ('opencode',     'OpenCode',         'subprocess', 'medium', 1, '["code_editing","local_agent","terminal_workflows"]'),
  ('gemini-cli',   'Gemini CLI',       'subprocess', 'medium', 1, '["long_context","multimodal"]'),
  ('kimi-cli',     'Kimi CLI',         'subprocess', 'medium', 1, '["long_context"]'),
  ('qwen-cli',     'Qwen Code',        'subprocess', 'medium', 1, '["code_generation","local"]'),
  ('aider',        'Aider',            'subprocess', 'high',   1, '["code_editing","git_integration"]'),
  ('openclaw',     'OpenClaw Gateway', 'http',       'high',   1, '["always_on","messaging","skills","memory"]'),
  ('hermes',       'Hermes Agent',     'http',       'high',   1, '["always_on","learning","skills","memory","messaging"]'),
  ('mcp',          'MCP Server',       'mcp',        'medium', 1, '["tool_calling"]'),
  ('n8n-webhook',  'n8n Webhook',      'webhook',    'medium', 0, '["workflow_automation"]'),
  ('direct-api',   'Direct LLM API',   'direct-api', 'medium', 1, '["chat","tool_calling","streaming"]'),
  ('manual',       'Manual',           'manual',     'low',    0, '[]'),
  ('sandbox-smoke', 'Sandbox Smoke',   'subprocess', 'low',    0, '["smoke_test"]'),
  ('sandbox-memory-smoke', 'Sandbox Memory Smoke', 'subprocess', 'low', 0, '["smoke_test","memory"]')
ON CONFLICT (id) DO NOTHING;

INSERT INTO providers (id, name, kind, env_key, docs_url) VALUES
  ('openai',     'OpenAI',         'api',     'OPENAI_API_KEY',     'https://platform.openai.com/docs/api-reference'),
  ('anthropic',  'Anthropic',      'api',     'ANTHROPIC_API_KEY',  'https://docs.anthropic.com/en/api/overview'),
  ('google',     'Google Gemini',  'api',     'GOOGLE_API_KEY',     'https://ai.google.dev/gemini-api/docs'),
  ('openrouter', 'OpenRouter',     'gateway', 'OPENROUTER_API_KEY', 'https://openrouter.ai/docs')
ON CONFLICT (id) DO NOTHING;

-- Modelos que el worker referencia al registrar usage (recordUsage). Sin estos
-- el INSERT de usage_events fallaba por FK (models no estaba seeded).
INSERT INTO models (id, provider_id, name, tier) VALUES
  ('claude-3-5-sonnet', 'anthropic', 'Claude 3.5 Sonnet', 1),
  ('gpt-4o',            'openai',    'GPT-4o',             1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO agents (id, slug, name, role, description, runtime_id, is_lead, is_meta, status)
VALUES ('novacore', 'novacore', 'NovaCore', 'System Orchestrator', 'Asistente de sistema de BattOS que te ayuda a administrar y diagnosticar el OS.', 'direct-api', 1, 1, 'active')
ON CONFLICT (id) DO NOTHING;
