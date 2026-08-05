// Package ojs provides OJS-specific service implementation.
package ojs

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

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
)

// Service provides OJS-specific operations.
type Service struct{}

// NewService creates a new OJS service.
func NewService() *Service {
	return &Service{}
}

// Config holds OJS database configuration.
type Config struct {
	DBHost  string
	DBUser  string
	DBPass  string
	DBName  string
}

// Connect establishes a connection to the OJS MySQL database.
func (s *Service) Connect(cfg Config) (*sql.DB, error) {
	// Extract host and port if host includes port
	hostParts := strings.Split(cfg.DBHost, ":")
	host := hostParts[0]
	port := "3306"
	if len(hostParts) > 1 {
		port = hostParts[1]
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s&readTimeout=30s&writeTimeout=30s",
		cfg.DBUser, cfg.DBPass, host, port, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection with timeout
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("connection timeout: %v", err)
	}
	conn.Close()

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping failed: %v", err)
	}

	return db, nil
}

// GetDetails retrieves OJS site details.
func (s *Service) GetDetails(ctx context.Context, db *sql.DB) (*models.OJSDetails, error) {
	var d models.OJSDetails

	// Site info
	db.QueryRowContext(ctx, "SELECT COALESCE(min_password_length, 6) FROM site").Scan(&d.MinPasswordLen)
	db.QueryRowContext(ctx, "SELECT COALESCE(primary_locale, 'en') FROM site").Scan(&d.PrimaryLocale)
	db.QueryRowContext(ctx, "SELECT COALESCE(installed_locales, '[\"en\"]') FROM site").Scan(&d.InstalledLocales)

	// Count journals
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM journals").Scan(&d.Jurournals)

	// Count users
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&d.Users)

	// Count submissions
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions").Scan(&d.Submissions)

	// Count published articles
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE status = 3").Scan(&d.Articles)

	// Count pending review assignments
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM review_assignments WHERE status = 1").Scan(&d.ReviewAssignments)

	return &d, nil
}

// DetectVersion reads the version from OJS version.xml file.
func (s *Service) DetectVersion(appPaths []string) string {
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

// FileRelation represents a file's relation to OJS database.
type FileRelation struct {
	FileID         int    `json:"file_id"`
	OriginalName  string `json:"original_name"`
	SubmissionID  int    `json:"submission_id"`
	ArticleTitle  string `json:"article_title"`
	AuthorName    string `json:"author_name"`
	UploaderID    int    `json:"uploader_user_id"`
	UploaderName  string `json:"uploader_name"`
	UploaderEmail string `json:"uploader_email"`
	FileType      string `json:"file_type"`
	DateUploaded  string `json:"date_uploaded"`
	StageID       int    `json:"stage_id"`
	ReviewRound   int    `json:"review_round"`
	Revision      int    `json:"revision"`
}

// GetFileRelations retrieves OJS file relations for a given file path.
func (s *Service) GetFileRelations(ctx context.Context, db *sql.DB, filePath string) ([]FileRelation, error) {
	basename := filepath.Base(filePath)
	var relations []FileRelation

	query := `
		SELECT
			sf.file_id, sf.original_file_name, sf.submission_id, sf.uploader_user_id,
			sf.file_type, sf.date_uploaded, sf.stage_id, sf.revision,
			COALESCE(u.username, 'unknown') as uploader_name,
			COALESCE(u.email, '') as uploader_email,
			COALESCE(sfr.round, 0) as review_round
		FROM submission_files sf
		LEFT JOIN users u ON u.user_id = sf.uploader_user_id
		LEFT JOIN (
			SELECT file_id, MAX(review_round_id) as round
			FROM review_round_files
			GROUP BY file_id
		) sfr ON sfr.file_id = sf.file_id
		WHERE sf.original_file_name = ?`

	rows, err := db.QueryContext(ctx, query, basename)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r FileRelation
		if err := rows.Scan(
			&r.FileID, &r.OriginalName, &r.SubmissionID, &r.UploaderID,
			&r.FileType, &r.DateUploaded, &r.StageID, &r.Revision,
			&r.UploaderName, &r.UploaderEmail, &r.ReviewRound,
		); err != nil {
			continue
		}

		// Get article title
		db.QueryRowContext(ctx,
			`SELECT COALESCE(ps.setting_value, CONCAT('Submission #', ?))
			 FROM submissions s
			 JOIN publications pub ON pub.submission_id = s.submission_id
			 LEFT JOIN publication_settings ps ON ps.publication_id = pub.publication_id AND ps.setting_name = 'title'
			 WHERE s.submission_id = ? LIMIT 1`,
			r.SubmissionID, r.SubmissionID).Scan(&r.ArticleTitle)

		// Get author
		db.QueryRowContext(ctx,
			`SELECT COALESCE(CONCAT(pa.given_name, ' ', pa.family_name), 'Unknown')
			 FROM publication_authors pa WHERE pa.submission_id = ? LIMIT 1`,
			r.SubmissionID).Scan(&r.AuthorName)

		relations = append(relations, r)
	}

	if relations == nil {
		relations = []FileRelation{}
	}

	return relations, nil
}

// DBMetrics holds database-derived metrics.
type DBMetrics struct {
	ActiveAdmins       int
	NewUsers           int
	ValidatedUsers     int
	UnvalidatedDisabled int
	UploadsByNewUsers  int
	BadSelfReg         int
}

// GetMetrics retrieves OJS database metrics.
func (s *Service) GetMetrics(ctx context.Context, db *sql.DB) (*DBMetrics, error) {
	var m DBMetrics

	// NEW_USERS (last 24 hours)
	db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE date_registered >= NOW() - INTERVAL 1 DAY").Scan(&m.NewUsers)

	// VALIDATED_USERS
	db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE disabled = 0 AND date_validated IS NOT NULL").Scan(&m.ValidatedUsers)

	// UNVALIDATED_DISABLED
	db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE disabled = 1 OR date_validated IS NULL").Scan(&m.UnvalidatedDisabled)

	// ACTIVE_ADMINS (last 7 days)
	db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT user_id) FROM users WHERE user_id IN (SELECT user_id FROM user_user_groups WHERE user_group_id IN (1, 16)) AND date_last_login >= NOW() - INTERVAL 7 DAY").Scan(&m.ActiveAdmins)

	// UPLOADS_BY_NEW_USERS
	db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT sf.file_id)
		FROM submission_files sf
		JOIN users u ON u.user_id = sf.uploader_user_id
		WHERE u.date_registered >= NOW() - INTERVAL 1 DAY`).Scan(&m.UploadsByNewUsers)

	// BAD_SELF_REG (user groups allowing self-registration)
	db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)`).Scan(&m.BadSelfReg)

	return &m, nil
}

// ValidateIntegrity runs OJS-specific integrity checks.
func (s *Service) ValidateIntegrity(ctx context.Context, db *sql.DB) ([]template.IntegrityWarning, error) {
	var warnings []template.IntegrityWarning

	// Check self-registration enabled
	var selfRegCount int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_groups
		WHERE permit_self_registration = 1
		  AND role_id NOT IN (65536, 1048576)`).Scan(&selfRegCount)
	if err == nil && selfRegCount > 0 {
		warnings = append(warnings, template.IntegrityWarning{
			Level:   template.WarningMedium,
			Code:    "OJS_SELF_REGISTRATION",
			Message: "Self-registration enabled for non-default roles",
			Details: fmt.Sprintf("%d user group(s) allow self-registration", selfRegCount),
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
			Details: fmt.Sprintf("%d user(s) have not validated their email", unvalidated),
		})
	}

	return warnings, nil
}
