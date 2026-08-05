// Package sqlite provides SQLite database implementation.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// AlertConfigRepository implements repository.AlertConfigRepository using SQLite
type AlertConfigRepository struct {
	db *DB
}

// NewAlertConfigRepository creates a new AlertConfigRepository
func NewAlertConfigRepository(db *DB) repository.AlertConfigRepository {
	return &AlertConfigRepository{db: db}
}

// Create creates a new alert config
func (r *AlertConfigRepository) Create(ctx context.Context, config *models.AlertConfig) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO alert_configs (project_id, name, channel, config, conditions, risk_level, enabled, dedup_window, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		config.ProjectID, config.Name, config.Channel, config.Config, config.Conditions,
		config.RiskLevel, boolToInt(config.Enabled), config.DedupWindow, time.Now().Unix(), time.Now().Unix())
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// GetByID retrieves an alert config by ID
func (r *AlertConfigRepository) GetByID(ctx context.Context, id int) (*models.AlertConfig, error) {
	var config models.AlertConfig
	err := r.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, channel, config, conditions, risk_level, enabled, dedup_window, created_at, updated_at
		FROM alert_configs WHERE id = ?`, id).Scan(
		&config.ID, &config.ProjectID, &config.Name, &config.Channel, &config.Config, &config.Conditions,
		&config.RiskLevel, &config.Enabled, &config.DedupWindow, &config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListByProject retrieves all alert configs for a project
func (r *AlertConfigRepository) ListByProject(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, channel, config, conditions, risk_level, enabled, dedup_window, created_at, updated_at
		FROM alert_configs WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.AlertConfig
	for rows.Next() {
		var config models.AlertConfig
		err := rows.Scan(&config.ID, &config.ProjectID, &config.Name, &config.Channel, &config.Config, &config.Conditions,
			&config.RiskLevel, &config.Enabled, &config.DedupWindow, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &config)
	}
	return configs, rows.Err()
}

// ListEnabled retrieves all enabled alert configs for a project
func (r *AlertConfigRepository) ListEnabled(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, channel, config, conditions, risk_level, enabled, dedup_window, created_at, updated_at
		FROM alert_configs WHERE project_id = ? AND enabled = 1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.AlertConfig
	for rows.Next() {
		var config models.AlertConfig
		err := rows.Scan(&config.ID, &config.ProjectID, &config.Name, &config.Channel, &config.Config, &config.Conditions,
			&config.RiskLevel, &config.Enabled, &config.DedupWindow, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, &config)
	}
	return configs, rows.Err()
}

// Update updates an alert config
func (r *AlertConfigRepository) Update(ctx context.Context, config *models.AlertConfig) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alert_configs SET name = ?, channel = ?, config = ?, conditions = ?, risk_level = ?, enabled = ?, dedup_window = ?, updated_at = ?
		WHERE id = ?`,
		config.Name, config.Channel, config.Config, config.Conditions, config.RiskLevel,
		boolToInt(config.Enabled), config.DedupWindow, time.Now().Unix(), config.ID)
	return err
}

// Delete deletes an alert config
func (r *AlertConfigRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM alert_configs WHERE id = ?`, id)
	return err
}

// Enable enables an alert config
func (r *AlertConfigRepository) Enable(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE alert_configs SET enabled = 1, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// Disable disables an alert config
func (r *AlertConfigRepository) Disable(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE alert_configs SET enabled = 0, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// AlertHistoryRepository implements repository.AlertHistoryRepository using SQLite
type AlertHistoryRepository struct {
	db *DB
}

// NewAlertHistoryRepository creates a new AlertHistoryRepository
func NewAlertHistoryRepository(db *DB) repository.AlertHistoryRepository {
	return &AlertHistoryRepository{db: db}
}

// Create creates a new alert history entry
func (r *AlertHistoryRepository) Create(ctx context.Context, history *models.AlertHistory) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO alert_history (alert_config_id, fim_event_id, project_id, channel, status, retry_count, max_retries, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		history.AlertConfigID, history.FIMEventID, history.ProjectID, history.Channel,
		history.Status, history.RetryCount, history.MaxRetries, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

// GetByID retrieves an alert history entry by ID
func (r *AlertHistoryRepository) GetByID(ctx context.Context, id int) (*models.AlertHistory, error) {
	var history models.AlertHistory
	var alertConfigID, fimEventID sql.NullInt64
	var sentAt sql.NullInt64
	var errorMessage sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT id, alert_config_id, fim_event_id, project_id, channel, status, retry_count, max_retries, error_message, sent_at, created_at
		FROM alert_history WHERE id = ?`, id).Scan(
		&history.ID, &alertConfigID, &fimEventID, &history.ProjectID, &history.Channel,
		&history.Status, &history.RetryCount, &history.MaxRetries, &errorMessage, &sentAt, &history.CreatedAt)
	if err != nil {
		return nil, err
	}

	if alertConfigID.Valid {
		history.AlertConfigID = int(alertConfigID.Int64)
	}
	if fimEventID.Valid {
		history.FIMEventID = int(fimEventID.Int64)
	}
	if sentAt.Valid {
		history.SentAt = sentAt.Int64
	}
	if errorMessage.Valid {
		history.ErrorMessage = errorMessage.String
	}

	return &history, nil
}

// ListByConfig retrieves alert history for a config
func (r *AlertHistoryRepository) ListByConfig(ctx context.Context, configID int, limit int) ([]*models.AlertHistory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, alert_config_id, fim_event_id, project_id, channel, status, retry_count, max_retries, error_message, sent_at, created_at
		FROM alert_history WHERE alert_config_id = ? ORDER BY created_at DESC LIMIT ?`, configID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []*models.AlertHistory
	for rows.Next() {
		var h models.AlertHistory
		err := rows.Scan(&h.ID, &h.AlertConfigID, &h.FIMEventID, &h.ProjectID, &h.Channel,
			&h.Status, &h.RetryCount, &h.MaxRetries, &h.ErrorMessage, &h.SentAt, &h.CreatedAt)
		if err != nil {
			return nil, err
		}
		histories = append(histories, &h)
	}
	return histories, rows.Err()
}

// ListByProject retrieves alert history for a project
func (r *AlertHistoryRepository) ListByProject(ctx context.Context, projectID int, limit int) ([]*models.AlertHistory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, alert_config_id, fim_event_id, project_id, channel, status, retry_count, max_retries, error_message, sent_at, created_at
		FROM alert_history WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []*models.AlertHistory
	for rows.Next() {
		var h models.AlertHistory
		var alertConfigID, fimEventID sql.NullInt64
		var sentAt sql.NullInt64
		var errorMessage sql.NullString

		err := rows.Scan(&h.ID, &alertConfigID, &fimEventID, &h.ProjectID, &h.Channel,
			&h.Status, &h.RetryCount, &h.MaxRetries, &errorMessage, &sentAt, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		if alertConfigID.Valid {
			h.AlertConfigID = int(alertConfigID.Int64)
		}
		if fimEventID.Valid {
			h.FIMEventID = int(fimEventID.Int64)
		}
		if sentAt.Valid {
			h.SentAt = sentAt.Int64
		}
		if errorMessage.Valid {
			h.ErrorMessage = errorMessage.String
		}

		histories = append(histories, &h)
	}
	return histories, rows.Err()
}

// UpdateStatus updates the status of an alert history entry
func (r *AlertHistoryRepository) UpdateStatus(ctx context.Context, id int, status models.AlertHistoryStatus, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alert_history SET status = ?, error_message = ? WHERE id = ?`,
		status, errorMsg, id)
	return err
}

// MarkSent marks an alert as sent
func (r *AlertHistoryRepository) MarkSent(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alert_history SET status = 'sent', sent_at = ? WHERE id = ?`,
		time.Now().Unix(), id)
	return err
}

// IncrementRetry increments retry count
func (r *AlertHistoryRepository) IncrementRetry(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alert_history SET retry_count = retry_count + 1, status = 'retry' WHERE id = ?`, id)
	return err
}

// CheckDedup checks if an alert was recently sent
func (r *AlertHistoryRepository) CheckDedup(ctx context.Context, projectID int, filePath string, riskLevel string, dedupWindow int) (bool, error) {
	var count int
	cutoff := time.Now().Unix() - int64(dedupWindow)
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alert_history
		WHERE project_id = ? AND status = 'sent'
		AND created_at >= ?`,
		projectID, cutoff).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return count > 0, nil
}

// DeleteOld deletes alert history older than specified days
func (r *AlertHistoryRepository) DeleteOld(ctx context.Context, days int) error {
	cutoff := time.Now().Unix() - int64(days*24*60*60)
	_, err := r.db.ExecContext(ctx, `DELETE FROM alert_history WHERE created_at < ?`, cutoff)
	return err
}

// Helper function to convert bool to int for SQLite
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
