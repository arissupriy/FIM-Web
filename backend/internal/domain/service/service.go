// Package service defines interfaces for domain services.
// Implementations should be in internal/infrastructure/.
package service

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// ScannerService defines the interface for file scanning operations
type ScannerService interface {
	// StartBaselineScan initiates a baseline scan for a project
	StartBaselineScan(ctx context.Context, projectID int) error

	// StartIntegrityScan initiates an integrity scan for a project
	StartIntegrityScan(ctx context.Context, projectID int) error

	// CancelScan cancels a running scan
	CancelScan(ctx context.Context, jobID int) error

	// GetScanStatus returns the current scan status
	GetScanStatus(ctx context.Context, projectID int) (*ScanStatus, error)
}

// ScanStatus represents the current status of a scan
type ScanStatus struct {
	IsRunning   bool
	JobID      int
	Progress   int // 0-100
	FilesTotal int
	FilesDone  int
	Message    string
}

// OJSService defines the interface for OJS database operations
type OJSService interface {
	// Connect establishes connection to OJS database
	Connect(ctx context.Context, host, user, pass, dbname string) error

	// GetDashboardMetrics retrieves dashboard metrics from OJS
	GetDashboardMetrics(ctx context.Context) (*models.DashboardMetrics, error)

	// GetOJSDetails retrieves OJS version and configuration
	GetOJSDetails(ctx context.Context) (*models.OJSDetails, error)

	// ReconcileFiles checks files against OJS database
	ReconcileFiles(ctx context.Context, files []*models.ProjectFile) ([]*models.ProjectFile, error)

	// GetFileRelations retrieves OJS relations for a file
	GetFileRelations(ctx context.Context, filePath string) ([]FileRelation, error)
}

// FileRelation represents a relation between a file and OJS entities
type FileRelation struct {
	Type           string `json:"type"`
	FileID         int    `json:"file_id"`
	OriginalName   string `json:"original_name"`
	SubmissionID   int    `json:"submission_id"`
	ArticleTitle   string `json:"article_title"`
	AuthorName     string `json:"author_name"`
	UploaderID     int    `json:"uploader_user_id"`
	UploaderName  string `json:"uploader_name"`
	UploaderEmail string `json:"uploader_email"`
	DateUploaded   string `json:"date_uploaded"`
}

// WatcherService defines the interface for FIM watcher operations
type WatcherService interface {
	// StartWatcher starts the FIM watcher for a project
	StartWatcher(ctx context.Context, projectID int, paths []string) error

	// StopWatcher stops the FIM watcher for a project
	StopWatcher(ctx context.Context, projectID int) error

	// GetWatcherStatus returns the watcher status for a project
	GetWatcherStatus(ctx context.Context, projectID int) (*WatcherStatus, error)

	// IsWatcherRunning returns true if watcher is running for project
	IsWatcherRunning(ctx context.Context, projectID int) (bool, error)

	// RestoreWatchers restores watchers for all active projects
	RestoreWatchers(ctx context.Context) error
}

// WatcherStatus represents the current watcher status
type WatcherStatus struct {
	IsRunning bool
	ProjectID int
	Paths    []string
}

// WorkerService defines the interface for background worker operations
type WorkerService interface {
	// Start starts the background worker
	Start(ctx context.Context) error

	// Stop gracefully stops the worker
	Stop() error

	// TriggerScan triggers an immediate scan
	TriggerScan()

	// IsRunning returns worker running status
	IsRunning() bool
}
