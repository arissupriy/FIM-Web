// Package ojs provides OJS-specific scanner operations.
package ojs

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"ojs-monitor/backend/internal/infrastructure/database/mysql"
)

// ScanConfig holds configuration for scanner operations.
type ScanConfig struct {
	DBHost     string
	DBUser     string
	DBPass     string
	DBName     string
	AppPaths   []string
	FilesPaths []string
}

// ScanMetrics holds OJS-specific metrics from database.
type ScanMetrics struct {
	// User metrics
	NewUsers            int
	ValidatedUsers      int
	UnvalidatedDisabled int

	// Activity metrics
	ActiveAdmins      int
	UploadsByNewUsers int
	BadSelfReg        int

	// Content metrics
	Journals          int
	Submissions       int
	Articles          int
	ReviewAssignments int
}

// SystemDetails holds OJS system information from database.
type SystemDetails struct {
	Version          string `json:"version"`
	Journals        int    `json:"journals"`
	Users           int    `json:"users"`
	Submissions     int    `json:"submissions"`
	Articles        int    `json:"articles"`
	ReviewAssignments int  `json:"review_assignments"`
	PrimaryLocale   string `json:"primary_locale"`
	MinPasswordLen  int    `json:"min_password_len"`
}

// GetScanMetrics retrieves database metrics for OJS.
func GetScanMetrics(ctx context.Context, db *mysql.OJS) (*ScanMetrics, error) {
	var m ScanMetrics

	// NEW_USERS (last 24 hours)
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.NewUsers)

	// VALIDATED_USERS (last 24 hours)
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE disabled = 0 AND date_validated >= NOW() - INTERVAL 1 DAY").Scan(&m.ValidatedUsers)

	// UNVALIDATED_DISABLED (last 24 hours)
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE (disabled = 1 OR date_validated IS NULL) AND date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.UnvalidatedDisabled)

	// ACTIVE_ADMINS (last 7 days)
	db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM users
		WHERE user_id IN (SELECT user_id FROM user_user_groups WHERE user_group_id IN (1, 16))
		AND date_last_login >= NOW() - INTERVAL 7 DAY`).Scan(&m.ActiveAdmins)

	// UPLOADS_BY_NEW_USERS (last 24 hours)
	db.DB().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT sf.file_id)
		FROM submission_files sf
		JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE u.date_registered >= NOW() - INTERVAL 1 DAY`).Scan(&m.UploadsByNewUsers)

	// BAD_SELF_REG (user groups allowing self-registration)
	db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)`).Scan(&m.BadSelfReg)

	return &m, nil
}

// GetSystemDetails retrieves system details from OJS database.
func GetSystemDetails(ctx context.Context, db *mysql.OJS, appPaths []string) (*SystemDetails, error) {
	var d SystemDetails

	// Site info
	db.DB().QueryRowContext(ctx,
		"SELECT COALESCE(min_password_length, 6) FROM site").Scan(&d.MinPasswordLen)
	db.DB().QueryRowContext(ctx,
		"SELECT COALESCE(primary_locale, 'en') FROM site").Scan(&d.PrimaryLocale)

	// Count journals
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM journals").Scan(&d.Journals)

	// Count users
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users").Scan(&d.Users)

	// Count submissions
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions").Scan(&d.Submissions)

	// Count published articles
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE status = 3").Scan(&d.Articles)

	// Count pending review assignments
	db.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM review_assignments WHERE status = 1").Scan(&d.ReviewAssignments)

	// Detect version from filesystem
	d.Version = DetectVersion(appPaths)

	return &d, nil
}

// DetectVersion reads the version from OJS version.xml file.
func DetectVersion(appPaths []string) string {
	versionPaths := []string{
		"dbscripts/xml/version.xml",
		"registry/version.xml",
		"lib/pkp/classes/version.xml",
	}

	for _, ap := range appPaths {
		if ap == "" {
			continue
		}
		for _, vp := range versionPaths {
			fullPath := filepath.Join(ap, vp)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			contentStr := string(content)

			// Try to extract <release>X.X.X.X</release>
			if idx := strings.Index(contentStr, "<release>"); idx != -1 {
				start := idx + len("<release>")
				end := strings.Index(contentStr[start:], "</release>")
				if end != -1 {
					release := contentStr[start : start+end]
					return "OJS " + release
				}
			}

			// Fallback: try to extract from tag <tag>3_5_0-4</tag>
			if idx := strings.Index(contentStr, "<tag>"); idx != -1 {
				start := idx + len("<tag>")
				end := strings.Index(contentStr[start:], "</tag>")
				if end != -1 {
					tag := contentStr[start : start+end]
					tag = strings.ReplaceAll(tag, "_", ".")
					return "OJS " + tag
				}
			}
		}
	}

	return "OJS 3.x (detected)"
}

// OrphanResult represents a file that's not in the OJS database.
type OrphanResult struct {
	Path        string
	OriginalName string
	IsUpload    bool
}

// FindOrphans checks files against OJS database to find orphans.
func FindOrphans(ctx context.Context, db *sql.DB, files []string, filesPaths []string) ([]OrphanResult, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var orphans []OrphanResult

	for _, filePath := range files {
		// Only check files in files_paths (uploads)
		inFilesPath := false
		for _, fp := range filesPaths {
			if fp != "" && strings.HasPrefix(filePath, fp) {
				inFilesPath = true
				break
			}
		}
		if !inFilesPath {
			continue
		}

		baseName := filepath.Base(filePath)
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM submission_files WHERE original_file_name = ? LIMIT 1",
			baseName).Scan(&count)

		if err == sql.ErrNoRows || count == 0 {
			orphans = append(orphans, OrphanResult{
				Path:         filePath,
				OriginalName: baseName,
				IsUpload:     true,
			})
		}
	}

	return orphans, nil
}
