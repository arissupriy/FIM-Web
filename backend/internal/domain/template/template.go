// Package template defines interfaces for CMS-specific detection strategies.
// Templates enable platform to detect orphan files, validate integrity, and provide metrics
// specific to each CMS (OJS, WordPress, Drupal, etc.)
package template

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/infrastructure/database/mysql"
)

// Template represents a CMS-specific detection strategy.
// Implement this interface to add support for new CMS platforms.
type Template interface {
	// Name returns the template identifier (e.g., "ojs", "wordpress")
	Name() string

	// Version returns supported versions (e.g., "3.x", "2.x" for OJS)
	Version() string

	// Priority returns detection priority (higher = checked first)
	// Use 100 for primary CMS, lower for fallbacks
	Priority() int

	// DetectOrphans finds files not tracked in CMS database.
	// Called during reconciliation to flag untracked uploads as orphans.
	DetectOrphans(ctx context.Context, db *mysql.OJS, files []*models.ProjectFile) ([]*models.ProjectFile, error)

	// GetMetrics returns CMS-specific dashboard metrics.
	// These are merged with generic FIM metrics.
	GetMetrics(ctx context.Context, db *mysql.OJS) (*TemplateMetrics, error)

	// ValidateIntegrity checks CMS-specific integrity rules.
	// Return warnings or errors for policy violations.
	ValidateIntegrity(ctx context.Context, db *mysql.OJS, project *models.Project) ([]IntegrityWarning, error)

	// RequiredDBConfig returns required database configuration fields.
	// Used to validate project setup.
	RequiredDBConfig() []string

	// Compatible returns true if this template works with the given database schema.
	Compatible(ctx context.Context, db *mysql.OJS) (bool, error)
}

// TemplateMetrics holds CMS-specific metrics merged with generic metrics
type TemplateMetrics struct {
	TemplateName string                 `json:"template_name"`
	Version    string                  `json:"version"`
	Generic    *models.DashboardMetrics `json:"generic"`
	Specific  map[string]interface{}    `json:"specific"` // CMS-specific metrics
}

// IntegrityWarning represents a policy violation warning
type IntegrityWarning struct {
	Level   WarningLevel // LOW, MEDIUM, HIGH, CRITICAL
	Code    string      // Warning code for filtering
	Message string      // Human-readable message
	Details string      // Additional context
}

// WarningLevel represents warning severity
type WarningLevel string

const (
	WarningLow      WarningLevel = "LOW"
	WarningMedium  WarningLevel = "MEDIUM"
	WarningHigh    WarningLevel = "HIGH"
	WarningCritical WarningLevel = "CRITICAL"
)

// OJS-specific warning codes
const (
	WarningSelfRegistration   = "OJS_SELF_REGISTRATION"
	WarningWeakPassword     = "OJS_WEAK_PASSWORD"
	WarningUnvalidatedUsers = "OJS_UNVALIDATED_USERS"
	WarningOrphanFiles     = "OJS_ORPHAN_FILES"
	WarningPermissionChange = "PERMISSION_CHANGE"
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
