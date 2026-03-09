-- +goose Up
-- +goose StatementBegin

-- Dispatch messages for inter-agent communication
CREATE TABLE IF NOT EXISTS dispatch_messages (
    id TEXT PRIMARY KEY,
    from_agent TEXT NOT NULL CHECK (from_agent != ''),
    to_agent TEXT NOT NULL CHECK (to_agent != ''),
    session_id TEXT,
    parent_message_id TEXT,
    task TEXT NOT NULL,
    context TEXT DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed')),
    result TEXT,
    error TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    completed_at INTEGER,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_dispatch_to_agent_status ON dispatch_messages (to_agent, status);
CREATE INDEX IF NOT EXISTS idx_dispatch_from_agent ON dispatch_messages (from_agent);
CREATE INDEX IF NOT EXISTS idx_dispatch_session ON dispatch_messages (session_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_status_created ON dispatch_messages (status, created_at);

CREATE TRIGGER IF NOT EXISTS update_dispatch_updated_at
AFTER UPDATE ON dispatch_messages
BEGIN
UPDATE dispatch_messages SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;

-- Agent registry for multi-agent orchestration
CREATE TABLE IF NOT EXISTS agents (
    name TEXT PRIMARY KEY,
    description TEXT,
    capabilities TEXT DEFAULT '[]',
    system_prompt TEXT,
    cli_command TEXT,
    model_requirements TEXT DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_enabled ON agents (enabled);

CREATE TRIGGER IF NOT EXISTS update_agents_updated_at
AFTER UPDATE ON agents
BEGIN
UPDATE agents SET updated_at = strftime('%s', 'now')
WHERE name = new.name;
END;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_dispatch_updated_at;
DROP TRIGGER IF EXISTS update_agents_updated_at;
DROP INDEX IF EXISTS idx_agents_enabled;
DROP INDEX IF EXISTS idx_dispatch_status_created;
DROP INDEX IF EXISTS idx_dispatch_session;
DROP INDEX IF EXISTS idx_dispatch_from_agent;
DROP INDEX IF EXISTS idx_dispatch_to_agent_status;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS dispatch_messages;
-- +goose StatementEnd
