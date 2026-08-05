// Package template defines interfaces for CMS-specific detection strategies.
// Templates enable platform to detect orphan files, validate integrity, and provide metrics
// specific to each CMS (OJS, WordPress, Drupal, etc.)
package template

import (
	"context"
	"database/sql"

	"ojs-monitor/backend/internal/domain/models"
)

// DBConnection is a generic database connection interface for templates.
// Templates receive this interface to query CMS-specific data.
type DBConnection interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// TemplateConfig holds template-specific default configuration.
// These defaults are applied when creating a new project.
type TemplateConfig struct {
	// Template identifier (same as Name())
	Template string `json:"template"`

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

	// Template-specific settings (JSON)
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// Template represents a CMS-specific detection strategy.
// Implement this interface to add support for new CMS platforms.
type Template interface {
	// Name returns the template identifier (e.g., "ojs", "wordpress", "drupal")
	Name() string

	// Version returns supported versions (e.g., "3.x", "2.x" for OJS)
	Version() string

	// Priority returns detection priority (higher = checked first)
	// Use 100 for primary CMS, lower for fallbacks
	Priority() int

	// DefaultConfig returns the default configuration for projects using this template.
	// This is used when creating a new project with this template.
	DefaultConfig() *TemplateConfig

	// DetectOrphans finds files not tracked in CMS database.
	// Called during reconciliation to flag untracked uploads as orphans.
	// Returns files marked with Status="ORPHAN".
	DetectOrphans(ctx context.Context, db DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error)

	// GetMetrics returns CMS-specific dashboard metrics.
	// These are merged with generic FIM metrics.
	GetMetrics(ctx context.Context, db DBConnection) (*TemplateMetrics, error)

	// ValidateIntegrity checks CMS-specific integrity rules.
	// Return warnings or errors for policy violations.
	ValidateIntegrity(ctx context.Context, db DBConnection, project *models.Project) ([]IntegrityWarning, error)

	// CorrelateFile correlates a file change event with CMS data.
	// Returns actor information and risk classification.
	CorrelateFile(ctx context.Context, db DBConnection, filePath string, eventType string) (*CorrelationResult, error)

	// RequiredDBConfig returns required database configuration fields.
	// Used to validate project setup.
	RequiredDBConfig() []string

	// Compatible returns true if this template works with the given database.
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

	// Metadata is additional CMS-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateMetrics holds CMS-specific metrics merged with generic metrics
type TemplateMetrics struct {
	TemplateName string                 `json:"template_name"`
	Version    string                  `json:"version"`
	Generic    *models.DashboardMetrics `json:"generic"`
	Specific  map[string]interface{}  `json:"specific"` // CMS-specific metrics
}

// IntegrityWarning represents a policy violation warning
type IntegrityWarning struct {
	Level   WarningLevel // LOW, MEDIUM, HIGH, CRITICAL
	Code    string       // Warning code for filtering
	Message string       // Human-readable message
	Details string       // Additional context
}

// WarningLevel represents warning severity
type WarningLevel string

const (
	WarningLow      WarningLevel = "LOW"
	WarningMedium  WarningLevel = "MEDIUM"
	WarningHigh    WarningLevel = "HIGH"
	WarningCritical WarningLevel = "CRITICAL"
)

// NewTemplateMetrics creates a new TemplateMetrics with defaults
func NewTemplateMetrics(name, version string) *TemplateMetrics {
	return &TemplateMetrics{
		TemplateName: name,
		Version:     version,
		Generic:     &models.DashboardMetrics{},
		Specific:    make(map[string]interface{}),
	}
}

// AddMetric adds a CMS-specific metric
func (m *TemplateMetrics) AddMetric(key string, value interface{}) {
	m.Specific[key] = value
}

// NewCorrelationResult creates a default correlation result for unknown files
func NewCorrelationResult(filePath string, eventType string) *CorrelationResult {
	return &CorrelationResult{
		Found:         false,
		ActorType:     "UNKNOWN",
		Classification: "UNKNOWN_SOURCE",
		RiskLevel:     "LOW",
		Reason:        "File not found in CMS database",
	}
}

// SetActor sets actor information on a correlation result
func (r *CorrelationResult) SetActor(actorType, actorID, actorName, actorEmail string) {
	r.Found = true
	r.ActorType = actorType
	r.ActorID = actorID
	r.ActorName = actorName
	r.ActorEmail = actorEmail
}

// SetRiskLevel sets risk level based on event type and actor
func (r *CorrelationResult) SetRiskLevel(eventType string) {
	switch eventType {
	case "DELETED":
		r.RiskLevel = "MEDIUM"
	case "MODIFIED":
		if r.ActorType == "CMS_USER" {
			r.RiskLevel = "LOW"
		} else {
			r.RiskLevel = "HIGH"
		}
	case "CREATED":
		if r.ActorType == "CMS_USER" {
			r.RiskLevel = "LOW"
		} else {
			r.RiskLevel = "MEDIUM"
		}
	default:
		r.RiskLevel = "LOW"
	}
}
