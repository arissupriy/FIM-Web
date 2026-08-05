// Package ojs provides OJS-specific template implementation.
package ojs

import (
	"context"
	"path/filepath"

	"ojs-monitor/backend/internal/domain/template"
	"ojs-monitor/backend/internal/templates/ojs/mysql"
)

// correlateFile correlates a file change event with OJS database.
func correlateFile(ctx context.Context, db template.DBConnection, filePath string, eventType string) (*template.CorrelationResult, error) {
	result := template.NewCorrelationResult(filePath, eventType)
	result.Classification = "OJS_WORKFLOW"

	ojsConn, ok := db.(*mysql.Connection)
	if !ok {
		result.Reason = "Invalid database connection"
		return result, nil
	}

	baseName := filepath.Base(filePath)

	// Query OJS database for file info
	info, err := ojsConn.GetFileInfo(ctx, baseName)
	if err == nil && info.UserID > 0 {
		result.Found = true
		result.ActorType = "CMS_USER"
		result.ActorID = itoa(info.UserID)
		result.ActorName = info.Username
		result.ActorEmail = info.Email
		result.SubmissionID = itoa(info.SubmissionID)
		result.Classification = "OJS_WORKFLOW"
		result.RiskLevel = "LOW"
		result.Reason = "File found in OJS submission_files"
	} else {
		result.Reason = "File not found in OJS submission_files"
	}

	// Adjust risk level based on event type
	switch eventType {
	case "DELETED":
		result.RiskLevel = "MEDIUM"
	case "MODIFIED":
		if result.Found {
			result.RiskLevel = "LOW"
		} else {
			result.RiskLevel = "HIGH"
		}
	case "CREATED":
		if result.Found {
			result.RiskLevel = "LOW"
		} else {
			result.RiskLevel = "MEDIUM"
		}
	}

	return result, nil
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
