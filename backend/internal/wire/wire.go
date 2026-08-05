// Package wire provides dependency injection for the application.
// This module wires together repositories, use cases, and services.
package wire

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"

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
// Returns the database instance and any error encountered.
func InitDB() (*sql.DB, error) {
	var err error
	DB, err = sql.Open("sqlite", "./database/ojs_monitor.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err = DB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	_, err = DB.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}
	DB.Exec("PRAGMA synchronous=NORMAL;")

	// Run migrations
	if err := runMigrations(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	globalRepos = NewRepositories(DB)
	return DB, nil
}

// runMigrations runs database migrations using goose.
func runMigrations() error {
	// Set goose dialect for SQLite
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Get migrations directory - resolve relative to current working directory
	// because database is also relative to cwd
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	migrationsPath := filepath.Join(cwd, "database", "migrations")

	// Also check binary location as fallback
	execPath, _ := os.Executable()
	if execPath != "" {
		binDir := filepath.Dir(execPath)
		altPath := filepath.Join(binDir, "..", "database", "migrations")
		if _, err := os.Stat(altPath); err == nil {
			migrationsPath = altPath
		}
	}

	log.Printf("Migrations path: %s", migrationsPath)

	// Run goose migrations
	if err := goose.Up(DB, migrationsPath); err != nil && err != goose.ErrAlreadyApplied {
		return fmt.Errorf("goose up failed: %w", err)
	}

	log.Println("Database migrations applied.")
	return nil
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
