-- +goose Up

-- Templates registry table
-- Stores registered templates that can be used by projects
CREATE TABLE IF NOT EXISTS templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    version TEXT NOT NULL,
    description TEXT DEFAULT '',
    priority INTEGER DEFAULT 100,
    default_config TEXT,  -- JSON blob for default configuration
    enabled INTEGER DEFAULT 1,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now'))
);

-- Insert default OJS template
INSERT INTO templates (name, version, description, priority, default_config, enabled) VALUES (
    'ojs',
    '3.x',
    'Open Journal Systems - Open source journal management and publishing system',
    100,
    '{"template":"ojs","default_watch_paths":["public/","lib/pkp/","plugins/"],"default_files_paths":["files/"],"default_blacklist_exts":["php","phtml","php3","php4","php5","php7","pht","phar","sh","bash","zsh","pl","py","rb","exe","bat","cmd","ps1"],"default_whitelist_paths":["lib/pkp/classes/","plugins/generic/","plugins/themes/"],"default_rescan_interval":10,"watch_type":"OJS_WORKFLOW"}',
    1
);

-- Add template_id column to projects (nullable for backward compatibility)
ALTER TABLE projects ADD COLUMN template_id INTEGER;

-- Update existing projects to use OJS template
UPDATE projects SET template_id = (SELECT id FROM templates WHERE name = 'ojs') WHERE template = 'OJS 3.x' OR template = 'ojs';

-- Add foreign key constraint (after all projects have template_id)
-- This is done separately to ensure data integrity

-- Add template_version column for tracking
ALTER TABLE projects ADD COLUMN template_version TEXT;

-- Update template_version from templates table
UPDATE projects SET template_version = (SELECT version FROM templates WHERE id = projects.template_id);

-- Create index on template_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_projects_template ON projects(template_id);

-- +goose Down

-- Drop index first
DROP INDEX IF EXISTS idx_projects_template;

-- Drop template_version column
ALTER TABLE projects DROP COLUMN template_version;

-- Remove template_id foreign key (SQLite doesn't support DROP CONSTRAINT easily)
-- We use a workaround by recreating the table
BEGIN TRANSACTION;

CREATE TABLE projects_backup (
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

INSERT INTO projects_backup SELECT
    id, name, description, app_path, files_path, db_host, db_user, db_pass, db_name,
    blacklist_exts, whitelist_paths, template, status, baseline_total, baseline_processed,
    error_message, rescan_interval, baseline_at, watcher_status, integrity_scan_enabled, last_integrity_scan
FROM projects;

DROP TABLE projects;
ALTER TABLE projects_backup RENAME TO projects;

COMMIT;

-- Drop templates table
DROP TABLE IF EXISTS templates;
