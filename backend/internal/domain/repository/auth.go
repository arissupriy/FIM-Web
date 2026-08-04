// Package repository defines interfaces for data access.
package repository

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
)

// AuthRepository defines the interface for authentication data access
type AuthRepository interface {
	// CreateAdmin creates a new admin user
	CreateAdmin(ctx context.Context, username, passwordHash string) error

	// GetAdminByUsername retrieves an admin by username
	GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error)

	// UpdateAdminPassword updates an admin's password
	UpdateAdminPassword(ctx context.Context, username, passwordHash string) error

	// LogActivity logs an admin activity
	LogActivity(ctx context.Context, adminID int, action, target string) error

	// GetAuditLogs retrieves recent audit logs
	GetAuditLogs(ctx context.Context, limit int) ([]*models.AuditLog, error)
}

// SeedAdminFunc defines the function signature for seeding default admin
type SeedAdminFunc func() error
