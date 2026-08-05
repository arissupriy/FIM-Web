// Package project contains project-related use cases.
package project

import (
	"context"
	"errors"
	"log"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
	"ojs-monitor/backend/internal/domain/template"
)

var (
	// ErrNotFound when project doesn't exist
	ErrNotFound = errors.New("project not found")

	// ErrValidation when request validation fails
	ErrValidation = errors.New("validation error")

	// ErrInvalidTemplate when template is not registered
	ErrInvalidTemplate = errors.New("invalid template")

	// ErrNoTemplates when no templates are registered
	ErrNoTemplates = errors.New("no templates registered")
)

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
// If template is empty, uses the highest priority registered template
func (uc *UseCase) Create(ctx context.Context, p *models.Project) (int, error) {
	if p.Name == "" {
		return 0, ErrValidation
	}

	// Resolve template
	if p.Template == "" {
		// Use highest priority template as default
		templates := template.List()
		if len(templates) == 0 {
			log.Println("Warning: No templates registered, using 'ojs' as default")
			p.Template = "ojs"
		} else {
			p.Template = templates[0].Name()
			log.Printf("Using template '%s' (priority: %d) as default\n", templates[0].Name(), templates[0].Priority())
		}
	}

	// Validate template exists
	if _, ok := template.Get(p.Template); !ok {
		return 0, ErrInvalidTemplate
	}

	// Apply template defaults
	t, _ := template.Get(p.Template)
	if t != nil {
		cfg := t.DefaultConfig()
		if cfg != nil {
			// Set default blacklist if not specified
			if len(p.BlacklistExts) == 0 {
				p.BlacklistExts = cfg.DefaultBlacklistExts
			}
			// Set default whitelist if not specified
			if len(p.WhitelistPaths) == 0 {
				p.WhitelistPaths = cfg.DefaultWhitelistPaths
			}
			// Set default rescan interval if not specified
			if p.RescanInterval == 0 {
				p.RescanInterval = cfg.DefaultRescanInterval
			}
			// Set watch type
			if p.WatchType == "" {
				p.WatchType = cfg.WatchType
			}
		}
	}

	return uc.repo.Create(ctx, p)
}

// Update updates an existing project
func (uc *UseCase) Update(ctx context.Context, p *models.Project) error {
	// Validate template if changed
	if p.Template != "" {
		if _, ok := template.Get(p.Template); !ok {
			return ErrInvalidTemplate
		}
	}

	return uc.repo.Update(ctx, p)
}

// Delete removes a project
func (uc *UseCase) Delete(ctx context.Context, id int) error {
	return uc.repo.Delete(ctx, id)
}

// ValidateTemplate checks if a template is valid
func ValidateTemplate(templateName string) bool {
	_, ok := template.Get(templateName)
	return ok
}

// ListTemplates returns all registered templates
func ListTemplates() []*models.TemplateInfo {
	templates := template.List()
	result := make([]*models.TemplateInfo, len(templates))
	for i, t := range templates {
		result[i] = &models.TemplateInfo{
			Name:        t.Name(),
			Version:     t.Version(),
			Priority:    t.Priority(),
			RequiredDB:  t.RequiredDBConfig(),
			HasDBConfig: len(t.RequiredDBConfig()) > 0,
		}
	}
	return result
}
