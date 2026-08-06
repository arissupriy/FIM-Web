// Package alert contains alert configuration use cases.
package alert

import (
	"context"
	"errors"
	"time"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// Common errors
var (
	ErrAlertConfigNotFound = errors.New("alert config not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrChannelNotSupported = errors.New("channel not supported")
	ErrProjectMismatch     = errors.New("project ID mismatch")
)

// Supported channels
var SupportedChannels = []string{"email", "slack", "webhook"}

// CreateAlertConfigInput represents input for creating an alert config.
type CreateAlertConfigInput struct {
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	Channel     string `json:"channel"`
	Config      string `json:"config"`
	Conditions  string `json:"conditions,omitempty"`
	RiskLevel   string `json:"risk_level"`
	Enabled     bool   `json:"enabled"`
	DedupWindow int    `json:"dedup_window,omitempty"`
}

// UpdateAlertConfigInput represents input for updating an alert config.
type UpdateAlertConfigInput struct {
	ProjectID   int    `json:"project_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Config      string `json:"config,omitempty"`
	Conditions  string `json:"conditions,omitempty"`
	RiskLevel   string `json:"risk_level,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	DedupWindow int    `json:"dedup_window,omitempty"`
}

// AlertStats represents alert statistics for a project.
type AlertStats struct {
	ProjectID    int            `json:"project_id"`
	TotalSent    int            `json:"total_sent"`
	TotalFailed  int            `json:"total_failed"`
	TotalPending int            `json:"total_pending"`
	ByChannel    map[string]int `json:"by_channel"`
}

// Usecase provides alert configuration use cases.
type Usecase struct {
	configRepo  repository.AlertConfigRepository
	historyRepo repository.AlertHistoryRepository
}

// NewUsecase creates a new alert use case.
func NewUsecase(
	configRepo repository.AlertConfigRepository,
	historyRepo repository.AlertHistoryRepository,
) *Usecase {
	return &Usecase{
		configRepo:  configRepo,
		historyRepo: historyRepo,
	}
}

// CreateAlertConfig creates a new alert configuration.
func (u *Usecase) CreateAlertConfig(ctx context.Context, input *CreateAlertConfigInput) (*models.AlertConfig, error) {
	// Validate input
	if input.ProjectID <= 0 {
		return nil, ErrInvalidInput
	}
	if input.Name == "" {
		return nil, ErrInvalidInput
	}
	if !isValidChannel(input.Channel) {
		return nil, ErrChannelNotSupported
	}
	if !isValidRiskLevel(input.RiskLevel) {
		return nil, ErrInvalidInput
	}
	if input.Config == "" {
		return nil, ErrInvalidInput
	}

	// Set default dedup window
	dedupWindow := input.DedupWindow
	if dedupWindow <= 0 {
		dedupWindow = 60 // Default 60 seconds
	}

	config := &models.AlertConfig{
		ProjectID:   input.ProjectID,
		Name:        input.Name,
		Channel:     models.AlertChannel(input.Channel),
		Config:      input.Config,
		Conditions:  input.Conditions,
		RiskLevel:   models.RiskLevel(input.RiskLevel),
		Enabled:     input.Enabled,
		DedupWindow: dedupWindow,
		CreatedAt:   now(),
		UpdatedAt:   now(),
	}

	id, err := u.configRepo.Create(ctx, config)
	if err != nil {
		return nil, err
	}
	config.ID = id

	return config, nil
}

// GetAlertConfig retrieves an alert config by ID.
func (u *Usecase) GetAlertConfig(ctx context.Context, id int) (*models.AlertConfig, error) {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, ErrAlertConfigNotFound
	}
	return config, nil
}

// ListAlertConfigs lists all alert configs for a project.
func (u *Usecase) ListAlertConfigs(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	return u.configRepo.ListByProject(ctx, projectID)
}

// ListEnabledAlertConfigs lists all enabled alert configs for a project.
func (u *Usecase) ListEnabledAlertConfigs(ctx context.Context, projectID int) ([]*models.AlertConfig, error) {
	return u.configRepo.ListEnabled(ctx, projectID)
}

// UpdateAlertConfig updates an alert configuration.
func (u *Usecase) UpdateAlertConfig(ctx context.Context, id int, input *UpdateAlertConfigInput) (*models.AlertConfig, error) {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, ErrAlertConfigNotFound
	}

	// Validate and apply updates
	if input.ProjectID > 0 {
		if config.ProjectID != input.ProjectID {
			return nil, ErrProjectMismatch
		}
	}

	if input.Name != "" {
		config.Name = input.Name
	}

	if input.Channel != "" {
		if !isValidChannel(input.Channel) {
			return nil, ErrChannelNotSupported
		}
		config.Channel = models.AlertChannel(input.Channel)
	}

	if input.Config != "" {
		config.Config = input.Config
	}

	if input.Conditions != "" {
		config.Conditions = input.Conditions
	}

	if input.RiskLevel != "" {
		if !isValidRiskLevel(input.RiskLevel) {
			return nil, ErrInvalidInput
		}
		config.RiskLevel = models.RiskLevel(input.RiskLevel)
	}

	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}

	if input.DedupWindow > 0 {
		config.DedupWindow = input.DedupWindow
	}

	config.UpdatedAt = now()

	err = u.configRepo.Update(ctx, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// DeleteAlertConfig deletes an alert configuration.
func (u *Usecase) DeleteAlertConfig(ctx context.Context, id int) error {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if config == nil {
		return ErrAlertConfigNotFound
	}

	return u.configRepo.Delete(ctx, id)
}

// EnableAlertConfig enables an alert configuration.
func (u *Usecase) EnableAlertConfig(ctx context.Context, id int) error {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if config == nil {
		return ErrAlertConfigNotFound
	}

	return u.configRepo.Enable(ctx, id)
}

// DisableAlertConfig disables an alert configuration.
func (u *Usecase) DisableAlertConfig(ctx context.Context, id int) error {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if config == nil {
		return ErrAlertConfigNotFound
	}

	return u.configRepo.Disable(ctx, id)
}

// GetAlertHistory retrieves alert history for a config.
func (u *Usecase) GetAlertHistory(ctx context.Context, configID int, limit int) ([]*models.AlertHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	return u.historyRepo.ListByConfig(ctx, configID, limit)
}

// ListProjectAlertHistory retrieves alert history for a project.
func (u *Usecase) ListProjectAlertHistory(ctx context.Context, projectID int, limit int) ([]*models.AlertHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	return u.historyRepo.ListByProject(ctx, projectID, limit)
}

// GetAlertStats retrieves alert statistics for a project.
func (u *Usecase) GetAlertStats(ctx context.Context, projectID int) (*AlertStats, error) {
	history, err := u.historyRepo.ListByProject(ctx, projectID, 1000)
	if err != nil {
		return nil, err
	}

	stats := &AlertStats{
		ProjectID:    projectID,
		TotalSent:    0,
		TotalFailed:  0,
		TotalPending: 0,
		ByChannel:   make(map[string]int),
	}

	for _, h := range history {
		switch h.Status {
		case models.AlertStatusSent:
			stats.TotalSent++
		case models.AlertStatusFailed:
			stats.TotalFailed++
		case models.AlertStatusPending:
			stats.TotalPending++
		}
		stats.ByChannel[string(h.Channel)]++
	}

	return stats, nil
}

// TestAlertConfig tests an alert configuration by sending a test alert.
func (u *Usecase) TestAlertConfig(ctx context.Context, id int) error {
	config, err := u.configRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if config == nil {
		return ErrAlertConfigNotFound
	}

	// The actual test sending would be done by the handler/dispatcher
	// For now, we just validate the config exists and is valid
	_ = config
	return nil
}

// Helpers

func isValidChannel(channel string) bool {
	for _, c := range SupportedChannels {
		if c == channel {
			return true
		}
	}
	return false
}

func isValidRiskLevel(level string) bool {
	switch models.RiskLevel(level) {
	case models.RiskLevelLow, models.RiskLevelMedium, models.RiskLevelHigh, models.RiskLevelCritical:
		return true
	default:
		return false
	}
}

func now() int64 {
	return time.Now().Unix()
}
