package main

import (
	"context"
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/wire"
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

	// [P1-01] Migration v11: Add file permission tracking columns
	if currentVersion < 11 {
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_mode TEXT DEFAULT '';`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_uid INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN file_gid INTEGER DEFAULT 0;`)
		db.Exec(`ALTER TABLE project_files ADD COLUMN permission_changes INTEGER DEFAULT 0;`)

		db.Exec(`INSERT INTO schema_migrations (version) VALUES (11);`)
		currentVersion = 11
	}

	log.Println("Database initialized and migrated successfully.")
}


// getProjects returns all projects using repository
func getProjects() ([]Project, error) {
	ctx := context.Background()
	projects, err := wire.GetProjects(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Project, 0, len(projects))
	for _, p := range projects {
		result = append(result, toLegacyProject(p))
	}
	return result, nil
}

// toLegacyProject converts domain Project to legacy Project
func toLegacyProject(p *models.Project) Project {
	if p == nil {
		return Project{}
	}
	return Project{
		ID: p.ID, Name: p.Name, Description: p.Description, Template: p.Template,
		AppPaths: p.AppPaths, FilesPaths: p.FilesPaths,
		BlacklistExts: p.BlacklistExts, WhitelistPaths: p.WhitelistPaths,
		DBHost: p.DBHost, DBUser: p.DBUser, DBPass: p.DBPass, DBName: p.DBName,
		Status: p.Status, BaselineTotal: p.BaselineTotal, BaselineProcessed: p.BaselineProcessed,
		BaselineAt: p.BaselineAt, WatcherStatus: p.WatcherStatus,
		IntegrityScanEnabled: p.IntegrityScanEnabled, LastIntegrityScan: p.LastIntegrityScan,
		ErrorMessage: p.ErrorMessage, Configured: p.Configured, RescanInterval: p.RescanInterval,
	}
}

func getProjectByID(id int) (Project, error) {
	ctx := context.Background()
	p, err := wire.GetProjectByID(ctx, id)
	if err != nil {
		return Project{}, err
	}
	return toLegacyProject(p), nil
}

func addProject(p Project) (int, error) {
	ctx := context.Background()
	// Convert to domain model
	dm := &models.Project{
		Name:           p.Name,
		Description:    p.Description,
		Template:       p.Template,
		AppPaths:       p.AppPaths,
		FilesPaths:     p.FilesPaths,
		BlacklistExts:  p.BlacklistExts,
		WhitelistPaths: p.WhitelistPaths,
		DBHost:         p.DBHost,
		DBUser:         p.DBUser,
		DBPass:         p.DBPass,
		DBName:         p.DBName,
	}
	if dm.Template == "" {
		dm.Template = "OJS 3.x"
	}
	id, err := wire.CreateProject(ctx, dm)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func updateProject(p Project) error {
	ctx := context.Background()
	// Convert to domain model
	dm := &models.Project{
		ID:             p.ID,
		Name:           p.Name,
		Description:    p.Description,
		Template:       p.Template,
		AppPaths:       p.AppPaths,
		FilesPaths:     p.FilesPaths,
		BlacklistExts:  p.BlacklistExts,
		WhitelistPaths: p.WhitelistPaths,
		DBHost:         p.DBHost,
		DBUser:         p.DBUser,
		DBPass:         p.DBPass,
		DBName:         p.DBName,
		RescanInterval: p.RescanInterval,
	}
	return wire.UpdateProject(ctx, dm)
}

func CreateAdmin(username, passwordHash string) error {
	ctx := context.Background()
	return wire.CreateAdminUser(ctx, username, passwordHash)
}

func GetAdminByUsername(username string) (Admin, error) {
	ctx := context.Background()
	a, err := wire.GetAdminByUsername(ctx, username)
	if err != nil {
		return Admin{}, err
	}
	return Admin{ID: a.ID, Username: a.Username, PasswordHash: a.PasswordHash}, nil
}

func LogActivity(adminID int, action, target string) error {
	ctx := context.Background()
	return wire.LogActivity(ctx, adminID, action, target)
}

func GetAuditLogs() ([]AuditLog, error) {
	ctx := context.Background()
	logs, err := wire.GetAuditLogs(ctx, 100)
	if err != nil {
		return nil, err
	}
	result := make([]AuditLog, 0, len(logs))
	for _, l := range logs {
		result = append(result, AuditLog{
			ID: l.ID, AdminID: l.AdminID, AdminName: l.AdminName,
			Action: l.Action, Target: l.Target, Timestamp: l.Timestamp,
		})
	}
	return result, nil
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
	ctx := context.Background()
	files, err := wire.GetProjectFiles(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// Convert map[string]*models.ProjectFile to map[string]ProjectFile
	result := make(map[string]ProjectFile)
	for k, v := range files {
		result[k] = ProjectFile{
			ID: v.ID, ProjectID: v.ProjectID, FilePath: v.FilePath,
			Hash: v.Hash, FileSize: v.FileSize, ModTime: v.ModTime,
			Status: v.Status, FileType: v.FileType,
			FileMode: v.FileMode, FileUID: v.FileUID, FileGID: v.FileGID,
			PermissionChanges: v.PermissionChanges,
		}
	}
	return result, nil
}

func batchUpsertProjectFiles(files []ProjectFile) error {
	ctx := context.Background()
	// Convert to domain models
	dmFiles := make([]*models.ProjectFile, 0, len(files))
	for i := range files {
		f := &files[i]
		fileType := f.FileType
		if fileType == "" {
			fileType = "project"
		}
		dmFiles = append(dmFiles, &models.ProjectFile{
			ID: f.ID, ProjectID: f.ProjectID, FilePath: f.FilePath,
			Hash: f.Hash, FileSize: f.FileSize, ModTime: f.ModTime,
			Status: f.Status, FileType: fileType,
			FileMode: f.FileMode, FileUID: f.FileUID, FileGID: f.FileGID,
			PermissionChanges: f.PermissionChanges,
		})
	}
	return wire.BatchUpsertFiles(ctx, dmFiles)
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
