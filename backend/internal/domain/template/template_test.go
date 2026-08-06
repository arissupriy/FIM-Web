// Package template provides tests for template interfaces.
package template

import (
	"testing"
)

func TestTemplateConfig_Defaults(t *testing.T) {
	cfg := &TemplateConfig{
		Template:             "ojs",
		DefaultWatchPaths:    []string{"public/"},
		DefaultFilesPaths:    []string{"files/"},
		DefaultBlacklistExts: []string{"php", "sh"},
		DefaultRescanInterval: 10,
		WatchType:            "OJS_WORKFLOW",
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

	if result.Classification != "UNKNOWN" {
		t.Errorf("Classification = %q, want %q", result.Classification, "UNKNOWN")
	}

	if result.RiskLevel != "LOW" {
		t.Errorf("RiskLevel = %q, want %q", result.RiskLevel, "LOW")
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

	if m.Specific == nil {
		t.Error("Specific should not be nil")
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

func TestDBConnectionConfig(t *testing.T) {
	cfg := DBConnectionConfig{
		Host:     "localhost",
		User:     "root",
		Password: "secret",
		DBName:   "ojs",
		Timeout:  10,
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}

	if cfg.Timeout != 10 {
		t.Errorf("Timeout = %d, want 10", cfg.Timeout)
	}
}

func TestCorrelationResult_Fields(t *testing.T) {
	result := &CorrelationResult{
		Found:          true,
		ActorType:      "CMS_USER",
		ActorID:        "123",
		ActorName:      "john.doe",
		ActorEmail:     "john@example.com",
		SubmissionID:   "456",
		Classification: "OJS_WORKFLOW",
		RiskLevel:     "LOW",
		Reason:        "File found in OJS submission_files",
	}

	if !result.Found {
		t.Error("Found should be true")
	}

	if result.ActorType != "CMS_USER" {
		t.Errorf("ActorType = %q, want %q", result.ActorType, "CMS_USER")
	}

	if result.ActorID != "123" {
		t.Errorf("ActorID = %q, want %q", result.ActorID, "123")
	}

	if result.SubmissionID != "456" {
		t.Errorf("SubmissionID = %q, want %q", result.SubmissionID, "456")
	}
}

func TestTemplateConfig_DisplayName(t *testing.T) {
	cfg := &TemplateConfig{
		Template:    "wordpress",
		DisplayName: "WordPress CMS",
	}

	if cfg.DisplayName != "WordPress CMS" {
		t.Errorf("DisplayName = %q, want %q", cfg.DisplayName, "WordPress CMS")
	}
}
