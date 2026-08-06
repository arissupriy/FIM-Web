// Package response contains response DTOs for API handlers.
package response

import (
	"time"

	"ojs-monitor/backend/internal/domain/models"
)

// AlertConfigResponse represents an alert configuration response.
type AlertConfigResponse struct {
	ID          int64     `json:"id"`
	ProjectID   int       `json:"project_id"`
	Name        string    `json:"name"`
	Channel     string    `json:"channel"`
	Config      string    `json:"config"`
	Conditions  string    `json:"conditions"`
	RiskLevel   string    `json:"risk_level"`
	Enabled     bool      `json:"enabled"`
	DedupWindow int       `json:"dedup_window"`
	CreatedAt   int64     `json:"created_at"`
	UpdatedAt   int64     `json:"updated_at"`
}

// AlertHistoryResponse represents an alert history entry response.
type AlertHistoryResponse struct {
	ID            int64     `json:"id"`
	AlertConfigID int64     `json:"alert_config_id"`
	FIMEventID   int64     `json:"fim_event_id"`
	ProjectID    int       `json:"project_id"`
	Channel      string    `json:"channel"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	SentAt       *int64    `json:"sent_at,omitempty"`
	CreatedAt    int64     `json:"created_at"`
}

// AlertStatsResponse represents alert statistics for a project.
type AlertStatsResponse struct {
	ProjectID    int `json:"project_id"`
	TotalSent   int `json:"total_sent"`
	TotalFailed int `json:"total_failed"`
	TotalPending int `json:"total_pending"`
	ByChannel   map[string]int `json:"by_channel"`
}

// ToAlertConfigResponse converts a model to response DTO.
func ToAlertConfigResponse(m *models.AlertConfig) *AlertConfigResponse {
	return &AlertConfigResponse{
		ID:          int64(m.ID),
		ProjectID:   m.ProjectID,
		Name:        m.Name,
		Channel:     string(m.Channel),
		Config:      m.Config,
		Conditions:  m.Conditions,
		RiskLevel:   string(m.RiskLevel),
		Enabled:     m.Enabled,
		DedupWindow: m.DedupWindow,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ToAlertConfigResponseList converts a list of models to response DTOs.
func ToAlertConfigResponseList(m []*models.AlertConfig) []*AlertConfigResponse {
	result := make([]*AlertConfigResponse, len(m))
	for i, v := range m {
		result[i] = ToAlertConfigResponse(v)
	}
	return result
}

// ToAlertHistoryResponse converts a model to response DTO.
func ToAlertHistoryResponse(m *models.AlertHistory) *AlertHistoryResponse {
	resp := &AlertHistoryResponse{
		ID:            int64(m.ID),
		AlertConfigID: int64(m.AlertConfigID),
		FIMEventID:   int64(m.FIMEventID),
		ProjectID:    m.ProjectID,
		Channel:      string(m.Channel),
		Status:       string(m.Status),
		ErrorMessage: m.ErrorMessage,
		CreatedAt:   m.CreatedAt,
	}
	if m.SentAt > 0 {
		resp.SentAt = &m.SentAt
	}
	return resp
}

// ToAlertHistoryResponseList converts a list of models to response DTOs.
func ToAlertHistoryResponseList(m []*models.AlertHistory) []*AlertHistoryResponse {
	result := make([]*AlertHistoryResponse, len(m))
	for i, v := range m {
		result[i] = ToAlertHistoryResponse(v)
	}
	return result
}

// NewAlertStatsResponse creates a new stats response.
func NewAlertStatsResponse(projectID int, total, sent, failed, pending int, byChannel map[string]int) *AlertStatsResponse {
	return &AlertStatsResponse{
		ProjectID:    projectID,
		TotalSent:    sent,
		TotalFailed:  failed,
		TotalPending: pending,
		ByChannel:    byChannel,
	}
}

// FormatTimestamp formats a Unix timestamp to RFC3339.
func FormatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format(time.RFC3339)
}
