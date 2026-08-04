// Package repository defines interfaces for data access.
// Implementations should be in internal/infrastructure/database/.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// ProjectRepository defines the interface for project data access
type ProjectRepository interface {
	// Create inserts a new project and returns the ID
	Create(ctx context.Context, p *models.Project) (int, error)

	// GetByID retrieves a project by ID
	GetByID(ctx context.Context, id int) (*models.Project, error)

	// List retrieves all projects
	List(ctx context.Context) ([]*models.Project, error)

	// Update updates an existing project
	Update(ctx context.Context, p *models.Project) error

	// Delete removes a project
	Delete(ctx context.Context, id int) error

	// UpdateStatus updates project status
	UpdateStatus(ctx context.Context, id int, status string) error

	// UpdateWatcherStatus updates project watcher status
	UpdateWatcherStatus(ctx context.Context, id int, status string) error

	// GetActiveProjects retrieves all active projects
	GetActiveProjects(ctx context.Context) ([]*models.Project, error)

	// GetProjectsForIntegrityScan retrieves projects with integrity scan enabled
	GetProjectsForIntegrityScan(ctx context.Context) ([]*models.Project, error)

	// UpdateBaseline updates baseline progress and status
	UpdateBaseline(ctx context.Context, id int, status string, total, processed int) error

	// UpdateIntegrityScan updates last integrity scan timestamp and status
	UpdateIntegrityScan(ctx context.Context, id int, status string) error

	// Count returns the number of projects
	Count(ctx context.Context) (int, error)
}
