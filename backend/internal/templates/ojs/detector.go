// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"context"
	"database/sql"
	"path/filepath"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/infrastructure/database/mysql"
)

// Detector implements template.Template for OJS.
type Detector struct{}

// New creates a new OJS template detector.
func New() *Detector {
	return &Detector{}
}

// Name returns the template identifier.
func (d *Detector) Name() string {
	return "ojs"
}

// Version returns supported versions.
func (d *Detector) Version() string {
	return "2.x/3.x"
}

// Priority returns detection priority (higher = checked first).
func (d *Detector) Priority() int {
	return 100
}

// RequiredDBConfig returns required database configuration fields.
func (d *Detector) RequiredDBConfig() []string {
	return []string{"db_host", "db_user", "db_pass", "db_name"}
}

// Compatible checks if database contains OJS schema.
func (d *Detector) Compatible(ctx context.Context, db *mysql.OJS) (bool, error) {
	tables := []string{"journals", "users", "submissions"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table).Scan(&count)
		if err != nil {
			return false, nil
		}
		if count == 0 {
			return false, nil
		}
	}
	return true, nil
}

// DetectOrphans finds files not tracked in OJS submission_files table.
func (d *Detector) DetectOrphans(ctx context.Context, db *mysql.OJS, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var orphans []*models.ProjectFile
	for _, f := range files {
		// Skip non-upload files
		if f.FileType != "uploads" {
			continue
		}

		baseName := filepath.Base(f.FilePath)
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM submission_files WHERE original_file_name = ? LIMIT 1",
			baseName).Scan(&count)
		if err == sql.ErrNoRows || count == 0 {
			f.Status = "ORPHAN"
			orphans = append(orphans, f)
		}
	}

	return orphans, nil
}

// GetMetrics returns OJS-specific dashboard metrics.
func (d *Detector) GetMetrics(ctx context.Context, db *mysql.OJS) (*template.TemplateMetrics, error) {
	m := template.NewTemplateMetrics("ojs", "3.x")
	m.Generic = &models.DashboardMetrics{}
	m.Specific = make(map[string]interface{})

	// Total users
	var users int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users); err == nil {
		m.Specific["total_users"] = users
	}

	// Total submissions
	var submissions int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions").Scan(&submissions); err == nil {
		m.Specific["total_submissions"] = submissions
	}

	return m, nil
}

// ValidateIntegrity checks OJS-specific security policies.
func (d *Detector) ValidateIntegrity(ctx context.Context, db *mysql.OJS, p *models.Project) ([]template.IntegrityWarning, error) {
	var warnings []template.IntegrityWarning

	// Check self-registration
	var selfRegCount int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)
	`).Scan(&selfRegCount)
	if err == nil && selfRegCount > 0 {
		warnings = append(warnings, template.IntegrityWarning{
			Level:   template.WarningMedium,
			Code:    template.WarningSelfRegistration,
			Message: "Self-registration enabled for non-default roles",
			Details: "Users can self-register as reviewer/editor",
		})
	}

	return warnings, nil
}
