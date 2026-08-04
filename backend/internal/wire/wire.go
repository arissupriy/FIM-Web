// Package wire provides dependency injection for the application.
// This module wires together repositories, use cases, and services.
package wire

import (
	"context"
	"database/sql"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
	"ojs-monitor/backend/internal/infrastructure/database/sqlite"
)

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
	globalRepos = NewRepositories(db)
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
