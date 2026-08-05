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
	return "3.x"
}

// Priority returns detection priority (higher = checked first).
func (d *Detector) Priority() int {
	return 100
}

// DefaultConfig returns the default configuration for OJS projects.
func (d *Detector) DefaultConfig() *template.TemplateConfig {
	return &template.TemplateConfig{
		Template: "ojs",
		DefaultWatchPaths: []string{
			"public/",
			"lib/pkp/",
			"plugins/",
		},
		DefaultFilesPaths: []string{
			"files/",
		},
		DefaultBlacklistExts: []string{
			"php", "phtml", "php3", "php4", "php5", "php7", "pht", "phar",
			"sh", "bash", "zsh",
			"pl", "py", "rb",
			"exe", "bat", "cmd", "ps1",
		},
		DefaultWhitelistPaths: []string{
			"lib/pkp/classes/",           // OJS core classes
			"plugins/generic/",            // Trusted plugins
			"plugins/themes/",             // Trusted themes
		},
		DefaultRescanInterval: 10, // minutes
		WatchType:            "OJS_WORKFLOW",
		Settings: map[string]interface{}{
			"track_uploads": true,
		},
	}
}

// RequiredDBConfig returns required database configuration fields.
func (d *Detector) RequiredDBConfig() []string {
	return []string{"db_host", "db_user", "db_pass", "db_name"}
}

// Compatible checks if database contains OJS schema.
func (d *Detector) Compatible(ctx context.Context, db template.DBConnection) (bool, error) {
	// Cast to *sql.DB for information_schema query
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return false, nil
	}

	tables := []string{"journals", "users", "submissions"}
	for _, table := range tables {
		var count int
		err := sqlDB.QueryRowContext(ctx,
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
func (d *Detector) DetectOrphans(ctx context.Context, db template.DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
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
func (d *Detector) GetMetrics(ctx context.Context, db template.DBConnection) (*template.TemplateMetrics, error) {
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

	// New users (24h)
	var newUsers int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE date_registered >= NOW() - INTERVAL 1 DAY").Scan(&newUsers); err == nil {
		m.Specific["new_users_24h"] = newUsers
	}

	// Active admins (7d)
	var activeAdmins int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM users
		WHERE user_id IN (SELECT user_id FROM user_user_groups WHERE user_group_id IN (1, 16))
		AND date_last_login >= NOW() - INTERVAL 7 DAY`).Scan(&activeAdmins); err == nil {
		m.Specific["active_admins_7d"] = activeAdmins
	}

	return m, nil
}

// ValidateIntegrity checks OJS-specific security policies.
func (d *Detector) ValidateIntegrity(ctx context.Context, db template.DBConnection, p *models.Project) ([]template.IntegrityWarning, error) {
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
			Code:    "OJS_SELF_REGISTRATION",
			Message: "Self-registration enabled for non-default roles",
			Details: "Users can self-register as reviewer/editor",
		})
	}

	// Check for unvalidated users
	var unvalidated int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE date_validated IS NULL").Scan(&unvalidated)
	if err == nil && unvalidated > 0 {
		warnings = append(warnings, template.IntegrityWarning{
			Level:   template.WarningLow,
			Code:    "OJS_UNVALIDATED_USERS",
			Message: "Users without email validation",
			Details: "Some users have not validated their email",
		})
	}

	return warnings, nil
}

// CorrelateFile correlates a file change event with OJS database.
// This replaces the correlateOJS function in watcher.go.
func (d *Detector) CorrelateFile(ctx context.Context, db template.DBConnection, filePath string, eventType string) (*template.CorrelationResult, error) {
	result := template.NewCorrelationResult(filePath, eventType)
	result.Classification = "OJS_WORKFLOW"

	baseName := filepath.Base(filePath)

	// Query OJS database for file info
	var userID int
	var username, email string
	var submissionID int

	err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(u.user_id, 0),
			COALESCE(u.username, 'unknown'),
			COALESCE(u.email, ''),
			COALESCE(sf.submission_id, 0)
		FROM submission_files sf
		LEFT JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE sf.original_file_name = ?
		LIMIT 1
	`, baseName).Scan(&userID, &username, &email, &submissionID)

	if err == nil && userID > 0 {
		result.Found = true
		result.ActorType = "CMS_USER"
		result.ActorID = string(rune(userID))
		result.ActorName = username
		result.ActorEmail = email
		result.SubmissionID = string(rune(submissionID))
		result.Classification = "OJS_WORKFLOW"
		result.Reason = "File found in OJS submission_files"
	} else {
		result.Reason = "File not found in OJS submission_files"
	}

	// Set risk level based on event type and actor
	result.SetRiskLevel(eventType)

	return result, nil
}

// Ensure mysql.OJS implements template.DBConnection for backward compatibility
var _ template.DBConnection = (*mysql.OJS)(nil)
