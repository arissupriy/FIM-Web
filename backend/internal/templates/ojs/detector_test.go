// Package ojs provides tests for OJS template implementation.
package ojs

import (
	"testing"
)

func TestDetector_Name(t *testing.T) {
	d := New()
	if got := d.Name(); got != "ojs" {
		t.Errorf("Name() = %q, want %q", got, "ojs")
	}
}

func TestDetector_Version(t *testing.T) {
	d := New()
	if got := d.Version(); got != "3.x" {
		t.Errorf("Version() = %q, want %q", got, "3.x")
	}
}

func TestDetector_Priority(t *testing.T) {
	d := New()
	if got := d.Priority(); got != 100 {
		t.Errorf("Priority() = %d, want %d", got, 100)
	}
}

func TestDetector_RequiredDBConfig(t *testing.T) {
	d := New()
	reqs := d.RequiredDBConfig()

	expected := []string{"db_host", "db_user", "db_pass", "db_name"}
	if len(reqs) != len(expected) {
		t.Errorf("len(RequiredDBConfig()) = %d, want %d", len(reqs), len(expected))
	}

	for i, e := range expected {
		if reqs[i] != e {
			t.Errorf("RequiredDBConfig()[%d] = %q, want %q", i, reqs[i], e)
		}
	}
}

func TestDetector_DefaultConfig(t *testing.T) {
	d := New()
	cfg := d.DefaultConfig()

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

func TestOrphanResult(t *testing.T) {
	result := OrphanResult{
		Path:         "/var/www/ojs/files/article.pdf",
		OriginalName: "article.pdf",
		IsUpload:     true,
	}

	if result.OriginalName != "article.pdf" {
		t.Errorf("OriginalName = %q, want %q", result.OriginalName, "article.pdf")
	}

	if !result.IsUpload {
		t.Error("IsUpload should be true")
	}
}
