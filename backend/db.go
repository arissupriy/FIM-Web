package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./database/ojs_monitor.db")
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

	log.Println("Database initialized and migrated successfully.")
}


func getProjects() ([]Project, error) {
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

// SeedDefaultAdmin creates or updates the default admin
func SeedDefaultAdmin() error {
	username := "admin"
	password := "admin123"

	// Check if admin exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Admin exists, verify password is correct by checking
		// We use a fixed bcrypt cost for consistency
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return err
		}

		// Update the password hash to ensure it's correct
		_, err = db.Exec("UPDATE admins SET password_hash = ? WHERE username = ?", string(hash), username)
		if err != nil {
			log.Printf("Warning: failed to update admin password: %v", err)
		}
		log.Println("Admin verified: admin / admin123")
	} else {
		// Create new admin
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return err
		}
		_, err = db.Exec("INSERT INTO admins (username, password_hash) VALUES (?, ?)", username, string(hash))
		if err != nil {
			return err
		}
		log.Println("Default admin created: admin / admin123")
	}
	return nil
}

func getProjectFiles(projectID int) (map[string]ProjectFile, error) {
	rows, err := db.Query("SELECT id, project_id, file_path, hash, file_size, mod_time, status FROM project_files WHERE project_id=?", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make(map[string]ProjectFile)
	for rows.Next() {
		var pf ProjectFile
		if err := rows.Scan(&pf.ID, &pf.ProjectID, &pf.FilePath, &pf.Hash, &pf.FileSize, &pf.ModTime, &pf.Status); err != nil {
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
	stmtInsert, err := tx.Prepare("INSERT OR IGNORE INTO project_files (project_id, file_path, hash, file_size, mod_time, status, file_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now'))")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmtInsert.Close()

	stmtUpdate, err := tx.Prepare("UPDATE project_files SET hash=?, file_size=?, mod_time=?, status=?, updated_at=strftime('%s', 'now') WHERE id=?")
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
		if f.ID == 0 {
			// INSERT OR IGNORE - skip if already exists (handles orphan re-detection)
			_, err = tx.Stmt(stmtInsert).Exec(f.ProjectID, f.FilePath, f.Hash, f.FileSize, f.ModTime, f.Status, fileType)
		} else {
			_, err = stmtUpdate.Exec(f.Hash, f.FileSize, f.ModTime, f.Status, f.ID)
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
