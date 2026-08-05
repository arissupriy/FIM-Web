// Package ojs provides OJS-specific scanning operations.
package ojs

import (
	"context"

	"ojs-monitor/backend/internal/templates/ojs/mysql"
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
	Journals    int
	Submissions int
	Articles    int
}

// SystemDetails is an alias for mysql.SystemDetails for backward compatibility.
type SystemDetails = mysql.SystemDetails

// GetScanMetrics retrieves database metrics for OJS.
func GetScanMetrics(ctx context.Context, conn *mysql.Connection) (*ScanMetrics, error) {
	m, err := conn.GetMetrics(ctx)
	if err != nil {
		return nil, err
	}

	return &ScanMetrics{
		NewUsers:            m.NewUsers,
		ValidatedUsers:      m.ValidatedUsers,
		UnvalidatedDisabled: m.UnvalidatedDisabled,
		ActiveAdmins:       m.ActiveAdmins,
		UploadsByNewUsers:  m.UploadsByNewUsers,
		BadSelfReg:         m.BadSelfReg,
	}, nil
}

// GetSystemDetails retrieves system details from OJS database.
func GetSystemDetails(ctx context.Context, conn *mysql.Connection, appPaths []string) (*SystemDetails, error) {
	details, err := conn.GetSystemDetails(ctx)
	if err != nil {
		return nil, err
	}

	// Detect version from filesystem
	details.Version = DetectVersion(appPaths)

	return details, nil
}
