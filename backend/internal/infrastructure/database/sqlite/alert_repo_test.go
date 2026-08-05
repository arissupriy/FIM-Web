// Package sqlite provides tests for alert repositories.
package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"ojs-monitor/backend/internal/domain/models"
)

// setupTestDB creates a temporary test database.
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Enable foreign keys
	db.Exec("PRAGMA foreign_keys = ON;")

	// Create tables matching actual migration schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS alert_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			channel TEXT NOT NULL,
			config TEXT NOT NULL,
			conditions TEXT NOT NULL DEFAULT '{}',
			risk_level TEXT NOT NULL DEFAULT 'LOW',
			enabled INTEGER NOT NULL DEFAULT 1,
			dedup_window INTEGER NOT NULL DEFAULT 60,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS alert_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_config_id INTEGER,
			fim_event_id INTEGER,
			project_id INTEGER NOT NULL,
			channel TEXT NOT NULL,
			status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'sent', 'failed', 'retry')),
			retry_count INTEGER DEFAULT 0,
			max_retries INTEGER DEFAULT 3,
			error_message TEXT DEFAULT '',
			response_body TEXT DEFAULT '',
			sent_at INTEGER,
			created_at INTEGER DEFAULT (strftime('%s', 'now'))
		);

		CREATE INDEX IF NOT EXISTS idx_alert_configs_project ON alert_configs(project_id);
		CREATE INDEX IF NOT EXISTS idx_alert_configs_enabled ON alert_configs(enabled);
		CREATE INDEX IF NOT EXISTS idx_alert_history_config ON alert_history(alert_config_id);
		CREATE INDEX IF NOT EXISTS idx_alert_history_status ON alert_history(status);
		CREATE INDEX IF NOT EXISTS idx_alert_history_created ON alert_history(created_at);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return db
}

func TestAlertConfigRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	config := &models.AlertConfig{
		ProjectID:  1,
		Name:      "Test Alert",
		Channel:   models.AlertChannelEmail,
		Config:    `{"recipients": ["test@example.com"]}`,
		Conditions: `{"event_types": ["MODIFIED"]}`,
		RiskLevel: models.RiskLevelHigh,
		Enabled:   true,
		DedupWindow: 60,
	}

	id, err := repo.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if id <= 0 {
		t.Error("expected positive ID")
	}

	config.ID = id

	// Verify it was created
	retrieved, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if retrieved.Name != "Test Alert" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "Test Alert")
	}
	if retrieved.Channel != models.AlertChannelEmail {
		t.Errorf("Channel = %q, want %q", retrieved.Channel, models.AlertChannelEmail)
	}
}

func TestAlertConfigRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	// Create config
	config := &models.AlertConfig{
		ProjectID:  1,
		Name:      "Original Name",
		Channel:   models.AlertChannelSlack,
		Config:    `{}`,
		Conditions: `{}`,
		RiskLevel: models.RiskLevelLow,
		Enabled:   true,
		DedupWindow: 30,
	}

	id, err := repo.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	config.ID = id

	// Update
	config.Name = "Updated Name"
	config.RiskLevel = models.RiskLevelCritical
	err = repo.Update(ctx, config)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify
	retrieved, _ := repo.GetByID(ctx, id)
	if retrieved.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "Updated Name")
	}
	if retrieved.RiskLevel != models.RiskLevelCritical {
		t.Errorf("RiskLevel = %q, want %q", retrieved.RiskLevel, models.RiskLevelCritical)
	}
}

func TestAlertConfigRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	config := &models.AlertConfig{
		ProjectID:  1,
		Name:      "To Delete",
		Channel:   models.AlertChannelWebhook,
		Config:    `{}`,
		Conditions: `{}`,
		RiskLevel: models.RiskLevelMedium,
		Enabled:   false,
	}

	id, _ := repo.Create(ctx, config)
	config.ID = id

	err := repo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = repo.GetByID(ctx, id)
	if err != sql.ErrNoRows {
		t.Error("expected ErrNoRows after delete")
	}
}

func TestAlertConfigRepository_ListByProject(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	// Create configs for two projects
	repo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Project 1 Config 1", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})
	repo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Project 1 Config 2", Channel: models.AlertChannelSlack,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelMedium, Enabled: true,
	})
	repo.Create(ctx, &models.AlertConfig{
		ProjectID: 2, Name: "Project 2 Config", Channel: models.AlertChannelWebhook,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelHigh, Enabled: true,
	})

	// List project 1
	configs, err := repo.ListByProject(ctx, 1)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}

	if len(configs) != 2 {
		t.Errorf("len(configs) = %d, want 2", len(configs))
	}

	// List project 2
	configs, err = repo.ListByProject(ctx, 2)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("len(configs) = %d, want 1", len(configs))
	}
}

func TestAlertConfigRepository_ListEnabled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	repo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Enabled", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})
	repo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Disabled", Channel: models.AlertChannelSlack,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: false,
	})

	configs, err := repo.ListEnabled(ctx, 1)
	if err != nil {
		t.Fatalf("ListEnabled() error = %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("len(configs) = %d, want 1", len(configs))
	}
	if configs[0].Name != "Enabled" {
		t.Errorf("Name = %q, want %q", configs[0].Name, "Enabled")
	}
}

func TestAlertConfigRepository_EnableDisable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertConfigRepository(NewDB(db))
	ctx := context.Background()

	id, _ := repo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	// Disable
	err := repo.Disable(ctx, id)
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	config, _ := repo.GetByID(ctx, id)
	if config.Enabled {
		t.Error("Expected Enabled = false")
	}

	// Enable
	err = repo.Enable(ctx, id)
	if err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	config, _ = repo.GetByID(ctx, id)
	if !config.Enabled {
		t.Error("Expected Enabled = true")
	}
}

// AlertHistory Repository Tests

func TestAlertHistoryRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	// First create a config (needed for FK)
	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, _ := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	history := &models.AlertHistory{
		AlertConfigID: configID,
		FIMEventID:    123,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusPending,
		MaxRetries:    3,
	}

	id, err := repo.Create(ctx, history)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if id <= 0 {
		t.Error("expected positive ID")
	}
}

func TestAlertHistoryRepository_CheckDedup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	// Create config
	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, err := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	// Create history entry with sent status
	_, err = repo.Create(ctx, &models.AlertHistory{
		AlertConfigID: configID,
		FIMEventID:    100,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusSent,
	})
	if err != nil {
		t.Fatalf("failed to create history: %v", err)
	}

	// Check dedup - implementation checks if any sent alert exists for project within window
	// Since we just created a sent alert, it should be a duplicate
	isDup, err := repo.CheckDedup(ctx, 1, "/var/www/file.php", "HIGH", 60)
	if err != nil {
		t.Fatalf("CheckDedup() error = %v", err)
	}
	if !isDup {
		t.Error("expected duplicate - any sent alert for project should be deduped")
	}

	// For different project, should not be duplicate
	isDup, err = repo.CheckDedup(ctx, 2, "/var/www/file.php", "HIGH", 60)
	if err != nil {
		t.Fatalf("CheckDedup() error = %v", err)
	}
	if isDup {
		t.Error("expected no duplicate for different project")
	}
}

func TestAlertHistoryRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	// Create config and history
	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, err := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	historyID, err := repo.Create(ctx, &models.AlertHistory{
		AlertConfigID: configID,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusPending,
	})
	if err != nil {
		t.Fatalf("failed to create history: %v", err)
	}

	// Update status
	err = repo.UpdateStatus(ctx, historyID, models.AlertStatusFailed, "connection refused")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	history, err := repo.GetByID(ctx, historyID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if history.Status != models.AlertStatusFailed {
		t.Errorf("Status = %q, want %q", history.Status, models.AlertStatusFailed)
	}
	if history.ErrorMessage != "connection refused" {
		t.Errorf("ErrorMessage = %q, want %q", history.ErrorMessage, "connection refused")
	}
}

func TestAlertHistoryRepository_MarkSent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, _ := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	historyID, _ := repo.Create(ctx, &models.AlertHistory{
		AlertConfigID: configID,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusPending,
	})

	err := repo.MarkSent(ctx, historyID)
	if err != nil {
		t.Fatalf("MarkSent() error = %v", err)
	}

	history, _ := repo.GetByID(ctx, historyID)
	if history.Status != models.AlertStatusSent {
		t.Errorf("Status = %q, want %q", history.Status, models.AlertStatusSent)
	}
	if history.SentAt == 0 {
		t.Error("expected SentAt to be set")
	}
}

func TestAlertHistoryRepository_IncrementRetry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, _ := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	historyID, _ := repo.Create(ctx, &models.AlertHistory{
		AlertConfigID: configID,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusPending,
		RetryCount:    0,
		MaxRetries:    3,
	})

	// Increment retry
	err := repo.IncrementRetry(ctx, historyID)
	if err != nil {
		t.Fatalf("IncrementRetry() error = %v", err)
	}

	history, _ := repo.GetByID(ctx, historyID)
	if history.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", history.RetryCount)
	}

	// Increment again
	repo.IncrementRetry(ctx, historyID)
	history, _ = repo.GetByID(ctx, historyID)
	if history.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", history.RetryCount)
	}
}

func TestAlertHistoryRepository_ListByProject(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, _ := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	// Create multiple history entries
	for i := 0; i < 3; i++ {
		repo.Create(ctx, &models.AlertHistory{
			AlertConfigID: configID,
			ProjectID:     1,
			Channel:       models.AlertChannelEmail,
			Status:        models.AlertStatusSent,
		})
	}

	histories, err := repo.ListByProject(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}

	if len(histories) != 3 {
		t.Errorf("len(histories) = %d, want 3", len(histories))
	}
}

func TestAlertHistoryRepository_DeleteOld(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewAlertHistoryRepository(NewDB(db))
	ctx := context.Background()

	configRepo := NewAlertConfigRepository(NewDB(db))
	configID, _ := configRepo.Create(ctx, &models.AlertConfig{
		ProjectID: 1, Name: "Test", Channel: models.AlertChannelEmail,
		Config: "{}", Conditions: "{}", RiskLevel: models.RiskLevelLow, Enabled: true,
	})

	// Create history
	repo.Create(ctx, &models.AlertHistory{
		AlertConfigID: configID,
		ProjectID:     1,
		Channel:       models.AlertChannelEmail,
		Status:        models.AlertStatusSent,
	})

	// Delete old (older than 30 days)
	err := repo.DeleteOld(ctx, 30)
	if err != nil {
		t.Fatalf("DeleteOld() error = %v", err)
	}
}
