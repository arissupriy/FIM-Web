// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"context"
	"path/filepath"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// detectOrphans finds files not tracked in OJS submission_files table.
// Only files in files/ paths (uploads) are checked.
func detectOrphans(ctx context.Context, db template.DBConnection, files []*models.ProjectFile) ([]*models.ProjectFile, error) {
	if len(files) == 0 {
		return nil, nil
	}

	ojsConn, ok := db.(*mysql.Connection)
	if !ok {
		return nil, nil
	}

	var orphans []*models.ProjectFile
	for _, f := range files {
		// Skip non-upload files
		if f.FileType != "uploads" {
			continue
		}

		baseName := filepath.Base(f.FilePath)
		isOrphan, err := ojsConn.CheckOrphan(ctx, baseName)
		if err == nil && isOrphan {
			f.Status = "ORPHAN"
			orphans = append(orphans, f)
		}
	}

	return orphans, nil
}
