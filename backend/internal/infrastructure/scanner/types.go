// Package scanner provides generic file integrity monitoring scanner.
// This package is CMS-agnostic - use templates for CMS-specific scanning.
package scanner

import (
	"ojs-monitor/backend/internal/domain/models"
)

// ScanResult represents the result of a file scan.
type ScanResult struct {
	// Path information
	Path string

	// File metadata
	Size    int64
	Mode    string
	UID     uint32
	GID     uint32

	// Hash
	Hash string

	// Classification
	Status        string // ADDED, MODIFIED, DELETED
	Classification string
}

// Scanner provides generic file scanning operations.
type Scanner interface {
	// ScanFiles scans files and returns their hash and metadata.
	ScanFiles(paths []string) ([]*ScanResult, error)

	// CalculateHash computes SHA256 hash of a file.
	CalculateHash(filePath string) (string, error)

	// GetMetadata extracts file metadata (size, mode, uid, gid).
	GetMetadata(filePath string) (size int64, mode string, uid uint32, gid uint32, err error)
}

// FileClassifier classifies files based on their path.
type FileClassifier interface {
	// ClassifyFile returns the file type based on its path.
	ClassifyFile(filePath string) string

	// IsBlacklisted checks if a file extension is blacklisted.
	IsBlacklisted(filePath string) bool

	// IsWhitelisted checks if a path is whitelisted.
	IsWhitelisted(filePath string) bool
}

// DefaultClassifier provides basic file classification.
type DefaultClassifier struct {
	BlacklistExts  []string
	WhitelistPaths []string
}

// NewDefaultClassifier creates a classifier with default settings.
func NewDefaultClassifier(blacklist []string, whitelist []string) *DefaultClassifier {
	if blacklist == nil {
		blacklist = []string{}
	}
	if whitelist == nil {
		whitelist = []string{}
	}
	return &DefaultClassifier{
		BlacklistExts:  blacklist,
		WhitelistPaths: whitelist,
	}
}

// ClassifyFile returns a simple classification based on path patterns.
func (c *DefaultClassifier) ClassifyFile(filePath string) string {
	// This is a basic classifier - templates can provide more specific classification
	return "project"
}

// IsBlacklisted checks if file extension is blacklisted.
func (c *DefaultClassifier) IsBlacklisted(filePath string) bool {
	ext := getExtension(filePath)
	for _, blacklisted := range c.BlacklistExts {
		if ext == blacklisted {
			return true
		}
	}
	return false
}

// IsWhitelisted checks if path matches any whitelist pattern.
func (c *DefaultClassifier) IsWhitelisted(filePath string) bool {
	for _, pattern := range c.WhitelistPaths {
		if hasPrefix(filePath, pattern) {
			return true
		}
	}
	return false
}

// getExtension extracts file extension.
func getExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			if i > 0 {
				return path[i+1:]
			}
			break
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

// hasPrefix checks if path starts with pattern.
func hasPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// CompareFiles compares a file against its baseline.
func CompareFiles(current *ScanResult, baseline *models.ProjectFile) (changed bool, details map[string]interface{}) {
	details = make(map[string]interface{})

	if current.Hash != baseline.Hash {
		details["hash_changed"] = true
		details["old_hash"] = baseline.Hash
		details["new_hash"] = current.Hash
		changed = true
	}

	if current.Size != baseline.FileSize {
		details["size_changed"] = true
		details["old_size"] = baseline.FileSize
		details["new_size"] = current.Size
		changed = true
	}

	if current.Mode != baseline.FileMode {
		details["mode_changed"] = true
		details["old_mode"] = baseline.FileMode
		details["new_mode"] = current.Mode
		changed = true
	}

	if current.UID != baseline.FileUID {
		details["uid_changed"] = true
		changed = true
	}

	if current.GID != baseline.FileGID {
		details["gid_changed"] = true
		changed = true
	}

	return changed, details
}
