// Package request contains request DTOs for API handlers.
package request

// CreateAlertConfig represents a request to create an alert configuration.
type CreateAlertConfig struct {
	ProjectID   int      `json:"project_id" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	Channel     string   `json:"channel" validate:"required,oneof=email slack webhook"`
	Config      string   `json:"config" validate:"required"`
	Conditions  string   `json:"conditions,omitempty"`
	RiskLevel   string   `json:"risk_level" validate:"required,oneof=LOW MEDIUM HIGH CRITICAL"`
	Enabled     bool     `json:"enabled"`
	DedupWindow int      `json:"dedup_window,omitempty"`
}

// UpdateAlertConfig represents a request to update an alert configuration.
type UpdateAlertConfig struct {
	ProjectID   *int    `json:"project_id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Channel     *string `json:"channel,omitempty" validate:"omitempty,oneof=email slack webhook"`
	Config      *string `json:"config,omitempty"`
	Conditions  *string `json:"conditions,omitempty"`
	RiskLevel   *string `json:"risk_level,omitempty" validate:"omitempty,oneof=LOW MEDIUM HIGH CRITICAL"`
	Enabled     *bool   `json:"enabled,omitempty"`
	DedupWindow *int    `json:"dedup_window,omitempty"`
}

// AlertCondition represents alert filtering conditions.
type AlertCondition struct {
	EventTypes       []string `json:"event_types,omitempty"`
	FilePatterns     []string `json:"file_patterns,omitempty"`
	Classifications  []string `json:"classifications,omitempty"`
	RiskLevels       []string `json:"risk_levels,omitempty"`
}
