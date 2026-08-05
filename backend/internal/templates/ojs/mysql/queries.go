// Package mysql provides OJS-specific MySQL queries.
package mysql

import (
	"context"
)

// SiteInfo holds OJS site information.
type SiteInfo struct {
	MinPasswordLen int
	PrimaryLocale string
}

// GetSiteInfo retrieves OJS site information.
func (c *Connection) GetSiteInfo(ctx context.Context) (*SiteInfo, error) {
	var info SiteInfo
	err := c.db.QueryRowContext(ctx, `
		SELECT COALESCE(min_password_length, 6), COALESCE(primary_locale, 'en')
		FROM site`).Scan(&info.MinPasswordLen, &info.PrimaryLocale)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GetUserCount returns total number of users.
func (c *Connection) GetUserCount(ctx context.Context) (int, error) {
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetSubmissionCount returns total number of submissions.
func (c *Connection) GetSubmissionCount(ctx context.Context) (int, error) {
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions").Scan(&count)
	return count, err
}

// GetJournalCount returns total number of journals.
func (c *Connection) GetJournalCount(ctx context.Context) (int, error) {
	var count int
	err := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM journals").Scan(&count)
	return count, err
}

// FileInfo holds submission file information.
type FileInfo struct {
	UserID        int
	Username      string
	FullName      string
	Email         string
	SubmissionID  int
	Stage         string
}

// GetFileInfo retrieves file information from submission_files table.
func (c *Connection) GetFileInfo(ctx context.Context, fileName string) (*FileInfo, error) {
	var info FileInfo
	err := c.db.QueryRowContext(ctx, `
		SELECT COALESCE(u.user_id, 0), COALESCE(u.username, ''),
		       COALESCE(u.first_name || ' ' || u.last_name, ''),
		       COALESCE(u.email, ''), COALESCE(sf.submission_id, 0),
		       COALESCE(sf.file_stage, '')
		FROM submission_files sf
		LEFT JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE sf.original_file_name = ?
		LIMIT 1
	`, fileName).Scan(&info.UserID, &info.Username, &info.FullName, &info.Email, &info.SubmissionID, &info.Stage)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// FileExists checks if a file exists in submission_files.
func (c *Connection) FileExists(ctx context.Context, fileName string) (bool, error) {
	var count int
	err := c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submission_files WHERE original_file_name = ?",
		fileName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CheckCompatibility verifies OJS database schema.
func (c *Connection) CheckCompatibility(ctx context.Context) (bool, error) {
	tables := []string{"journals", "users", "submissions"}
	for _, table := range tables {
		var count int
		err := c.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table).Scan(&count)
		if err != nil {
			return false, err
		}
		if count == 0 {
			return false, nil
		}
	}
	return true, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Metrics Queries
// ─────────────────────────────────────────────────────────────────────────────

// Metrics holds OJS database metrics.
type Metrics struct {
	NewUsers            int
	ValidatedUsers      int
	UnvalidatedDisabled int
	ActiveAdmins       int
	UploadsByNewUsers  int
	BadSelfReg         int
}

// GetMetrics retrieves database metrics for OJS.
func (c *Connection) GetMetrics(ctx context.Context) (*Metrics, error) {
	var m Metrics

	// NEW_USERS (last 24 hours)
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.NewUsers)

	// VALIDATED_USERS (last 24 hours)
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE disabled = 0 AND date_validated >= NOW() - INTERVAL 1 DAY").Scan(&m.ValidatedUsers)

	// UNVALIDATED_DISABLED (last 24 hours)
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE (disabled = 1 OR date_validated IS NULL) AND date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.UnvalidatedDisabled)

	// ACTIVE_ADMINS (last 7 days)
	c.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM users
		WHERE user_id IN (SELECT user_id FROM user_user_groups WHERE user_group_id IN (1, 16))
		AND date_last_login >= NOW() - INTERVAL 7 DAY`).Scan(&m.ActiveAdmins)

	// UPLOADS_BY_NEW_USERS (last 24 hours)
	c.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT sf.file_id)
		FROM submission_files sf
		JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE u.date_registered >= NOW() - INTERVAL 1 DAY`).Scan(&m.UploadsByNewUsers)

	// BAD_SELF_REG (user groups allowing self-registration)
	c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)`).Scan(&m.BadSelfReg)

	return &m, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// System Details Queries
// ─────────────────────────────────────────────────────────────────────────────

// SystemDetails holds OJS system information.
type SystemDetails struct {
	Version           string `json:"version"`
	Journals         int    `json:"journals"`
	Users            int    `json:"users"`
	Submissions      int    `json:"submissions"`
	Articles         int    `json:"articles"`
	ReviewAssignments int   `json:"review_assignments"`
	PrimaryLocale   string `json:"primary_locale"`
	MinPasswordLen  int    `json:"min_password_len"`
}

// GetSystemDetails retrieves system details from OJS database.
func (c *Connection) GetSystemDetails(ctx context.Context) (*SystemDetails, error) {
	var d SystemDetails

	// Site info
	c.db.QueryRowContext(ctx,
		"SELECT COALESCE(min_password_length, 6) FROM site").Scan(&d.MinPasswordLen)
	c.db.QueryRowContext(ctx,
		"SELECT COALESCE(primary_locale, 'en') FROM site").Scan(&d.PrimaryLocale)

	// Count journals
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM journals").Scan(&d.Journals)

	// Count users
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users").Scan(&d.Users)

	// Count submissions
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions").Scan(&d.Submissions)

	// Count published articles
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE status = 3").Scan(&d.Articles)

	// Count pending review assignments
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM review_assignments WHERE status = 1").Scan(&d.ReviewAssignments)

	return &d, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Integrity Check Queries
// ─────────────────────────────────────────────────────────────────────────────

// IntegrityWarning holds OJS-specific integrity warnings.
type IntegrityWarning struct {
	SelfRegistration int
	UnvalidatedUsers int
}

// GetIntegrityWarnings retrieves OJS integrity warnings.
func (c *Connection) GetIntegrityWarnings(ctx context.Context) (*IntegrityWarning, error) {
	var w IntegrityWarning

	// Self-registration enabled
	c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)`).Scan(&w.SelfRegistration)

	// Unvalidated users
	c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE date_validated IS NULL").Scan(&w.UnvalidatedUsers)

	return &w, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Orphan Detection
// ─────────────────────────────────────────────────────────────────────────────

// CheckOrphan checks if a file is an orphan (not in submission_files).
func (c *Connection) CheckOrphan(ctx context.Context, fileName string) (bool, error) {
	exists, err := c.FileExists(ctx, fileName)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// GetOrphans returns all orphan files from a list of file names.
func (c *Connection) GetOrphans(ctx context.Context, fileNames []string) ([]string, error) {
	var orphans []string

	for _, name := range fileNames {
		isOrphan, err := c.CheckOrphan(ctx, name)
		if err != nil {
			continue
		}
		if isOrphan {
			orphans = append(orphans, name)
		}
	}

	return orphans, nil
}
