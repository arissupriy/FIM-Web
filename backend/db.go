package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// Migration mutex to prevent concurrent migrations
var migrationMutex sync.Mutex

func initDB() {
	var err error

	// Ensure directory exists
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	db, err = sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}
	db.Exec("PRAGMA synchronous=NORMAL;")

	runMigrations()
}

func runMigrations() {
	// Lock to prevent concurrent migrations from multiple server starts
	migrationMutex.Lock()
	defer migrationMutex.Unlock()

	// Create schema_migrations table
	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`)

	var currentVersion int
	err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}

	if currentVersion < 1 {
		db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			app_path TEXT DEFAULT '[]',
			files_path TEXT DEFAULT '[]',
			db_host TEXT DEFAULT '',
			db_user TEXT DEFAULT '',
			db_pass TEXT DEFAULT '',
			db_name TEXT DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			admin_id INTEGER,
			action TEXT NOT NULL,
			target TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (admin_id) REFERENCES admins(id)
		);
		INSERT INTO schema_migrations (version) VALUES (1);
		`)
		currentVersion = 1
	}

	if currentVersion < 2 {
		// Migration v2: Add blacklist and whitelist columns
		db.Exec(`ALTER TABLE projects ADD COLUMN blacklist_exts TEXT DEFAULT '["php","phtml","sh"]';`)
		db.Exec(`ALTER TABLE projects ADD COLUMN whitelist_paths TEXT DEFAULT '[]';`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (2);`)
		currentVersion = 2
	}

	if currentVersion < 3 {
		// Migration v3: Add template to projects, create project_files
		db.Exec(`ALTER TABLE projects ADD COLUMN template TEXT DEFAULT 'OJS 3.x';`)
		db.Exec(`
		CREATE TABLE IF NOT EXISTS project_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER,
			file_path TEXT NOT NULL,
			file_size INTEGER,
			mod_time INTEGER,
			status TEXT,
			hash TEXT DEFAULT '',
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now')),
			FOREIGN KEY (project_id) REFERENCES projects(id)
		);
		`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (3);`)
		currentVersion = 3
	}

	if currentVersion < 4 {
		// Migration v4: Async Worker Support
		db.Exec(`ALTER TABLE projects ADD COLUMN status TEXT DEFAULT 'pending_baseline';`)
		db.Exec(`ALTER TABLE projects ADD COLUMN baseline_total INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE projects ADD COLUMN baseline_processed INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE projects ADD COLUMN error_message TEXT DEFAULT '';`)
		
		db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER,
			type TEXT,
			status TEXT DEFAULT 'queued',
			error_message TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			finished_at DATETIME,
			FOREIGN KEY (project_id) REFERENCES projects(id)
		);
		`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (4);`)
		currentVersion = 4
	}

	if currentVersion < 5 {
		// Migration v5: Production Robustness
		db.Exec(`ALTER TABLE jobs ADD COLUMN files_success INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE jobs ADD COLUMN files_skipped INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE jobs ADD COLUMN files_error INTEGER DEFAULT 0;`)

		// Constraint: Prevent duplicate running jobs
		db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_one_running_job ON jobs(project_id) WHERE status = 'running';`)

		// Constraint: Unique file paths per project
		db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_files_unique ON project_files(project_id, file_path);`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (5);`)
		currentVersion = 5
	}

	if currentVersion < 6 {
		// Migration v6: Configurable Rescan Interval (in minutes, 0 = disabled)
		db.Exec(`ALTER TABLE projects ADD COLUMN rescan_interval INTEGER DEFAULT 10;`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (6);`)
		currentVersion = 6
	}

	if currentVersion < 7 {
		// Migration v7: Add proper timestamps to project_files
		db.Exec(`ALTER TABLE project_files ADD COLUMN created_at INTEGER DEFAULT (strftime('%s', 'now'));`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN updated_at INTEGER DEFAULT (strftime('%s', 'now'));`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (7);`)
		currentVersion = 7
	}

	if currentVersion < 8 {
		// Migration v8: Add file_type column for classifying project vs upload files
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_type TEXT DEFAULT 'project';`)
		db.Exec(`INSERT INTO schema_migrations (version) VALUES (8);`)
		currentVersion = 8
	}

	if currentVersion < 9 {
		// Migration v9: FIM Forensic Events & Watch Paths
		db.Exec(`
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
		`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_project ON fim_events(project_id);`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_timestamp ON fim_events(timestamp);`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_file ON fim_events(file_path);`)

		db.Exec(`
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
		`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (9);`)
		currentVersion = 9
	}

	if currentVersion < 10 {
		// Migration v10: Add baseline_at and watcher_status to projects
		db.Exec(`ALTER TABLE projects ADD COLUMN baseline_at INTEGER;`)
		db.Exec(`ALTER TABLE projects ADD COLUMN watcher_status TEXT DEFAULT 'stopped';`)
		db.Exec(`ALTER TABLE projects ADD COLUMN integrity_scan_enabled INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE projects ADD COLUMN last_integrity_scan INTEGER;`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (10);`)
		currentVersion = 10
	}

	// Migration v11: Add file_id to fim_events for linking to project_files
	if currentVersion < 11 {
		db.Exec(`ALTER TABLE fim_events ADD COLUMN file_id INTEGER REFERENCES project_files(id);`)

		// Backfill file_id for existing events
		db.Exec(`
			UPDATE fim_events
			SET file_id = (
				SELECT pf.id FROM project_files pf
				WHERE pf.project_id = fim_events.project_id
				AND pf.file_path = fim_events.file_path
			)
			WHERE file_id IS NULL AND file_path IS NOT NULL
		`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (11);`)
		currentVersion = 11
	}

	// Migration v12: Add scheduled_at to jobs for scheduled integrity scans
	if currentVersion < 12 {
		db.Exec(`ALTER TABLE jobs ADD COLUMN scheduled_at INTEGER;`)
		db.Exec(`ALTER TABLE projects ADD COLUMN integrity_scan_interval_hours INTEGER DEFAULT 24;`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (12);`)
		currentVersion = 12
	}

	// Migration v13: Add file permission tracking columns
	if currentVersion < 13 {
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_mode TEXT DEFAULT '';`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_uid INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_gid INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN permission_changes INTEGER DEFAULT 0;`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (13);`)
		currentVersion = 13
	}

	// Migration v14: Alert system tables
	if currentVersion < 14 {
		// Alert configurations per project and channel
		db.Exec(`
			CREATE TABLE IF NOT EXISTS alert_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER,
				channel_type TEXT NOT NULL,
				config_json TEXT DEFAULT '{}',
				enabled INTEGER DEFAULT 1,
				min_risk_level TEXT DEFAULT 'HIGH',
				created_at INTEGER DEFAULT (strftime('%s', 'now')),
				updated_at INTEGER DEFAULT (strftime('%s', 'now')),
				FOREIGN KEY (project_id) REFERENCES projects(id),
				UNIQUE(project_id, channel_type)
			);
		`)

		// Alert history for tracking sent alerts
		db.Exec(`
			CREATE TABLE IF NOT EXISTS alert_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER,
				event_id INTEGER,
				channel_type TEXT NOT NULL,
				status TEXT DEFAULT 'pending',
				error_message TEXT DEFAULT '',
				retry_count INTEGER DEFAULT 0,
				sent_at INTEGER,
				created_at INTEGER DEFAULT (strftime('%s', 'now')),
				FOREIGN KEY (project_id) REFERENCES projects(id),
				FOREIGN KEY (event_id) REFERENCES fim_events(id)
			);
		`)

		// Indexes for alert_history
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_alert_history_project ON alert_history(project_id);`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_alert_history_status ON alert_history(status);`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_alert_history_created ON alert_history(created_at);`)

			db.Exec(`INSERT INTO schema_migrations (version) VALUES (14);`)
		currentVersion = 14
	}

	// Migration v15: Compliance reports and hash chain
	if currentVersion < 15 {
		// Scheduled reports table
		db.Exec(`
			CREATE TABLE IF NOT EXISTS scheduled_reports (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				project_id INTEGER,
				name TEXT NOT NULL,
				framework TEXT DEFAULT 'soc2',
				format TEXT DEFAULT 'html',
				schedule_cron TEXT DEFAULT '0 6 * * 1',
				recipients TEXT DEFAULT '[]',
				enabled INTEGER DEFAULT 1,
				last_run INTEGER,
				next_run INTEGER,
				created_at INTEGER DEFAULT (strftime('%s', 'now')),
				updated_at INTEGER DEFAULT (strftime('%s', 'now')),
				FOREIGN KEY (project_id) REFERENCES projects(id)
			);
		`)

		// Generated reports table
		db.Exec(`
			CREATE TABLE IF NOT EXISTS generated_reports (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				report_id INTEGER,
				project_id INTEGER,
				file_path TEXT,
				file_size INTEGER,
				status TEXT DEFAULT 'pending',
				error_message TEXT DEFAULT '',
				generated_at INTEGER,
				sent_at INTEGER,
				sent_to TEXT DEFAULT '[]',
				created_at INTEGER DEFAULT (strftime('%s', 'now'))
			);
		`)

		// Add hash chain columns to fim_events
		db.Exec(`ALTER TABLE fim_events ADD COLUMN prev_event_hash TEXT DEFAULT '';`)
		db.Exec(`ALTER TABLE fim_events ADD COLUMN event_hash TEXT DEFAULT '';`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (15);`)
		currentVersion = 15
	}

	log.Println("Database initialized and migrated successfully.")
}


func getProjects() ([]Project, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := db.Query("SELECT id, name, description, template, app_path, files_path, blacklist_exts, whitelist_paths, db_host, db_user, db_pass, db_name, status, COALESCE(baseline_total, 0), COALESCE(baseline_processed, 0), COALESCE(error_message, ''), COALESCE(rescan_interval, 10), COALESCE(baseline_at, 0), COALESCE(watcher_status, 'stopped'), COALESCE(integrity_scan_enabled, 0), COALESCE(last_integrity_scan, 0) FROM projects")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var appPathJSON, filesPathJSON, blacklistJSON, whitelistJSON string

		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Template, &appPathJSON, &filesPathJSON, &blacklistJSON, &whitelistJSON, &p.DBHost, &p.DBUser, &p.DBPass, &p.DBName, &p.Status, &p.BaselineTotal, &p.BaselineProcessed, &p.ErrorMessage, &p.RescanInterval, &p.BaselineAt, &p.WatcherStatus, &p.IntegrityScanEnabled, &p.LastIntegrityScan); err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(appPathJSON), &p.AppPaths)
		json.Unmarshal([]byte(filesPathJSON), &p.FilesPaths)
		json.Unmarshal([]byte(blacklistJSON), &p.BlacklistExts)
		json.Unmarshal([]byte(whitelistJSON), &p.WhitelistPaths)

		if p.AppPaths == nil { p.AppPaths = []string{} }
		if p.FilesPaths == nil { p.FilesPaths = []string{} }
		if p.BlacklistExts == nil { p.BlacklistExts = []string{"php", "phtml", "sh"} }
		if p.WhitelistPaths == nil { p.WhitelistPaths = []string{} }
		if p.RescanInterval == 0 { p.RescanInterval = 10 } // Default 10 minutes (for backward compat)
		if p.WatcherStatus == "" { p.WatcherStatus = "stopped" }

		projects = append(projects, p)
	}
	return projects, nil
}

func getProjectByID(id int) (Project, error) {
	if db == nil {
		return Project{}, fmt.Errorf("database not initialized")
	}
	projects, err := getProjects()
	if err != nil {
		return Project{}, err
	}
	for _, p := range projects {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("project not found")
}

func addProject(p Project) (int, error) {
	if p.Template == "" {
		p.Template = "OJS 3.x"
	}
	appPathJSON, _ := json.Marshal(p.AppPaths)
	filesPathJSON, _ := json.Marshal(p.FilesPaths)
	blacklistJSON, _ := json.Marshal(p.BlacklistExts)
	whitelistJSON, _ := json.Marshal(p.WhitelistPaths)

	result, err := db.Exec("INSERT INTO projects (name, description, template, app_path, files_path, blacklist_exts, whitelist_paths, db_host, db_user, db_pass, db_name, status, baseline_total, baseline_processed, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unconfigured', 0, 0, '')",
		p.Name, p.Description, p.Template, string(appPathJSON), string(filesPathJSON), string(blacklistJSON), string(whitelistJSON), p.DBHost, p.DBUser, p.DBPass, p.DBName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	return int(id), nil
}

func updateProject(p Project) error {
	appPathJSON, _ := json.Marshal(p.AppPaths)
	filesPathJSON, _ := json.Marshal(p.FilesPaths)
	blacklistJSON, _ := json.Marshal(p.BlacklistExts)
	whitelistJSON, _ := json.Marshal(p.WhitelistPaths)

	// Determine status based on configuration completeness
	newStatus := "unconfigured"
	if p.DBHost != "" && p.DBUser != "" && p.DBName != "" && len(p.AppPaths) > 0 && p.AppPaths[0] != "" {
		newStatus = "pending_baseline"
	}

	_, err := db.Exec("UPDATE projects SET name=?, description=?, template=?, app_path=?, files_path=?, blacklist_exts=?, whitelist_paths=?, db_host=?, db_user=?, db_pass=?, db_name=?, status=?, rescan_interval=? WHERE id=?",
		p.Name, p.Description, p.Template, string(appPathJSON), string(filesPathJSON), string(blacklistJSON), string(whitelistJSON), p.DBHost, p.DBUser, p.DBPass, p.DBName, newStatus, p.RescanInterval, p.ID)

	return err
}

func CreateAdmin(username, passwordHash string) error {
	_, err := db.Exec("INSERT INTO admins (username, password_hash) VALUES (?, ?)", username, passwordHash)
	return err
}

func GetAdminByUsername(username string) (Admin, error) {
	var a Admin
	err := db.QueryRow("SELECT id, username, password_hash FROM admins WHERE username = ?", username).
		Scan(&a.ID, &a.Username, &a.PasswordHash)
	return a, err
}

func LogActivity(adminID int, action, target string) error {
	_, err := db.Exec("INSERT INTO audit_logs (admin_id, action, target) VALUES (?, ?, ?)", adminID, action, target)
	return err
}

func GetAuditLogs() ([]AuditLog, error) {
	rows, err := db.Query(`
		SELECT l.id, l.admin_id, a.username, l.action, l.target, l.timestamp
		FROM audit_logs l
		JOIN admins a ON l.admin_id = a.id
		ORDER BY l.timestamp DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.AdminID, &l.AdminName, &l.Action, &l.Target, &l.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// SeedDefaultAdmin creates the default admin if none exists
// NOTE: On first run, creates admin/admin123. In production, set ADMIN_PASSWORD env
// and change credentials immediately after first login.
func SeedDefaultAdmin() error {
	username := "admin"
	// Get password from env, or use default (CHANGE IN PRODUCTION!)
	password := getEnv("ADMIN_PASSWORD", "admin123")

	// Check if admin exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Admin already exists - don't update password every startup
		// This prevents accidental password reset in production
		log.Printf("Admin user '%s' already exists\n", username)
	} else {
		// Create new admin with bcrypt cost 12 (secure default)
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		_, err = db.Exec("INSERT INTO admins (username, password_hash) VALUES (?, ?)", username, string(hash))
		if err != nil {
			return err
		}
		log.Printf("Default admin created: %s / (set via ADMIN_PASSWORD env)\n", username)
	}
	return nil
}

func getProjectFiles(projectID int) (map[string]ProjectFile, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rows, err := db.Query("SELECT id, project_id, file_path, hash, file_size, mod_time, status, COALESCE(file_mode, ''), COALESCE(file_uid, 0), COALESCE(file_gid, 0), COALESCE(permission_changes, 0) FROM project_files WHERE project_id=?", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make(map[string]ProjectFile)
	for rows.Next() {
		var pf ProjectFile
		if err := rows.Scan(&pf.ID, &pf.ProjectID, &pf.FilePath, &pf.Hash, &pf.FileSize, &pf.ModTime, &pf.Status, &pf.FileMode, &pf.FileUID, &pf.FileGID, &pf.PermissionChanges); err != nil {
			return nil, err
		}
		files[pf.FilePath] = pf
	}
	return files, nil
}

func batchUpsertProjectFiles(files []ProjectFile) error {
	if len(files) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmtInsert, err := tx.Prepare("INSERT OR IGNORE INTO project_files (project_id, file_path, hash, file_size, mod_time, status, file_type, file_mode, file_uid, file_gid, permission_changes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmtInsert.Close()

	stmtUpdate, err := tx.Prepare("UPDATE project_files SET hash=?, file_size=?, mod_time=?, file_mode=?, file_uid=?, file_gid=?, status=?, updated_at=strftime('%s', 'now') WHERE id=?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmtUpdate.Close()

	for _, f := range files {
		fileType := f.FileType
		if fileType == "" {
			fileType = "project"
		}
		fileMode := f.FileMode
		if fileMode == "" {
			fileMode = ""
		}
		if f.ID == 0 {
			// INSERT OR IGNORE - skip if already exists (handles orphan re-detection)
			_, err = tx.Stmt(stmtInsert).Exec(f.ProjectID, f.FilePath, f.Hash, f.FileSize, f.ModTime, f.Status, fileType, fileMode, f.FileUID, f.FileGID, f.PermissionChanges)
		} else {
			_, err = stmtUpdate.Exec(f.Hash, f.FileSize, f.ModTime, fileMode, f.FileUID, f.FileGID, f.Status, f.ID)
		}
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func batchDeleteProjectFiles(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("DELETE FROM project_files WHERE id=?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
