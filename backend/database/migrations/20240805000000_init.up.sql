-- +goose Up
-- +goose StatementBegin

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    app_path TEXT DEFAULT '[]',
    files_path TEXT DEFAULT '[]',
    db_host TEXT DEFAULT '',
    db_user TEXT DEFAULT '',
    db_pass TEXT DEFAULT '',
    db_name TEXT DEFAULT '',
    blacklist_exts TEXT DEFAULT '["php","phtml","sh"]',
    whitelist_paths TEXT DEFAULT '[]',
    template TEXT DEFAULT 'OJS 3.x',
    status TEXT DEFAULT 'pending_baseline',
    baseline_total INTEGER DEFAULT 0,
    baseline_processed INTEGER DEFAULT 0,
    error_message TEXT DEFAULT '',
    rescan_interval INTEGER DEFAULT 10,
    baseline_at INTEGER,
    watcher_status TEXT DEFAULT 'stopped',
    integrity_scan_enabled INTEGER DEFAULT 0,
    last_integrity_scan INTEGER
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Admins table
CREATE TABLE IF NOT EXISTS admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id INTEGER,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (admin_id) REFERENCES admins(id)
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Project files table
CREATE TABLE IF NOT EXISTS project_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    mod_time INTEGER,
    status TEXT,
    hash TEXT DEFAULT '',
    file_type TEXT DEFAULT 'project',
    file_mode TEXT DEFAULT '',
    file_uid INTEGER DEFAULT 0,
    file_gid INTEGER DEFAULT 0,
    permission_changes INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    type TEXT,
    status TEXT DEFAULT 'queued',
    error_message TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    finished_at DATETIME,
    files_success INTEGER DEFAULT 0,
    files_skipped INTEGER DEFAULT 0,
    files_error INTEGER DEFAULT 0,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- +goose StatementEnd

-- +goose StatementBegin

-- FIM events table
CREATE TABLE IF NOT EXISTS fim_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    event_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_hash TEXT,
    actor_type TEXT,
    actor_id TEXT,
    actor_name TEXT,
    actor_details TEXT,
    risk_level TEXT DEFAULT 'LOW',
    classification TEXT,
    source TEXT DEFAULT 'WATCHER',
    details TEXT,
    processed INTEGER DEFAULT 0,
    alert_sent INTEGER DEFAULT 0,
    timestamp INTEGER,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

-- +goose StatementEnd

-- +goose StatementBegin

-- FIM watch paths table
CREATE TABLE IF NOT EXISTS fim_watch_paths (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    path TEXT NOT NULL,
    watch_type TEXT DEFAULT 'OJS_WORKFLOW',
    enabled INTEGER DEFAULT 1,
    alert_on_unknown INTEGER DEFAULT 1,
    alert_level TEXT DEFAULT 'HIGH',
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (project_id) REFERENCES projects(id),
    UNIQUE(project_id, path)
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Alert configs table (P2-01)
CREATE TABLE IF NOT EXISTS alert_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    name TEXT NOT NULL,
    channel TEXT NOT NULL CHECK(channel IN ('email', 'slack', 'webhook')),
    config TEXT NOT NULL DEFAULT '{}',
    conditions TEXT NOT NULL DEFAULT '{}',
    risk_level TEXT DEFAULT 'MEDIUM' CHECK(risk_level IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    enabled INTEGER DEFAULT 1,
    dedup_window INTEGER DEFAULT 60,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Alert history table (P2-01)
CREATE TABLE IF NOT EXISTS alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_config_id INTEGER,
    fim_event_id INTEGER,
    project_id INTEGER,
    channel TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'sent', 'failed', 'retry')),
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    error_message TEXT DEFAULT '',
    response_body TEXT DEFAULT '',
    sent_at INTEGER,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (alert_config_id) REFERENCES alert_configs(id) ON DELETE SET NULL,
    FOREIGN KEY (fim_event_id) REFERENCES fim_events(id) ON DELETE SET NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- +goose StatementEnd

-- +goose StatementBegin

-- Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_files_unique ON project_files(project_id, file_path);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE UNIQUE INDEX IF NOT EXISTS idx_one_running_job ON jobs(project_id) WHERE status = 'running';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_fim_events_project ON fim_events(project_id);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_fim_events_timestamp ON fim_events(timestamp);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_fim_events_file ON fim_events(file_path);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_alert_configs_project ON alert_configs(project_id);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_alert_configs_enabled ON alert_configs(enabled);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_alert_history_config ON alert_history(alert_config_id);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_alert_history_status ON alert_history(status);

-- +goose StatementEnd

-- +goose StatementBegin

CREATE INDEX IF NOT EXISTS idx_alert_history_created ON alert_history(created_at);

-- +goose StatementEnd
