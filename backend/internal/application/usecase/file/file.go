// Package file provides file-related use cases.
package file

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// UseCase handles file-related business logic.
type UseCase struct {
	fileRepo    repository.FileRepository
	projectRepo repository.ProjectRepository
}

// New creates a new file use case.
func New(fileRepo repository.FileRepository, projectRepo repository.ProjectRepository) *UseCase {
	return &UseCase{fileRepo: fileRepo, projectRepo: projectRepo}
}

// GetByProjectID returns all files for a project.
func (uc *UseCase) GetByProjectID(ctx context.Context, projectID int, limit int) ([]*models.ProjectFile, error) {
	if limit <= 0 {
		limit = 500
	}
	return uc.fileRepo.GetByProjectIDPaginated(ctx, projectID, limit, 0)
}

// ListParams holds pagination and filter parameters.
type ListParams struct {
	ProjectID  int
	Search     string
	Status     string
	FileType   string // "project", "uploads", "all"
	Page       int
	Limit      int
}

// ListResult holds paginated file list result.
type ListResult struct {
	Files      []*models.ProjectFile `json:"files"`
	Pagination Pagination            `json:"pagination"`
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// List returns paginated files with filters.
func (uc *UseCase) List(ctx context.Context, params ListParams) (*ListResult, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200 // Max limit
	}

	// Get project for file type filtering
	var filesPaths []string
	if params.FileType != "all" {
		project, err := uc.projectRepo.GetByID(ctx, params.ProjectID)
		if err == nil && project != nil {
			filesPaths = project.FilesPaths
		}
	}

	offset := (params.Page - 1) * params.Limit

	// Get files
	files, err := uc.fileRepo.List(ctx, repository.FileListParams{
		ProjectID: params.ProjectID,
		Search:    params.Search,
		Status:    params.Status,
		FileType:  params.FileType,
		FilesPaths: filesPaths,
		Limit:     params.Limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}

	// Get total count
	total, err := uc.fileRepo.Count(ctx, params.ProjectID, params.Status)
	if err != nil {
		return nil, err
	}

	totalPages := total / params.Limit
	if total%params.Limit > 0 {
		totalPages++
	}

	return &ListResult{
		Files: files,
		Pagination: Pagination{
			Page:       params.Page,
			Limit:      params.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetOrphans returns orphan files for a project.
func (uc *UseCase) GetOrphans(ctx context.Context, projectID int, limit int) ([]*models.ProjectFile, error) {
	if limit <= 0 {
		limit = 500
	}
	return uc.fileRepo.GetOrphans(ctx, projectID, limit)
}

// GetByID returns a file by ID.
func (uc *UseCase) GetByID(ctx context.Context, fileID, projectID int) (*models.ProjectFile, error) {
	return uc.fileRepo.GetByID(ctx, fileID, projectID)
}

// GetByHash returns files with the same hash.
func (uc *UseCase) GetByHash(ctx context.Context, hash string) ([]*models.ProjectFile, error) {
	return uc.fileRepo.GetByHash(ctx, hash)
}

// FileStats holds file statistics.
type FileStats struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Deleted  int `json:"deleted"`
	Orphan   int `json:"orphan"`
}

// GetStats returns file statistics for a project.
func (uc *UseCase) GetStats(ctx context.Context, projectID int) (*FileStats, error) {
	added, modified, deleted, orphan, err := uc.fileRepo.GetStats(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &FileStats{
		Added:    added,
		Modified: modified,
		Deleted:  deleted,
		Orphan:   orphan,
	}, nil
}
