// Package scanner provides generic file scanning for FIM.
// This package is CMS-agnostic - all CMS-specific logic is in templates/.
package scanner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/wire"
)

// DashboardMetrics holds FIM and database metrics for dashboard.
type DashboardMetrics struct {
	// FIM Metrics (from SQLite)
	NewFilesCount      int
	ModifiedFilesCount int
	DeletedFilesCount  int
	OrphanFilesCount   int
}

// FastAuditProject collects FIM metrics from SQLite.
func FastAuditProject(ctx context.Context, projectID int) (DashboardMetrics, error) {
	var metrics DashboardMetrics

	added, modified, deleted, orphan, err := wire.GetFileStats(ctx, projectID)
	if err == nil {
		metrics.NewFilesCount = added
		metrics.ModifiedFilesCount = modified
		metrics.DeletedFilesCount = deleted
		metrics.OrphanFilesCount = orphan
	}

	return metrics, nil
}

// ScanDirectory scans a directory and returns all files.
func ScanDirectory(rootPath string, recursive bool) ([]string, error) {
	var files []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// CalculateFileHash computes SHA256 hash of a file.
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetFileInfo extracts file metadata.
func GetFileInfo(filePath string) (size int64, mode string, uid uint32, gid uint32, err error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, "", 0, 0, err
	}

	size = info.Size()
	mode = fmt.Sprintf("%04o", info.Mode().Perm())

	if sys := info.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			uid = stat.Uid
			gid = stat.Gid
		}
	}

	return size, mode, uid, gid, nil
}

// ScanFile performs a full scan of a single file.
func ScanFile(filePath string) (*ScanResult, error) {
	result := &ScanResult{Path: filePath}

	// Get file info
	size, mode, uid, gid, err := GetFileInfo(filePath)
	if err != nil {
		return nil, err
	}

	result.Size = size
	result.Mode = mode
	result.UID = uid
	result.GID = gid

	// Calculate hash for files under 10MB
	if size <= 10*1024*1024 {
		hash, err := CalculateFileHash(filePath)
		if err == nil {
			result.Hash = hash
		}
	}

	return result, nil
}

// ScanFiles scans multiple files.
func ScanFiles(paths []string) ([]*ScanResult, error) {
	var results []*ScanResult

	for _, path := range paths {
		result, err := ScanFile(path)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// ClassifyFile classifies a file based on its path.
func ClassifyFile(filePath string, filesPaths []string) string {
	// Check if in files/uploads path
	for _, fp := range filesPaths {
		if fp != "" && strings.HasPrefix(filePath, fp) {
			return "uploads"
		}
	}
	return "project"
}

// IsOrphan checks if a file is an orphan (not in CMS database).
// This is a stub - actual implementation uses CMS-specific queries via template.
func IsOrphan(filePath string, dbHost string) bool {
	// This should be replaced by template-specific implementation
	return false
}

// OJSLookupTimeout is the timeout for OJS database lookups.
const OJSLookupTimeout = 5 * time.Second

// Legacy types for backward compatibility with existing code

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

// ReconcileFiles uses the template system to detect orphan files.
// This is the generic reconciliation function - templates provide CMS-specific logic.
func ReconcileFiles(ctx context.Context, p *models.Project, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
	// Get template for this project
	t, ok := template.Get(p.Template)
	if !ok {
		// No template registered for this project type
		return nil, nil
	}

	// Create CMS database connection using template
	dbCfg := template.DBConnectionConfig{
		Host:     p.DBHost,
		User:     p.DBUser,
		Password: p.DBPass,
		DBName:   p.DBName,
		Timeout:  30,
	}

	cmsDB, err := t.CreateDBConnection(ctx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CMS database: %w", err)
	}
	defer cmsDB.Close()

	// Use template to detect orphans
	return t.DetectOrphans(ctx, cmsDB, files)
}
