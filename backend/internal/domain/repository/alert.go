// Package repository defines interfaces for data access.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// AlertConfigRepository defines the interface for alert configuration data access
type AlertConfigRepository interface {
	// Create creates a new alert config
	Create(ctx context.Context, config *models.AlertConfig) (int, error)

	// GetByID retrieves an alert config by ID
	GetByID(ctx context.Context, id int) (*models.AlertConfig, error)

	// ListByProject retrieves all alert configs for a project
	ListByProject(ctx context.Context, projectID int) ([]*models.AlertConfig, error)

	// ListEnabled retrieves all enabled alert configs for a project
	ListEnabled(ctx context.Context, projectID int) ([]*models.AlertConfig, error)

	// Update updates an alert config
	Update(ctx context.Context, config *models.AlertConfig) error

	// Delete deletes an alert config
	Delete(ctx context.Context, id int) error

	// Enable enables an alert config
	Enable(ctx context.Context, id int) error

	// Disable disables an alert config
	Disable(ctx context.Context, id int) error
}

// AlertHistoryRepository defines the interface for alert history data access
type AlertHistoryRepository interface {
	// Create creates a new alert history entry
	Create(ctx context.Context, history *models.AlertHistory) (int, error)

	// GetByID retrieves an alert history entry by ID
	GetByID(ctx context.Context, id int) (*models.AlertHistory, error)

	// ListByConfig retrieves alert history for a config
	ListByConfig(ctx context.Context, configID int, limit int) ([]*models.AlertHistory, error)

	// ListByProject retrieves alert history for a project
	ListByProject(ctx context.Context, projectID int, limit int) ([]*models.AlertHistory, error)

	// UpdateStatus updates the status of an alert history entry
	UpdateStatus(ctx context.Context, id int, status models.AlertHistoryStatus, errorMsg string) error

	// MarkSent marks an alert as sent
	MarkSent(ctx context.Context, id int) error

	// IncrementRetry increments retry count
	IncrementRetry(ctx context.Context, id int) error

	// CheckDedup checks if an alert was recently sent (within dedup window)
	// Returns true if a similar alert was sent within the window
	CheckDedup(ctx context.Context, projectID int, filePath string, riskLevel string, dedupWindow int) (bool, error)

	// DeleteOld deletes alert history older than specified days
	DeleteOld(ctx context.Context, days int) error
}
