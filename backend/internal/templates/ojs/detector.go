// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"os"
	"path/filepath"
	"strings"
)

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

// IsOJSPath checks if a path looks like an OJS installation.
func IsOJSPath(path string) bool {
	requiredDirs := []string{
		"lib/pkp",
		"plugins",
		"public",
	}

	for _, dir := range requiredDirs {
		if _, err := os.Stat(filepath.Join(path, dir)); err == nil {
			return true
		}
	}

	return false
}
