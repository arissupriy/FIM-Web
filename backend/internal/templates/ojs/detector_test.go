// Package ojs provides tests for OJS template implementation.
package ojs

import (
	"testing"

	"ojs-monitor/backend/internal/domain/template"
)

func TestService_Name(t *testing.T) {
	s := New()
	if got := s.Name(); got != "ojs" {
		t.Errorf("Name() = %q, want %q", got, "ojs")
	}
}

func TestService_Version(t *testing.T) {
	s := New()
	if got := s.Version(); got != "3.x" {
		t.Errorf("Version() = %q, want %q", got, "3.x")
	}
}

func TestService_Priority(t *testing.T) {
	s := New()
	if got := s.Priority(); got != 100 {
		t.Errorf("Priority() = %d, want %d", got, 100)
	}
}

func TestService_RequiredDBConfig(t *testing.T) {
	s := New()
	reqs := s.RequiredDBConfig()

	expected := []string{"host", "user", "password", "database"}
	if len(reqs) != len(expected) {
		t.Errorf("len(RequiredDBConfig()) = %d, want %d", len(reqs), len(expected))
	}

	for i, e := range expected {
		if reqs[i] != e {
			t.Errorf("RequiredDBConfig()[%d] = %q, want %q", i, reqs[i], e)
		}
	}
}

func TestService_DefaultConfig(t *testing.T) {
	s := New()
	cfg := s.DefaultConfig()

	if cfg.Template != "ojs" {
		t.Errorf("Template = %q, want %q", cfg.Template, "ojs")
	}

	// Check default watch paths
	if len(cfg.DefaultWatchPaths) == 0 {
		t.Error("DefaultWatchPaths should not be empty")
	}

	// Check default blacklist
	expectedBl := []string{"php", "phtml", "sh"}
	for _, ext := range expectedBl {
		found := false
		for _, bl := range cfg.DefaultBlacklistExts {
			if bl == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected blacklist to contain %q", ext)
		}
	}

	// Check default rescan interval
	if cfg.DefaultRescanInterval != 10 {
		t.Errorf("DefaultRescanInterval = %d, want %d", cfg.DefaultRescanInterval, 10)
	}

	// Check watch type
	if cfg.WatchType != "OJS_WORKFLOW" {
		t.Errorf("WatchType = %q, want %q", cfg.WatchType, "OJS_WORKFLOW")
	}
}

func TestDetectVersion(t *testing.T) {
	// Test with empty paths
	version := DetectVersion([]string{})
	if version == "" {
		t.Error("DetectVersion should return a version string, got empty")
	}
}

func TestIsOJSPath(t *testing.T) {
	// Test with non-existent path
	if IsOJSPath("/nonexistent/path") {
		t.Error("IsOJSPath should return false for non-existent path")
	}
}

func TestService_ImplementsTemplate(t *testing.T) {
	s := New()
	var _ template.Template = s
}
