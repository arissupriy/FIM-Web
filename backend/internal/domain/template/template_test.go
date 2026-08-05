// Package template provides tests for template interfaces.
package template

import (
	"testing"
)

func TestTemplateConfig_Defaults(t *testing.T) {
	cfg := &TemplateConfig{
		Template: "ojs",
		DefaultWatchPaths: []string{"public/"},
		DefaultFilesPaths: []string{"files/"},
		DefaultBlacklistExts: []string{"php", "sh"},
		DefaultRescanInterval: 10,
		WatchType: "OJS_WORKFLOW",
	}

	if cfg.Template != "ojs" {
		t.Errorf("Template = %q, want %q", cfg.Template, "ojs")
	}

	if len(cfg.DefaultBlacklistExts) != 2 {
		t.Errorf("len(DefaultBlacklistExts) = %d, want 2", len(cfg.DefaultBlacklistExts))
	}

	if cfg.DefaultRescanInterval != 10 {
		t.Errorf("DefaultRescanInterval = %d, want 10", cfg.DefaultRescanInterval)
	}
}

func TestNewCorrelationResult(t *testing.T) {
	result := NewCorrelationResult("/path/to/file.php", "CREATED")

	if result.Found {
		t.Error("expected Found = false")
	}

	if result.ActorType != "UNKNOWN" {
		t.Errorf("ActorType = %q, want %q", result.ActorType, "UNKNOWN")
	}

	if result.Classification != "UNKNOWN_SOURCE" {
		t.Errorf("Classification = %q, want %q", result.Classification, "UNKNOWN_SOURCE")
	}

	if result.RiskLevel != "LOW" {
		t.Errorf("RiskLevel = %q, want %q", result.RiskLevel, "LOW")
	}
}

func TestCorrelationResult_SetActor(t *testing.T) {
	result := NewCorrelationResult("/path/to/file.pdf", "MODIFIED")
	result.SetActor("CMS_USER", "123", "john.doe", "john@example.com")

	if !result.Found {
		t.Error("expected Found = true")
	}

	if result.ActorType != "CMS_USER" {
		t.Errorf("ActorType = %q, want %q", result.ActorType, "CMS_USER")
	}

	if result.ActorID != "123" {
		t.Errorf("ActorID = %q, want %q", result.ActorID, "123")
	}

	if result.ActorName != "john.doe" {
		t.Errorf("ActorName = %q, want %q", result.ActorName, "john.doe")
	}

	if result.ActorEmail != "john@example.com" {
		t.Errorf("ActorEmail = %q, want %q", result.ActorEmail, "john@example.com")
	}
}

func TestCorrelationResult_SetRiskLevel(t *testing.T) {
	tests := []struct {
		eventType string
		actorType string
		want      string
	}{
		{"CREATED", "CMS_USER", "LOW"},
		{"CREATED", "SYSTEM", "MEDIUM"},
		{"MODIFIED", "CMS_USER", "LOW"},
		{"MODIFIED", "UNKNOWN", "HIGH"},
		{"DELETED", "CMS_USER", "MEDIUM"},
		{"DELETED", "SYSTEM", "MEDIUM"},
		{"UNKNOWN", "UNKNOWN", "LOW"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType+"_"+tt.actorType, func(t *testing.T) {
			result := &CorrelationResult{
				ActorType: tt.actorType,
			}
			result.SetRiskLevel(tt.eventType)

			if result.RiskLevel != tt.want {
				t.Errorf("RiskLevel = %q, want %q", result.RiskLevel, tt.want)
			}
		})
	}
}

func TestNewTemplateMetrics(t *testing.T) {
	m := NewTemplateMetrics("wordpress", "6.x")

	if m.TemplateName != "wordpress" {
		t.Errorf("TemplateName = %q, want %q", m.TemplateName, "wordpress")
	}

	if m.Version != "6.x" {
		t.Errorf("Version = %q, want %q", m.Version, "6.x")
	}

	if m.Generic == nil {
		t.Error("Generic should not be nil")
	}

	if m.Specific == nil {
		t.Error("Specific should not be nil")
	}
}

func TestTemplateMetrics_AddMetric(t *testing.T) {
	m := NewTemplateMetrics("ojs", "3.x")

	m.AddMetric("total_users", 100)
	m.AddMetric("active_journals", 5)

	if val, ok := m.Specific["total_users"]; !ok || val != 100 {
		t.Errorf("Specific[total_users] = %v, want 100", val)
	}

	if val, ok := m.Specific["active_journals"]; !ok || val != 5 {
		t.Errorf("Specific[active_journals] = %v, want 5", val)
	}
}

func TestWarningLevels(t *testing.T) {
	tests := []struct {
		level WarningLevel
		want  string
	}{
		{WarningLow, "LOW"},
		{WarningMedium, "MEDIUM"},
		{WarningHigh, "HIGH"},
		{WarningCritical, "CRITICAL"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("WarningLevel = %q, want %q", tt.level, tt.want)
		}
	}
}

func TestIntegrityWarning(t *testing.T) {
	w := IntegrityWarning{
		Level:   WarningHigh,
		Code:    "TEST_WARNING",
		Message: "Test warning message",
		Details: "Additional details",
	}

	if w.Level != WarningHigh {
		t.Errorf("Level = %q, want %q", w.Level, WarningHigh)
	}

	if w.Code != "TEST_WARNING" {
		t.Errorf("Code = %q, want %q", w.Code, "TEST_WARNING")
	}
}
