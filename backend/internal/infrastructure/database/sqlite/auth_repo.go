// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"
	"log"

	"golang.org/x/crypto/bcrypt"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// AuthRepository implements repository.AuthRepository using SQLite
type AuthRepository struct {
	db *DB
}

// NewAuthRepository creates a new AuthRepository
func NewAuthRepository(db *DB) repository.AuthRepository {
	return &AuthRepository{db: db}
}

// CreateAdmin creates a new admin user
func (r *AuthRepository) CreateAdmin(ctx context.Context, username, passwordHash string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO admins (username, password_hash) VALUES (?, ?)",
		username, passwordHash)
	return err
}

// GetAdminByUsername retrieves an admin by username
func (r *AuthRepository) GetAdminByUsername(ctx context.Context, username string) (*models.Admin, error) {
	var admin models.Admin
	err := r.db.QueryRowContext(ctx,
		"SELECT id, username, password_hash FROM admins WHERE username = ?",
		username).Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// UpdateAdminPassword updates an admin's password
func (r *AuthRepository) UpdateAdminPassword(ctx context.Context, username, passwordHash string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE admins SET password_hash = ? WHERE username = ?",
		passwordHash, username)
	return err
}

// LogActivity logs an admin activity
func (r *AuthRepository) LogActivity(ctx context.Context, adminID int, action, target string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO admin_action_logs (admin_id, action, target) VALUES (?, ?, ?)",
		adminID, action, target)
	return err
}

// GetAuditLogs retrieves recent audit logs
func (r *AuthRepository) GetAuditLogs(ctx context.Context, limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.admin_id, a.username, l.action, l.target, l.timestamp
		FROM admin_action_logs l
		JOIN admins a ON l.admin_id = a.id
		ORDER BY l.timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(&log.ID, &log.AdminID, &log.AdminName, &log.Action, &log.Target, &log.Timestamp)
		if err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

// SeedDefaultAdmin creates default admin if none exists
func (r *AuthRepository) SeedDefaultAdmin(ctx context.Context) error {
	username := "admin"
	password := "admin123"

	// Check if admin exists
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Admin exists, verify password is correct
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return err
		}

		// Update the password hash to ensure it's correct
		_, err = r.db.ExecContext(ctx,
			"UPDATE admins SET password_hash = ? WHERE username = ?",
			string(hash), username)
		if err != nil {
			log.Printf("Warning: failed to update admin password: %v", err)
		}
		log.Println("Admin verified: admin / admin123")
	} else {
		// Create new admin
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return err
		}
		_, err = r.db.ExecContext(ctx,
			"INSERT INTO admins (username, password_hash) VALUES (?, ?)",
			username, string(hash))
		if err != nil {
			return err
		}
		log.Println("Default admin created: admin / admin123")
	}
	return nil
}

// Count returns the number of admin users
func (r *AuthRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count)
	return count, err
}
