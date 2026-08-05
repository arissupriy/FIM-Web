// Package ojs provides OJS-specific template implementation.
// All OJS-specific logic lives here - scanner/watcher/worker are generic.
package ojs

import (
	"context"
	"database/sql"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// Service implements template.Template for OJS.
// This is the single entry point for all OJS-specific operations.
type Service struct{}

// New creates a new OJS template service.
func New() *Service {
	return &Service{}
}

// Name returns the template identifier.
func (s *Service) Name() string {
	return "ojs"
}

// Version returns supported versions.
func (s *Service) Version() string {
	return "3.x"
}

// Priority returns detection priority (higher = checked first).
func (s *Service) Priority() int {
	return 100
}

// DefaultConfig returns the default configuration for OJS projects.
func (s *Service) DefaultConfig() *template.TemplateConfig {
	return getDefaultConfig()
}

// CreateDBConnection creates a database connection for OJS.
func (s *Service) CreateDBConnection(ctx context.Context, cfg template.DBConnectionConfig) (template.DBConnection, error) {
	timeout := 10
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	mysqlCfg := mysql.Config{
		Host:     cfg.Host,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		Timeout:  time.Duration(timeout) * time.Second,
	}

	return mysql.Connect(mysqlCfg)
}

// RequiredDBConfig returns required database configuration fields.
func (s *Service) RequiredDBConfig() []string {
	return []string{"host", "user", "password", "database"}
}

// Compatible checks if database contains OJS schema.
func (s *Service) Compatible(ctx context.Context, db template.DBConnection) (bool, error) {
	ojsConn, ok := db.(*mysql.Connection)
	if !ok {
		return false, nil
	}
	return ojsConn.CheckCompatibility(ctx)
}

// DetectOrphans finds files not tracked in OJS submission_files table.
func (s *Service) DetectOrphans(ctx context.Context, db template.DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
	return detectOrphans(ctx, db, files)
}

// GetMetrics returns OJS-specific dashboard metrics.
func (s *Service) GetMetrics(ctx context.Context, db template.DBConnection) (*template.TemplateMetrics, error) {
	return getMetrics(ctx, db)
}

// ValidateIntegrity checks OJS-specific security policies.
func (s *Service) ValidateIntegrity(ctx context.Context, db template.DBConnection, p *models.Project) ([]template.IntegrityWarning, error) {
	return validateIntegrity(ctx, db, p)
}

// CorrelateFile correlates a file change event with OJS database.
func (s *Service) CorrelateFile(ctx context.Context, db template.DBConnection, filePath string, eventType string) (*template.CorrelationResult, error) {
	return correlateFile(ctx, db, filePath, eventType)
}

// Ensure mysql.Connection implements template.DBConnection
var _ template.DBConnection = (*mysql.Connection)(nil)

// Ensure *Service implements template.Template
var _ template.Template = (*Service)(nil)

// UnwrapDBConnection extracts the underlying *sql.DB if it's a mysql.Connection.
// This is needed for direct mysql operations not covered by the interface.
func UnwrapDBConnection(db template.DBConnection) (*sql.DB, bool) {
	if conn, ok := db.(*mysql.Connection); ok {
		return conn.DB(), true
	}
	return nil, false
}
