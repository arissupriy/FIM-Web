// Package alert contains tests for alert use cases.
package alert

import (
	"context"
	"testing"

	"ojs-monitor/backend/internal/domain/models"
)

// MockAlertConfigRepository is a mock implementation of AlertConfigRepository.
type MockAlertConfigRepository struct {
	configs   map[int]*models.AlertConfig
	nextID    int
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

func NewMockAlertConfigRepository() *MockAlertConfigRepository {
	return &MockAlertConfigRepository{
		configs: make(map[int]*models.AlertConfig),
		nextID:  1,
	}
}

func (m *MockAlertConfigRepository) Create(ctx context.Context, config *models.AlertConfig) (int, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	config.ID = m.nextID
	m.configs[m.nextID] = config
	m.nextID++
	return config.ID, nil
}

func (m *MockAlertConfigRepository) GetByID(ctx context.Context, id int) (*models.AlertConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	config, ok := m.configs[id]
	if !ok {
		return nil, nil
	}
	return config, nil
}

func (m *MockAlertConfigRepository) ListByProject(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*models.AlertConfig
	for _, config := range m.configs {
		if config.ProjectID == projectID {
			result = append(result, config)
		}
	}
	return result, nil
}

func (m *MockAlertConfigRepository) ListEnabled(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*models.AlertConfig
	for _, config := range m.configs {
		if config.ProjectID == projectID && config.Enabled {
			result = append(result, config)
		}
	}
	return result, nil
}

func (m *MockAlertConfigRepository) Update(ctx context.Context, config *models.AlertConfig) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.configs[config.ID] = config
	return nil
}

func (m *MockAlertConfigRepository) Delete(ctx context.Context, id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.configs, id)
	return nil
}

func (m *MockAlertConfigRepository) Enable(ctx context.Context, id int) error {
	if config, ok := m.configs[id]; ok {
		config.Enabled = true
	}
	return nil
}

func (m *MockAlertConfigRepository) Disable(ctx context.Context, id int) error {
	if config, ok := m.configs[id]; ok {
		config.Enabled = false
	}
	return nil
}

// MockAlertHistoryRepository is a mock implementation of AlertHistoryRepository.
type MockAlertHistoryRepository struct {
	history   []*models.AlertHistory
	nextID    int64
	listErr   error
	createErr error
}

func NewMockAlertHistoryRepository() *MockAlertHistoryRepository {
	return &MockAlertHistoryRepository{
		history: make([]*models.AlertHistory, 0),
		nextID:  1,
	}
}

func (m *MockAlertHistoryRepository) Create(ctx context.Context, history *models.AlertHistory) (int, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	history.ID = int(m.nextID)
	m.history = append(m.history, history)
	m.nextID++
	return history.ID, nil
}

func (m *MockAlertHistoryRepository) GetByID(ctx context.Context, id int) (*models.AlertHistory, error) {
	for _, h := range m.history {
		if int(h.ID) == id {
			return h, nil
		}
	}
	return nil, nil
}

func (m *MockAlertHistoryRepository) ListByConfig(ctx context.Context, configID int, limit int) ([]*models.AlertHistory, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := []*models.AlertHistory{}
	for _, h := range m.history {
		if int(h.AlertConfigID) == configID {
			result = append(result, h)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockAlertHistoryRepository) ListByProject(ctx context.Context, projectID int, limit int) ([]*models.AlertHistory, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := []*models.AlertHistory{}
	for _, h := range m.history {
		if h.ProjectID == projectID {
			result = append(result, h)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockAlertHistoryRepository) UpdateStatus(ctx context.Context, id int, status models.AlertHistoryStatus, errorMsg string) error {
	for _, h := range m.history {
		if int(h.ID) == id {
			h.Status = status
			h.ErrorMessage = errorMsg
			break
		}
	}
	return nil
}

func (m *MockAlertHistoryRepository) MarkSent(ctx context.Context, id int) error {
	for _, h := range m.history {
		if int(h.ID) == id {
			h.Status = models.AlertStatusSent
			break
		}
	}
	return nil
}

func (m *MockAlertHistoryRepository) IncrementRetry(ctx context.Context, id int) error {
	return nil
}

func (m *MockAlertHistoryRepository) CheckDedup(ctx context.Context, projectID int, filePath string, riskLevel string, dedupWindow int) (bool, error) {
	return false, nil
}

func (m *MockAlertHistoryRepository) DeleteOld(ctx context.Context, days int) error {
	return nil
}

// Tests

func TestCreateAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	tests := []struct {
		name    string
		input   *CreateAlertConfigInput
		wantErr error
	}{
		{
			name: "valid email config",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "Test Alert",
				Channel:   "email",
				Config:    `{"recipients":["test@example.com"]}`,
				RiskLevel: "HIGH",
				Enabled:   true,
			},
			wantErr: nil,
		},
		{
			name: "valid slack config",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "Slack Alert",
				Channel:   "slack",
				Config:    `{"webhook_url":"https://hooks.slack.com/test"}`,
				RiskLevel: "CRITICAL",
				Enabled:   true,
			},
			wantErr: nil,
		},
		{
			name: "invalid project ID",
			input: &CreateAlertConfigInput{
				ProjectID: 0,
				Name:      "Test",
				Channel:   "email",
				Config:    `{}`,
				RiskLevel: "HIGH",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty name",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "",
				Channel:   "email",
				Config:    `{}`,
				RiskLevel: "HIGH",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "unsupported channel",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "Test",
				Channel:   "sms",
				Config:    `{}`,
				RiskLevel: "HIGH",
			},
			wantErr: ErrChannelNotSupported,
		},
		{
			name: "invalid risk level",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "Test",
				Channel:   "email",
				Config:    `{}`,
				RiskLevel: "INVALID",
			},
			wantErr: ErrInvalidInput,
		},
		{
			name: "empty config",
			input: &CreateAlertConfigInput{
				ProjectID: 1,
				Name:      "Test",
				Channel:   "email",
				Config:    "",
				RiskLevel: "HIGH",
			},
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := uc.CreateAlertConfig(ctx, tt.input)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("CreateAlertConfig() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("CreateAlertConfig() unexpected error = %v", err)
				return
			}
			if config.ID == 0 {
				t.Error("CreateAlertConfig() config.ID should not be 0")
			}
			if config.Name != tt.input.Name {
				t.Errorf("CreateAlertConfig() config.Name = %v, want %v", config.Name, tt.input.Name)
			}
		})
	}
}

func TestGetAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Test Alert",
		Channel:   "email",
		Config:    `{"recipients":["test@example.com"]}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("get existing config", func(t *testing.T) {
		config, err := uc.GetAlertConfig(ctx, created.ID)
		if err != nil {
			t.Errorf("GetAlertConfig() error = %v", err)
			return
		}
		if config.ID != created.ID {
			t.Errorf("GetAlertConfig() config.ID = %v, want %v", config.ID, created.ID)
		}
	})

	t.Run("get non-existent config", func(t *testing.T) {
		_, err := uc.GetAlertConfig(ctx, 9999)
		if err != ErrAlertConfigNotFound {
			t.Errorf("GetAlertConfig() error = %v, want %v", err, ErrAlertConfigNotFound)
		}
	})
}

func TestListAlertConfigs(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create configs for different projects
	for i := 0; i < 3; i++ {
		input := &CreateAlertConfigInput{
			ProjectID: 1,
			Name:      "Test Alert",
			Channel:   "email",
			Config:    `{}`,
			RiskLevel: "HIGH",
			Enabled:   true,
		}
		_, err := uc.CreateAlertConfig(ctx, input)
		if err != nil {
			t.Fatalf("CreateAlertConfig() error = %v", err)
		}
	}

	t.Run("list configs for project 1", func(t *testing.T) {
		configs, err := uc.ListAlertConfigs(ctx, 1)
		if err != nil {
			t.Errorf("ListAlertConfigs() error = %v", err)
			return
		}
		if len(configs) != 3 {
			t.Errorf("ListAlertConfigs() returned %d configs, want 3", len(configs))
		}
	})

	t.Run("list configs for project 2", func(t *testing.T) {
		configs, err := uc.ListAlertConfigs(ctx, 2)
		if err != nil {
			t.Errorf("ListAlertConfigs() error = %v", err)
			return
		}
		if len(configs) != 0 {
			t.Errorf("ListAlertConfigs() returned %d configs, want 0", len(configs))
		}
	})
}

func TestUpdateAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Original Name",
		Channel:   "email",
		Config:    `{"recipients":["old@example.com"]}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("update name", func(t *testing.T) {
		newName := "Updated Name"
		update := &UpdateAlertConfigInput{
			Name: newName,
		}
		updated, err := uc.UpdateAlertConfig(ctx, created.ID, update)
		if err != nil {
			t.Errorf("UpdateAlertConfig() error = %v", err)
			return
		}
		if updated.Name != newName {
			t.Errorf("UpdateAlertConfig() Name = %v, want %v", updated.Name, newName)
		}
	})

	t.Run("update to invalid channel", func(t *testing.T) {
		update := &UpdateAlertConfigInput{
			Channel: "sms",
		}
		_, err := uc.UpdateAlertConfig(ctx, created.ID, update)
		if err != ErrChannelNotSupported {
			t.Errorf("UpdateAlertConfig() error = %v, want %v", err, ErrChannelNotSupported)
		}
	})

	t.Run("update non-existent config", func(t *testing.T) {
		update := &UpdateAlertConfigInput{
			Name: "Test",
		}
		_, err := uc.UpdateAlertConfig(ctx, 9999, update)
		if err != ErrAlertConfigNotFound {
			t.Errorf("UpdateAlertConfig() error = %v, want %v", err, ErrAlertConfigNotFound)
		}
	})
}

func TestDeleteAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Test Alert",
		Channel:   "email",
		Config:    `{}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("delete existing config", func(t *testing.T) {
		err := uc.DeleteAlertConfig(ctx, created.ID)
		if err != nil {
			t.Errorf("DeleteAlertConfig() error = %v", err)
			return
		}
		// Verify it's gone
		_, err = uc.GetAlertConfig(ctx, created.ID)
		if err != ErrAlertConfigNotFound {
			t.Errorf("GetAlertConfig() after delete error = %v, want %v", err, ErrAlertConfigNotFound)
		}
	})

	t.Run("delete non-existent config", func(t *testing.T) {
		err := uc.DeleteAlertConfig(ctx, 9999)
		if err != ErrAlertConfigNotFound {
			t.Errorf("DeleteAlertConfig() error = %v, want %v", err, ErrAlertConfigNotFound)
		}
	})
}

func TestEnableDisableAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Test Alert",
		Channel:   "email",
		Config:    `{}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("disable enabled config", func(t *testing.T) {
		err := uc.DisableAlertConfig(ctx, created.ID)
		if err != nil {
			t.Errorf("DisableAlertConfig() error = %v", err)
			return
		}
		config, _ := uc.GetAlertConfig(ctx, created.ID)
		if config.Enabled {
			t.Error("DisableAlertConfig() config.Enabled should be false")
		}
	})

	t.Run("enable disabled config", func(t *testing.T) {
		err := uc.EnableAlertConfig(ctx, created.ID)
		if err != nil {
			t.Errorf("EnableAlertConfig() error = %v", err)
			return
		}
		config, _ := uc.GetAlertConfig(ctx, created.ID)
		if !config.Enabled {
			t.Error("EnableAlertConfig() config.Enabled should be true")
		}
	})
}

func TestIsValidChannel(t *testing.T) {
	tests := []struct {
		channel string
		valid   bool
	}{
		{"email", true},
		{"slack", true},
		{"webhook", true},
		{"sms", false},
		{"push", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			if got := isValidChannel(tt.channel); got != tt.valid {
				t.Errorf("isValidChannel(%q) = %v, want %v", tt.channel, got, tt.valid)
			}
		})
	}
}

func TestIsValidRiskLevel(t *testing.T) {
	tests := []struct {
		level string
		valid bool
	}{
		{"LOW", true},
		{"MEDIUM", true},
		{"HIGH", true},
		{"CRITICAL", true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			if got := isValidRiskLevel(tt.level); got != tt.valid {
				t.Errorf("isValidRiskLevel(%q) = %v, want %v", tt.level, got, tt.valid)
			}
		})
	}
}

func TestGetAlertHistory(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Test Alert",
		Channel:   "email",
		Config:    `{}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("get history for config", func(t *testing.T) {
		history, err := uc.GetAlertHistory(ctx, created.ID, 50)
		if err != nil {
			t.Errorf("GetAlertHistory() error = %v", err)
			return
		}
		// Empty list is valid, just not nil
		if history == nil {
			t.Error("GetAlertHistory() returned nil, expected empty slice")
		}
	})
}

func TestListProjectAlertHistory(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	t.Run("get project history", func(t *testing.T) {
		history, err := uc.ListProjectAlertHistory(ctx, 1, 50)
		if err != nil {
			t.Errorf("ListProjectAlertHistory() error = %v", err)
			return
		}
		// Empty list is valid, just not nil
		if history == nil {
			t.Error("ListProjectAlertHistory() returned nil, expected empty slice")
		}
	})
}

func TestGetAlertStats(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	t.Run("get stats for project", func(t *testing.T) {
		stats, err := uc.GetAlertStats(ctx, 1)
		if err != nil {
			t.Errorf("GetAlertStats() error = %v", err)
			return
		}
		if stats == nil {
			t.Error("GetAlertStats() returned nil")
		}
		if stats.ProjectID != 1 {
			t.Errorf("GetAlertStats() ProjectID = %v, want 1", stats.ProjectID)
		}
	})
}

func TestTestAlertConfig(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	// Create a config first
	input := &CreateAlertConfigInput{
		ProjectID: 1,
		Name:      "Test Alert",
		Channel:   "email",
		Config:    `{}`,
		RiskLevel: "HIGH",
		Enabled:   true,
	}
	created, err := uc.CreateAlertConfig(ctx, input)
	if err != nil {
		t.Fatalf("CreateAlertConfig() error = %v", err)
	}

	t.Run("test existing config", func(t *testing.T) {
		err := uc.TestAlertConfig(ctx, created.ID)
		if err != nil {
			t.Errorf("TestAlertConfig() error = %v", err)
		}
	})

	t.Run("test non-existent config", func(t *testing.T) {
		err := uc.TestAlertConfig(ctx, 9999)
		if err != ErrAlertConfigNotFound {
			t.Errorf("TestAlertConfig() error = %v, want %v", err, ErrAlertConfigNotFound)
		}
	})
}

func TestDefaultDedupWindow(t *testing.T) {
	ctx := context.Background()
	configRepo := NewMockAlertConfigRepository()
	historyRepo := NewMockAlertHistoryRepository()
	uc := NewUsecase(configRepo, historyRepo)

	t.Run("default dedup window is 60 seconds", func(t *testing.T) {
		input := &CreateAlertConfigInput{
			ProjectID: 1,
			Name:      "Test Alert",
			Channel:   "email",
			Config:    `{}`,
			RiskLevel: "HIGH",
			Enabled:   true,
			// DedupWindow not set
		}
		config, err := uc.CreateAlertConfig(ctx, input)
		if err != nil {
			t.Fatalf("CreateAlertConfig() error = %v", err)
		}
		if config.DedupWindow != 60 {
			t.Errorf("CreateAlertConfig() DedupWindow = %v, want 60", config.DedupWindow)
		}
	})

	t.Run("custom dedup window", func(t *testing.T) {
		input := &CreateAlertConfigInput{
			ProjectID:   1,
			Name:        "Test Alert",
			Channel:     "email",
			Config:      `{}`,
			RiskLevel:   "HIGH",
			Enabled:     true,
			DedupWindow: 120,
		}
		config, err := uc.CreateAlertConfig(ctx, input)
		if err != nil {
			t.Fatalf("CreateAlertConfig() error = %v", err)
		}
		if config.DedupWindow != 120 {
			t.Errorf("CreateAlertConfig() DedupWindow = %v, want 120", config.DedupWindow)
		}
	})
}
