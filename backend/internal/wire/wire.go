// Package wire provides dependency injection for the application.
// This module wires together repositories, use cases, and services.
package wire

import (
	"context"
	"database/sql"
	"log"

	_ "modernc.org/sqlite"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
	"ojs-monitor/backend/internal/infrastructure/database/sqlite"
)

// DB returns the underlying sql.DB
var DB *sql.DB

// Repositories holds all repository instances.
type Repositories struct {
	Project   repository.ProjectRepository
	Job       repository.JobRepository
	File      repository.FileRepository
	FIMEvent  repository.FIMEventRepository
	Auth      repository.AuthRepository
}

// NewRepositories creates all repository instances with the given database.
func NewRepositories(db *sql.DB) *Repositories {
	sqliteDB := sqlite.NewDB(db)

	return &Repositories{
		Project:  sqlite.NewProjectRepository(sqliteDB),
		Job:      sqlite.NewJobRepository(sqliteDB),
		File:     sqlite.NewFileRepository(sqliteDB),
		FIMEvent: sqlite.NewFIMEventRepository(sqliteDB),
		Auth:     sqlite.NewAuthRepository(sqliteDB),
	}
}

// Global repositories (set by main.go)
var globalRepos *Repositories

// Init initializes global repositories.
func Init(db *sql.DB) {
	DB = db
	globalRepos = NewRepositories(db)
}

// InitDB initializes the database connection and runs migrations.
// Returns the database instance for backward compatibility.
func InitDB() *sql.DB {
	var err error
	DB, err = sql.Open("sqlite", "./database/ojs_monitor.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}
	DB.Exec("PRAGMA synchronous=NORMAL;")

	runMigrations()
	globalRepos = NewRepositories(DB)

	return DB
}

func runMigrations() {
	// Create schema_migrations table
	DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`)

	var currentVersion int
	err := DB.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}

	if currentVersion < 1 {
		DB.Exec(`
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
		DB.Exec(`ALTER TABLE projects ADD COLUMN blacklist_exts TEXT DEFAULT '["php","phtml","sh"]';`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN whitelist_paths TEXT DEFAULT '[]';`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (2);`)
		currentVersion = 2
	}

	if currentVersion < 3 {
		DB.Exec(`ALTER TABLE projects ADD COLUMN template TEXT DEFAULT 'OJS 3.x';`)
		DB.Exec(`
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
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (3);`)
		currentVersion = 3
	}

	if currentVersion < 4 {
		DB.Exec(`ALTER TABLE projects ADD COLUMN status TEXT DEFAULT 'pending_baseline';`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN baseline_total INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN baseline_processed INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN error_message TEXT DEFAULT '';`)
		DB.Exec(`
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
		`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (4);`)
		currentVersion = 4
	}

	if currentVersion < 5 {
		DB.Exec(`ALTER TABLE jobs ADD COLUMN files_success INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE jobs ADD COLUMN files_skipped INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE jobs ADD COLUMN files_error INTEGER DEFAULT 0;`)
		DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_one_running_job ON jobs(project_id) WHERE status = 'running';`)
		DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_files_unique ON project_files(project_id, file_path);`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (5);`)
		currentVersion = 5
	}

	if currentVersion < 6 {
		DB.Exec(`ALTER TABLE projects ADD COLUMN rescan_interval INTEGER DEFAULT 10;`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (6);`)
		currentVersion = 6
	}

	if currentVersion < 7 {
		DB.Exec(`ALTER TABLE project_files ADD COLUMN created_at INTEGER DEFAULT (strftime('%s', 'now'));`)
		DB.Exec(`ALTER TABLE project_files ADD COLUMN updated_at INTEGER DEFAULT (strftime('%s', 'now'));`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (7);`)
		currentVersion = 7
	}

	if currentVersion < 8 {
		DB.Exec(`ALTER TABLE project_files ADD COLUMN file_type TEXT DEFAULT 'project';`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (8);`)
		currentVersion = 8
	}

	if currentVersion < 9 {
		DB.Exec(`
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
		DB.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_project ON fim_events(project_id);`)
		DB.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_timestamp ON fim_events(timestamp);`)
		DB.Exec(`CREATE INDEX IF NOT EXISTS idx_fim_events_file ON fim_events(file_path);`)
		DB.Exec(`
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
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (9);`)
		currentVersion = 9
	}

	if currentVersion < 10 {
		DB.Exec(`ALTER TABLE projects ADD COLUMN baseline_at INTEGER;`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN watcher_status TEXT DEFAULT 'stopped';`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN integrity_scan_enabled INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE projects ADD COLUMN last_integrity_scan INTEGER;`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (10);`)
		currentVersion = 10
	}

	if currentVersion < 11 {
		DB.Exec(`ALTER TABLE project_files ADD COLUMN file_mode TEXT DEFAULT '';`)
		DB.Exec(`ALTER TABLE project_files ADD COLUMN file_uid INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE project_files ADD COLUMN file_gid INTEGER DEFAULT 0;`)
		DB.Exec(`ALTER TABLE project_files ADD COLUMN permission_changes INTEGER DEFAULT 0;`)
		DB.Exec(`INSERT INTO schema_migrations (version) VALUES (11);`)
		currentVersion = 11
	}

	log.Println("Database initialized and migrated successfully.")
}

// Project returns the project repository.
func Project() repository.ProjectRepository {
	return globalRepos.Project
}

// Job returns the job repository.
func Job() repository.JobRepository {
	return globalRepos.Job
}

// File returns the file repository.
func File() repository.FileRepository {
	return globalRepos.File
}

// FIMEvent returns the FIM event repository.
func FIMEvent() repository.FIMEventRepository {
	return globalRepos.FIMEvent
}

// Auth returns the auth repository.
func Auth() repository.AuthRepository {
	return globalRepos.Auth
}

// ─────────────────────────────────────────────────────────────────────────────
// Repository-backed functions for legacy code compatibility
// These functions wrap repository calls for use by legacy db.go, handlers.go, etc.
// ─────────────────────────────────────────────────────────────────────────────

// GetProjects returns all projects.
func GetProjects(ctx context.Context) ([]*models.Project, error) {
	return globalRepos.Project.List(ctx)
}

// GetProjectByID returns a project by ID.
func GetProjectByID(ctx context.Context, id int) (*models.Project, error) {
	return globalRepos.Project.GetByID(ctx, id)
}

// CreateProject creates a new project.
func CreateProject(ctx context.Context, p *models.Project) (int, error) {
	return globalRepos.Project.Create(ctx, p)
}

// UpdateProject updates a project.
func UpdateProject(ctx context.Context, p *models.Project) error {
	return globalRepos.Project.Update(ctx, p)
}

// CreateAdminUser creates a new admin.
func CreateAdminUser(ctx context.Context, username, passwordHash string) error {
	return globalRepos.Auth.CreateAdmin(ctx, username, passwordHash)
}

// GetAdminByUsername returns an admin by username.
func GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error) {
	return globalRepos.Auth.GetAdminByUsername(ctx, username)
}

// LogActivity logs an activity.
func LogActivity(ctx context.Context, adminID int, action, target string) error {
	return globalRepos.Auth.LogActivity(ctx, adminID, action, target)
}

// GetAuditLogs returns audit logs.
func GetAuditLogs(ctx context.Context, limit int) ([]*models.AuditLog, error) {
	return globalRepos.Auth.GetAuditLogs(ctx, limit)
}

// GetAdminCount returns the number of admin users.
func GetAdminCount(ctx context.Context) int {
	count, err := globalRepos.Auth.Count(ctx)
	if err != nil {
		return 0
	}
	return count
}

// GetProjectsCount returns the number of projects.
func GetProjectsCount(ctx context.Context) int {
	count, err := globalRepos.Project.Count(ctx)
	if err != nil {
		return 0
	}
	return count
}

// GetProjectFiles returns all files for a project.
func GetProjectFiles(ctx context.Context, projectID int) (map[string]*models.ProjectFile, error) {
	return globalRepos.File.GetByProjectID(ctx, projectID)
}

// BatchUpsertFiles inserts or updates files.
func BatchUpsertFiles(ctx context.Context, files []*models.ProjectFile) error {
	return globalRepos.File.BatchUpsert(ctx, files)
}

// CreateFIMEvent creates a FIM event.
func CreateFIMEvent(ctx context.Context, event *models.FIMEvent) error {
	return globalRepos.FIMEvent.Create(ctx, event)
}

// SeedDefaultAdmin seeds the default admin user.
func SeedDefaultAdmin(ctx context.Context) error {
	return globalRepos.Auth.SeedDefaultAdmin(ctx)
}

// BatchDeleteFiles deletes files by IDs.
func BatchDeleteFiles(ctx context.Context, ids []int) error {
	return globalRepos.File.BatchDelete(ctx, ids)
}

// GetProjectsForIntegrityScan returns projects with integrity scan enabled.
func GetProjectsForIntegrityScan(ctx context.Context) ([]*models.Project, error) {
	return globalRepos.Project.GetProjectsForIntegrityScan(ctx)
}

// UpdateBaseline updates baseline progress and status.
func UpdateBaseline(ctx context.Context, id int, status string, total, processed int) error {
	return globalRepos.Project.UpdateBaseline(ctx, id, status, total, processed)
}

// UpdateIntegrityScan updates last integrity scan timestamp and status.
func UpdateIntegrityScan(ctx context.Context, id int, status string) error {
	return globalRepos.Project.UpdateIntegrityScan(ctx, id, status)
}

// ResuscitateCrashedJobs marks running jobs as queued.
func ResuscitateCrashedJobs(ctx context.Context) (int, error) {
	return globalRepos.Job.ResuscitateCrashedJobs(ctx)
}

// ClaimNextQueued attempts to claim the next queued job.
func ClaimNextQueued(ctx context.Context) (jobID, projectID int, jobType string, success bool, err error) {
	return globalRepos.Job.ClaimNextQueued(ctx)
}

// CompleteJob marks a job as done with stats.
func CompleteJob(ctx context.Context, id int, success, skipped, errors int) error {
	return globalRepos.Job.Complete(ctx, id, success, skipped, errors)
}

// FailJob marks a job as failed with error message.
func FailJob(ctx context.Context, id int, errMsg string) error {
	return globalRepos.Job.Fail(ctx, id, errMsg)
}

// CreateIntegrityScanJob creates a new integrity scan job.
func CreateIntegrityScanJob(ctx context.Context, projectID int) error {
	_, err := globalRepos.Job.Create(ctx, projectID, "integrity_scan")
	return err
}

// GetJobRunningCount returns running/queued job count for a project.
func GetJobRunningCount(ctx context.Context, projectID int) (int, error) {
	return globalRepos.Job.GetRunningCount(ctx, projectID)
}

// UpdateProjectStatus updates project status.
func UpdateProjectStatus(ctx context.Context, id int, status string) error {
	return globalRepos.Project.UpdateStatus(ctx, id, status)
}

// GetProjectStatus returns the current project status.
func GetProjectStatus(ctx context.Context, id int) string {
	p, err := globalRepos.Project.GetByID(ctx, id)
	if err != nil {
		return "active"
	}
	return p.Status
}

// UpdateProjectStatusWithError updates project status to error with message.
func UpdateProjectStatusWithError(ctx context.Context, id int, errMsg string) error {
	// First get the project to merge fields
	p, err := globalRepos.Project.GetByID(ctx, id)
	if err != nil {
		return err
	}
	p.Status = "error"
	p.ErrorMessage = errMsg
	return globalRepos.Project.Update(ctx, p)
}

// UpdateWatcherStatus updates project watcher status.
func UpdateWatcherStatus(ctx context.Context, id int, status string) error {
	return globalRepos.Project.UpdateWatcherStatus(ctx, id, status)
}

// GetBaselineFile retrieves a baseline file by project ID and path.
func GetBaselineFile(ctx context.Context, projectID int, filePath string) (*models.ProjectFile, error) {
	return globalRepos.File.GetBaselineFile(ctx, projectID, filePath)
}

// IncrementPermissionChanges increments permission change counter.
func IncrementPermissionChanges(ctx context.Context, fileID, projectID int) error {
	return globalRepos.File.IncrementPermissionChanges(ctx, fileID, projectID)
}

// GetFileStats returns file statistics for a project.
func GetFileStats(ctx context.Context, projectID int) (added, modified, deleted, orphan int, err error) {
	return globalRepos.File.GetStats(ctx, projectID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy compatibility wrappers for files that still use legacy types
// ─────────────────────────────────────────────────────────────────────────────

// LegacyProject is the legacy project type from models.go
type LegacyProject struct {
	ID                    int
	Name                  string
	Description           string
	Template              string
	AppPaths              []string
	FilesPaths            []string
	BlacklistExts         []string
	WhitelistPaths        []string
	DBHost                string
	DBUser                string
	DBPass                string
	DBName                string
	Status                string
	BaselineTotal         int
	BaselineProcessed     int
	BaselineAt            int64
	WatcherStatus         string
	IntegrityScanEnabled  int
	LastIntegrityScan     int64
	ErrorMessage          string
	Configured            bool
	RescanInterval        int
}

// LegacyProjectFile is the legacy project file type
type LegacyProjectFile struct {
	ID                 int
	ProjectID          int
	FilePath           string
	Hash               string
	FileSize           int64
	ModTime            int64
	Status             string
	FileType           string
	FileMode           string
	FileUID            uint32
	FileGID            uint32
	PermissionChanges  int
}

// toLegacyProject converts domain Project to legacy Project
func ToLegacyProject(p *models.Project) LegacyProject {
	if p == nil {
		return LegacyProject{}
	}
	return LegacyProject{
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

// GetLegacyProjects returns all projects as legacy type.
func GetLegacyProjects(ctx context.Context) ([]LegacyProject, error) {
	projects, err := globalRepos.Project.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]LegacyProject, 0, len(projects))
	for _, p := range projects {
		result = append(result, ToLegacyProject(p))
	}
	return result, nil
}

// GetLegacyProjectFiles returns all files for a project as legacy type.
func GetLegacyProjectFiles(ctx context.Context, projectID int) (map[string]LegacyProjectFile, error) {
	files, err := globalRepos.File.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]LegacyProjectFile)
	for k, v := range files {
		result[k] = LegacyProjectFile{
			ID: v.ID, ProjectID: v.ProjectID, FilePath: v.FilePath,
			Hash: v.Hash, FileSize: v.FileSize, ModTime: v.ModTime,
			Status: v.Status, FileType: v.FileType,
			FileMode: v.FileMode, FileUID: v.FileUID, FileGID: v.FileGID,
			PermissionChanges: v.PermissionChanges,
		}
	}
	return result, nil
}

// BatchUpsertLegacyFiles inserts or updates legacy files.
func BatchUpsertLegacyFiles(ctx context.Context, files []LegacyProjectFile) error {
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
	return globalRepos.File.BatchUpsert(ctx, dmFiles)
}

// BatchDeleteLegacyFiles deletes files by IDs.
func BatchDeleteLegacyFiles(ctx context.Context, ids []int) error {
	return globalRepos.File.BatchDelete(ctx, ids)
}
