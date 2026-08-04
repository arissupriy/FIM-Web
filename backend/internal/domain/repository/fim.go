// Package repository defines interfaces for data access.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// FIMEventRepository defines the interface for FIM event data access
type FIMEventRepository interface {
	// Create inserts a new FIM event
	Create(ctx context.Context, event *models.FIMEvent) error

	// GetByProjectID retrieves FIM events with filters
	GetByProjectID(ctx context.Context, projectID int, filters FIMEventFilters) ([]*models.FIMEvent, int, error)

	// GetStats returns FIM event statistics
	GetStats(ctx context.Context, projectID int) (*FIMStats, error)
}

// FIMEventFilters defines filters for FIM event queries
type FIMEventFilters struct {
	EventType     string
	RiskLevel    string
	Classification string
	Search       string
	Page         int
	Limit        int
}

// NewFIMEventFilters creates filters with defaults
func NewFIMEventFilters() FIMEventFilters {
	return FIMEventFilters{
		Page:  1,
		Limit: 50,
	}
}

// Validate validates and normalizes the filters
func (f *FIMEventFilters) Validate() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 50
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
}

// Offset calculates the SQL offset
func (f *FIMEventFilters) Offset() int {
	return (f.Page - 1) * f.Limit
}

// FIMStats holds FIM event statistics
type FIMStats struct {
	Events        int `json:"events"`
	HighRisk     int `json:"high_risk"`
	CriticalRisk int `json:"critical_risk"`
	UnknownSrc  int `json:"unknown_source"`
	AlertsSent  int `json:"alerts_sent"`
}

// FIMWatcherRepository defines the interface for FIM watcher data access
type FIMWatcherRepository interface {
	// UpdateProjectWatcherStatus updates project watcher status
	UpdateProjectWatcherStatus(ctx context.Context, projectID int, status string) error
}
