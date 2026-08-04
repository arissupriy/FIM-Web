// Package scanner provides OJS (Open Journal Systems) file reconciliation
// and database metrics collection.
package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"ojs-monitor/backend/internal/wire"
)

// connectMySQL establishes a connection to an OJS MySQL database.
func ConnectMySQL(user, pass, host, dbName string) (*sql.DB, error) {
	// Extract host and port if host includes port
	hostParts := strings.Split(host, ":")
	actualHost := hostParts[0]
	port := "3306"
	if len(hostParts) > 1 {
		port = hostParts[1]
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s&readTimeout=30s&writeTimeout=30s",
		user, pass, actualHost, port, dbName)

	mysqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	mysqlDB.SetMaxOpenConns(5)
	mysqlDB.SetMaxIdleConns(2)
	mysqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection with timeout
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(actualHost, port), 5*time.Second)
	if err != nil {
		mysqlDB.Close()
		return nil, fmt.Errorf("connection timeout: %v", err)
	}
	conn.Close()

	if err := mysqlDB.Ping(); err != nil {
		mysqlDB.Close()
		return nil, fmt.Errorf("ping failed: %v", err)
	}

	return mysqlDB, nil
}

// FastAuditProject collects dashboard metrics for a project.
func FastAuditProject(ctx context.Context, p wire.LegacyProject) (DashboardMetrics, error) {
	var metrics DashboardMetrics

	// 1. Fast FIM Metrics from SQLite using wire
	added, modified, deleted, orphan, err := wire.GetFileStats(ctx, p.ID)
	if err == nil {
		metrics.NewFilesCount = added
		metrics.ModifiedFilesCount = modified
		metrics.DeletedFilesCount = deleted
		metrics.OrphanFilesCount = orphan
	} else {
		fmt.Printf("Warning: Failed to query FIM metrics: %v\n", err)
	}

	// 2. Query Database Metrics (Fast)
	dbMetrics, err := QueryDBMetrics(ctx, p)
	if err != nil {
		return metrics, fmt.Errorf("failed to query db metrics: %v", err)
	}

	metrics.ActiveAdmins = dbMetrics.ActiveAdmins
	metrics.NewUsers = dbMetrics.NewUsers
	metrics.ValidatedUsers = dbMetrics.ValidatedUsers
	metrics.UnvalidatedDisabled = dbMetrics.UnvalidatedDisabled
	metrics.UploadsByNewUsers = dbMetrics.UploadsByNewUsers
	metrics.BadSelfReg = dbMetrics.BadSelfReg

	return metrics, nil
}

// ReconcileOJSFiles checks files against OJS database to find orphans.
func ReconcileOJSFiles(ctx context.Context, p wire.LegacyProject, added []wire.LegacyProjectFile, modified []wire.LegacyProjectFile) ([]wire.LegacyProjectFile, error) {
	if len(added) == 0 && len(modified) == 0 {
		return nil, nil
	}

	mysqlDB, err := ConnectMySQL(p.DBUser, p.DBPass, p.DBHost, p.DBName)
	if err != nil {
		return nil, err
	}
	defer mysqlDB.Close()

	var orphans []wire.LegacyProjectFile
	filesToCheck := append(added, modified...)

	for _, f := range filesToCheck {
		// Only reconcile files in files_paths (uploads)
		inFilesPath := false
		for _, fp := range p.FilesPaths {
			if fp != "" && strings.HasPrefix(f.FilePath, fp) {
				inFilesPath = true
				break
			}
		}
		if !inFilesPath {
			continue // Skip checking app source code files against OJS DB
		}

		baseName := filepath.Base(f.FilePath)
		count := 0
		err := mysqlDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM submission_files WHERE original_file_name = ? LIMIT 1
		`, baseName).Scan(&count)

		if err != nil || count == 0 {
			f.Status = "ORPHAN"
			orphans = append(orphans, f)
		}
	}

	return orphans, nil
}

// QueryDBMetrics collects user and submission metrics from OJS MySQL.
func QueryDBMetrics(ctx context.Context, p wire.LegacyProject) (DashboardMetrics, error) {
	var m DashboardMetrics

	mysqlDB, err := ConnectMySQL(p.DBUser, p.DBPass, p.DBHost, p.DBName)
	if err != nil {
		return m, err
	}
	defer mysqlDB.Close()

	// NEW_USERS
	err = mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.NewUsers)
	if err != nil {
		return m, err
	}

	// VALIDATED_USERS
	err = mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE date_validated >= NOW() - INTERVAL 1 DAY").Scan(&m.ValidatedUsers)
	if err != nil {
		return m, err
	}

	// UNVALIDATED_DISABLED
	err = mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE disabled = 1 AND date_validated IS NULL AND date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.UnvalidatedDisabled)
	if err != nil {
		return m, err
	}

	// UPLOADS_BY_NEW_USERS
	err = mysqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM submission_files sf
		JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE u.date_registered >= NOW() - INTERVAL 1 DAY
	`).Scan(&m.UploadsByNewUsers)
	if err != nil && err != sql.ErrNoRows {
		// table submission_files might not exist depending on OJS version, ignore error if missing table or just return 0
		m.UploadsByNewUsers = 0
	}

	// ACTIVE_ADMINS
	err = mysqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users u
		JOIN user_user_groups uug ON uug.user_id=u.user_id
		JOIN user_groups ug ON ug.user_group_id=uug.user_group_id
		WHERE ug.role_id=1 AND u.disabled=0
	`).Scan(&m.ActiveAdmins)
	if err != nil {
		return m, err
	}

	// BAD_SELF_REG
	err = mysqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_groups ug
		JOIN user_group_settings ugs ON ugs.user_group_id=ug.user_group_id
		WHERE ug.permit_self_registration=1
		  AND ug.role_id NOT IN (65536,1048576)
		  AND ugs.setting_name='name'
		  AND ugs.locale='en'
	`).Scan(&m.BadSelfReg)
	if err != nil {
		return m, err
	}

	return m, nil
}

// OJSDetails holds OJS system information.
type OJSDetails struct {
	Version          string `json:"version"`
	Jurournals       int    `json:"journals"`
	Users            int    `json:"users"`
	Submissions      int    `json:"submissions"`
	Articles         int    `json:"articles"`
	ReviewAssignments int  `json:"review_assignments"`
	PrimaryLocale    string `json:"primary_locale"`
	InstalledLocales string `json:"installed_locales"`
	MinPasswordLen   int    `json:"min_password_len"`
}

// GetOJSDetails collects OJS system details from database and filesystem.
func GetOJSDetails(ctx context.Context, p wire.LegacyProject) (OJSDetails, error) {
	var d OJSDetails

	mysqlDB, err := ConnectMySQL(p.DBUser, p.DBPass, p.DBHost, p.DBName)
	if err != nil {
		return d, err
	}
	defer mysqlDB.Close()

	// Site info
	mysqlDB.QueryRowContext(ctx, "SELECT COALESCE(min_password_length, 6) FROM site").Scan(&d.MinPasswordLen)
	mysqlDB.QueryRowContext(ctx, "SELECT COALESCE(primary_locale, 'en') FROM site").Scan(&d.PrimaryLocale)
	mysqlDB.QueryRowContext(ctx, "SELECT COALESCE(installed_locales, '[\"en\"]') FROM site").Scan(&d.InstalledLocales)

	// Count journals
	mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM journals").Scan(&d.Jurournals)

	// Count users
	mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&d.Users)

	// Count submissions
	mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions").Scan(&d.Submissions)

	// Count published articles
	mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE status = 3").Scan(&d.Articles)

	// Count pending review assignments
	mysqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM review_assignments WHERE status = 1").Scan(&d.ReviewAssignments)

	// Try to get version from file system (most accurate)
	d.Version = DetectOJSVersion(p.AppPaths)

	return d, nil
}

// DetectOJSVersion reads the version from OJS version.xml file.
func DetectOJSVersion(appPaths []string) string {
	// Try multiple paths to find version.xml
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

			// Try to extract <release>X.X.X.X</release> or <tag>X_X_X-X</tag>
			// Pattern: <release>3.5.0.4</release>
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
					// Convert 3_5_0-4 to 3.5.0-4
					tag = strings.ReplaceAll(tag, "_", ".")
					tag = strings.ReplaceAll(tag, "-", "-")
					return "OJS " + tag
				}
			}
		}
	}

	// Fallback to database detection
	return "OJS 3.x (detected)"
}

// DashboardMetrics holds FIM and database metrics for dashboard.
type DashboardMetrics struct {
	// FIM Metrics
	NewFilesCount       int
	ModifiedFilesCount  int
	DeletedFilesCount   int
	OrphanFilesCount    int

	// OJS Database Metrics
	ActiveAdmins        int
	NewUsers            int
	ValidatedUsers      int
	UnvalidatedDisabled int
	UploadsByNewUsers   int
	BadSelfReg          int
}
