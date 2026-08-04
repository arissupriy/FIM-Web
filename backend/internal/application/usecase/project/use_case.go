// Package project contains project-related use cases.
package project

import (
	"context"
	"errors"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// ErrNotFound when project doesn't exist
var ErrNotFound = errors.New("project not found")

// ErrValidation when request validation fails
var ErrValidation = errors.New("validation error")

// UseCase provides project operations
type UseCase struct {
	repo repository.ProjectRepository
}

// New creates a new project use case
func New(repo repository.ProjectRepository) *UseCase {
	return &UseCase{repo: repo}
}

// List returns all projects
func (uc *UseCase) List(ctx context.Context) ([]*models.Project, error) {
	return uc.repo.List(ctx)
}

// GetByID returns a project by ID
func (uc *UseCase) GetByID(ctx context.Context, id int) (*models.Project, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// Create creates a new project
func (uc *UseCase) Create(ctx context.Context, p *models.Project) (int, error) {
	if p.Name == "" {
		return 0, ErrValidation
	}
	if p.Template == "" {
		p.Template = "ojs" // default template
	}
	return uc.repo.Create(ctx, p)
}

// Update updates an existing project
func (uc *UseCase) Update(ctx context.Context, p *models.Project) error {
	if err := uc.repo.Update(ctx, p); err != nil {
		return err
	}
	return nil
}

// Delete removes a project
func (uc *UseCase) Delete(ctx context.Context, id int) error {
	return uc.repo.Delete(ctx, id)
}
