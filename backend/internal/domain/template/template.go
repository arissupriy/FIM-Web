// Package template defines interfaces for CMS-specific detection strategies.
// Templates enable platform to detect orphan files, validate integrity, and provide metrics
// specific to each CMS (OJS, WordPress, Drupal, etc.)
//
// Core principle: domain/template is AGNORSTIC - it only knows about interfaces.
// No template implementation (OJS, WordPress, etc.) should be imported here.
package template

import (
	"context"
	"database/sql"

	"ojs-monitor/backend/internal/domain/models"
)

// DBConnection is a generic database connection interface for templates.
// Templates receive this interface to query CMS-specific data.
type DBConnection interface {
	// QueryContext executes a query that returns rows.
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	// QueryRowContext executes a query that returns a single row.
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	// ExecContext executes a query that doesn't return rows.
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	// Close closes the database connection.
	Close() error
}

// DBConnectionConfig holds database connection configuration.
type DBConnectionConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Timeout  int // seconds
}

// TemplateConfig holds template-specific default configuration.
// These defaults are applied when creating a new project.
type TemplateConfig struct {
	// Template identifier (e.g., "ojs", "wordpress")
	Template string `json:"template"`

	// Template display name
	DisplayName string `json:"display_name"`

	// Default watch paths pattern for this CMS
	DefaultWatchPaths []string `json:"default_watch_paths"`

	// Default files paths to monitor
	DefaultFilesPaths []string `json:"default_files_paths"`

	// Default blacklist file extensions
	DefaultBlacklistExts []string `json:"default_blacklist_exts"`

	// Default whitelist paths (excluded from blacklist)
	DefaultWhitelistPaths []string `json:"default_whitelist_paths"`

	// Default rescan interval in minutes
	DefaultRescanInterval int `json:"default_rescan_interval"`

	// Workflow type identifier
	WatchType string `json:"watch_type"`
}

// Template represents a CMS-specific detection strategy.
// Implement this interface to add support for new CMS platforms.
type Template interface {
	// Name returns the template identifier (e.g., "ojs", "wordpress")
	Name() string

	// Version returns supported versions (e.g., "3.x", "2.x" for OJS)
	Version() string

	// Priority returns detection priority (higher = checked first)
	Priority() int

	// DefaultConfig returns the default configuration for projects using this template.
	DefaultConfig() *TemplateConfig

	// CreateDBConnection creates a database connection for the CMS.
	// This isolates CMS-specific connection logic from infrastructure.
	CreateDBConnection(ctx context.Context, config DBConnectionConfig) (DBConnection, error)

	// DetectOrphans finds files not tracked in CMS database.
	// This is OPTIONAL - templates without orphan detection return nil, nil.
	DetectOrphans(ctx context.Context, db DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error)

	// GetMetrics returns CMS-specific dashboard metrics.
	// This is OPTIONAL - templates without metrics return nil, nil.
	GetMetrics(ctx context.Context, db DBConnection) (*TemplateMetrics, error)

	// ValidateIntegrity checks CMS-specific integrity rules.
	// This is OPTIONAL - templates without validation return empty slice.
	ValidateIntegrity(ctx context.Context, db DBConnection, project *models.Project) ([]IntegrityWarning, error)

	// CorrelateFile correlates a file change event with CMS data.
	// This is OPTIONAL - templates without correlation return nil, nil.
	CorrelateFile(ctx context.Context, db DBConnection, filePath string, eventType string) (*CorrelationResult, error)

	// RequiredDBConfig returns required database configuration fields.
	RequiredDBConfig() []string

	// Compatible checks if database contains CMS schema.
	// Used for auto-detection.
	Compatible(ctx context.Context, db DBConnection) (bool, error)
}

// CorrelationResult holds file correlation data from CMS.
type CorrelationResult struct {
	// Found indicates if the file was found in CMS records
	Found bool `json:"found"`

	// ActorType identifies the actor (e.g., "CMS_USER", "SYSTEM", "PROCESS")
	ActorType string `json:"actor_type"`

	// ActorID is the user's ID in CMS (if applicable)
	ActorID string `json:"actor_id,omitempty"`

	// ActorName is the user's name (if applicable)
	ActorName string `json:"actor_name,omitempty"`

	// ActorEmail is the user's email (if applicable)
	ActorEmail string `json:"actor_email,omitempty"`

	// SubmissionID is the submission/article ID (if applicable)
	SubmissionID string `json:"submission_id,omitempty"`

	// Classification determines risk level based on CMS context
	Classification string `json:"classification"`

	// RiskLevel determines alert priority
	RiskLevel string `json:"risk_level"`

	// Reason describes why this classification was assigned
	Reason string `json:"reason"`
}

// TemplateMetrics holds CMS-specific metrics
type TemplateMetrics struct {
	TemplateName string                 `json:"template_name"`
	Version      string                 `json:"version"`
	Specific     map[string]interface{} `json:"specific"` // CMS-specific metrics
}

// IntegrityWarning represents a policy violation warning
type IntegrityWarning struct {
	Level   WarningLevel `json:"level"` // LOW, MEDIUM, HIGH, CRITICAL
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details string       `json:"details,omitempty"`
}

// WarningLevel represents warning severity
type WarningLevel string

const (
	WarningLow      WarningLevel = "LOW"
	WarningMedium   WarningLevel = "MEDIUM"
	WarningHigh     WarningLevel = "HIGH"
	WarningCritical WarningLevel = "CRITICAL"
)

// NewCorrelationResult creates a default correlation result
func NewCorrelationResult(filePath string, eventType string) *CorrelationResult {
	return &CorrelationResult{
		Found:         false,
		ActorType:     "UNKNOWN",
		Classification: "UNKNOWN",
		RiskLevel:     "LOW",
		Reason:        "No CMS correlation available",
	}
}

// NewTemplateMetrics creates a new TemplateMetrics
func NewTemplateMetrics(name, version string) *TemplateMetrics {
	return &TemplateMetrics{
		TemplateName: name,
		Version:      version,
		Specific:     make(map[string]interface{}),
	}
}
