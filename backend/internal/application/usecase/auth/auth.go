// Package auth provides authentication-related use cases.
package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserNotFound is returned when user doesn't exist.
var ErrUserNotFound = errors.New("user not found")

// UseCase handles authentication business logic.
type UseCase struct {
	authRepo repository.AuthRepository
}

// New creates a new auth use case.
func New(authRepo repository.AuthRepository) *UseCase {
	return &UseCase{authRepo: authRepo}
}

// Authenticate validates credentials and returns admin if valid.
func (uc *UseCase) Authenticate(ctx context.Context, username, password string) (*models.Admin, error) {
	admin, err := uc.authRepo.GetAdminByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return admin, nil
}

// GetAdminByUsername returns an admin by username.
func (uc *UseCase) GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error) {
	return uc.authRepo.GetAdminByUsername(ctx, username)
}

// CreateAdmin creates a new admin user.
func (uc *UseCase) CreateAdmin(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return uc.authRepo.CreateAdmin(ctx, username, string(hash))
}

// LogActivity logs an audit activity.
func (uc *UseCase) LogActivity(ctx context.Context, adminID int, action, target string) error {
	return uc.authRepo.LogActivity(ctx, adminID, action, target)
}

// GetAuditLogs returns audit logs.
func (uc *UseCase) GetAuditLogs(ctx context.Context, limit int) ([]*models.AuditLog, error) {
	return uc.authRepo.GetAuditLogs(ctx, limit)
}

// SeedDefaultAdmin creates default admin if not exists.
func (uc *UseCase) SeedDefaultAdmin(ctx context.Context) error {
	const (
		defaultUsername = "admin"
		defaultPassword = "admin123"
	)

	// Check if admin exists
	_, err := uc.authRepo.GetAdminByUsername(ctx, defaultUsername)
	if err == nil {
		// Admin exists, verify password
		return nil
	}

	// Create new admin
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), 10)
	if err != nil {
		return err
	}

	return uc.authRepo.CreateAdmin(ctx, defaultUsername, string(hash))
}
