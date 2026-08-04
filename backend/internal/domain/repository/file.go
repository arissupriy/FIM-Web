// Package repository defines interfaces for data access.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// FileFilters defines filters for file queries.
type FileFilters struct {
	Status   string // ADDED, MODIFIED, DELETED, ORPHAN
	FileType string // project, uploads
	Search   string
	FileID   int
	Limit    int
	Offset   int
}

// NewFileFilters creates FileFilters with defaults.
func NewFileFilters() FileFilters {
	return FileFilters{
		Limit: 50,
		Offset: 0,
	}
}

// FileListParams holds parameters for flexible file listing.
type FileListParams struct {
	ProjectID  int
	Search     string
	Status     string
	FileType   string
	FilesPaths []string // For distinguishing project vs uploads files
	Limit      int
	Offset     int
}

// FileRepository defines the interface for project file data access
type FileRepository interface {
	// BatchUpsert inserts or updates multiple files
	BatchUpsert(ctx context.Context, files []*models.ProjectFile) error

	// BatchDelete removes multiple files by ID
	BatchDelete(ctx context.Context, ids []int) error

	// GetByProjectID retrieves all files for a project as a map
	GetByProjectID(ctx context.Context, projectID int) (map[string]*models.ProjectFile, error)

	// GetByProjectIDPaginated retrieves files with pagination
	GetByProjectIDPaginated(ctx context.Context, projectID, limit, offset int) ([]*models.ProjectFile, error)

	// List retrieves files with flexible filters
	List(ctx context.Context, params FileListParams) ([]*models.ProjectFile, error)

	// Count returns total files count for a project
	Count(ctx context.Context, projectID int, status string) (int, error)

	// GetOrphans retrieves orphan files for a project
	GetOrphans(ctx context.Context, projectID, limit int) ([]*models.ProjectFile, error)

	// ListByProjectID retrieves files for a project with filters, returns files and total count
	ListByProjectID(ctx context.Context, projectID int, filters FileFilters) ([]*models.ProjectFile, int, error)

	// GetByID retrieves a single file
	GetByID(ctx context.Context, fileID, projectID int) (*models.ProjectFile, error)

	// GetByHash retrieves files with the same hash
	GetByHash(ctx context.Context, hash string) ([]*models.ProjectFile, error)

	// DeleteByProjectID removes all files for a project
	DeleteByProjectID(ctx context.Context, projectID int) error

	// IncrementPermissionChanges increments permission change counter
	IncrementPermissionChanges(ctx context.Context, fileID, projectID int) error

	// GetStats returns file statistics for a project
	GetStats(ctx context.Context, projectID int) (added, modified, deleted, orphan int, err error)

	// GetBaselineFile retrieves a file by project ID and path for permission comparison
	GetBaselineFile(ctx context.Context, projectID int, filePath string) (*models.ProjectFile, error)
}
