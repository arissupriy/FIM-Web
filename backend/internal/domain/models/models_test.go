package models

import (
	"encoding/json"
	"testing"
)

func TestProjectIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		project  Project
		expected bool
	}{
		{
			name: "fully configured",
			project: Project{
				DBHost:   "localhost",
				DBUser:   "user",
				DBName:   "testdb",
				AppPaths:  []string{"/var/www/ojs"},
			},
			expected: true,
		},
		{
			name: "missing db_host",
			project: Project{
				DBHost:   "",
				DBUser:   "user",
				DBName:   "testdb",
				AppPaths:  []string{"/var/www/ojs"},
			},
			expected: false,
		},
		{
			name: "missing db_user",
			project: Project{
				DBHost:   "localhost",
				DBUser:   "",
				DBName:   "testdb",
				AppPaths:  []string{"/var/www/ojs"},
			},
			expected: false,
		},
		{
			name: "missing db_name",
			project: Project{
				DBHost:   "localhost",
				DBUser:   "user",
				DBName:   "",
				AppPaths:  []string{"/var/www/ojs"},
			},
			expected: false,
		},
		{
			name: "missing app_paths",
			project: Project{
				DBHost:   "localhost",
				DBUser:   "user",
				DBName:   "testdb",
				AppPaths:  []string{},
			},
			expected: false,
		},
		{
			name: "empty app_path string",
			project: Project{
				DBHost:   "localhost",
				DBUser:   "user",
				DBName:   "testdb",
				AppPaths:  []string{""},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.project.IsConfigured()
			if result != tt.expected {
				t.Errorf("IsConfigured() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestProjectJSONMarshaling(t *testing.T) {
	project := Project{
		ID:              1,
		Name:            "Test Project",
		Description:     "Test Description",
		Template:        "OJS 3.x",
		AppPaths:        []string{"/var/www/ojs", "/var/www/ojs2"},
		FilesPaths:      []string{"/var/www/ojs/files"},
		BlacklistExts:   []string{"php", "phtml"},
		WhitelistPaths:  []string{"/var/www/ojs/cache"},
		DBHost:          "localhost",
		DBUser:          "user",
		DBPass:          "pass",
		DBName:          "testdb",
		Status:          "active",
	}

	// Marshal to JSON
	data, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("Failed to marshal project: %v", err)
	}

	// Unmarshal back
	var decoded Project
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal project: %v", err)
	}

	// Verify fields
	if decoded.ID != project.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, project.ID)
	}
	if decoded.Name != project.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, project.Name)
	}
	if len(decoded.AppPaths) != len(project.AppPaths) {
		t.Errorf("AppPaths length mismatch: got %d, want %d", len(decoded.AppPaths), len(project.AppPaths))
	}
}

func TestJobStatus(t *testing.T) {
	tests := []struct {
		name     string
		job      Job
		isQueued bool
		isRunning bool
		isFinished bool
	}{
		{
			name:      "queued job",
			job:       Job{Status: "queued"},
			isQueued:  true,
			isRunning:  false,
			isFinished: false,
		},
		{
			name:      "running job",
			job:       Job{Status: "running"},
			isQueued:  false,
			isRunning:  true,
			isFinished: false,
		},
		{
			name:      "done job",
			job:       Job{Status: "done"},
			isQueued:  false,
			isRunning:  false,
			isFinished: true,
		},
		{
			name:      "failed job",
			job:       Job{Status: "failed"},
			isQueued:  false,
			isRunning:  false,
			isFinished: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.job.IsQueued() != tt.isQueued {
				t.Errorf("IsQueued() = %v, want %v", tt.job.IsQueued(), tt.isQueued)
			}
			if tt.job.IsRunning() != tt.isRunning {
				t.Errorf("IsRunning() = %v, want %v", tt.job.IsRunning(), tt.isRunning)
			}
			if tt.job.IsFinished() != tt.isFinished {
				t.Errorf("IsFinished() = %v, want %v", tt.job.IsFinished(), tt.isFinished)
			}
		})
	}
}

func TestProjectFileStatus(t *testing.T) {
	tests := []struct {
		name    string
		file   ProjectFile
		isOrphan   bool
		isModified bool
	}{
		{
			name:    "orphan file",
			file:   ProjectFile{Status: "ORPHAN"},
			isOrphan:   true,
			isModified: false,
		},
		{
			name:    "modified file",
			file:   ProjectFile{Status: "MODIFIED"},
			isOrphan:   false,
			isModified: true,
		},
		{
			name:    "added file",
			file:   ProjectFile{Status: "ADDED"},
			isOrphan:   false,
			isModified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.file.IsOrphan() != tt.isOrphan {
				t.Errorf("IsOrphan() = %v, want %v", tt.file.IsOrphan(), tt.isOrphan)
			}
			if tt.file.IsModified() != tt.isModified {
				t.Errorf("IsModified() = %v, want %v", tt.file.IsModified(), tt.isModified)
			}
		})
	}
}

func TestFIMEventRiskLevel(t *testing.T) {
	tests := []struct {
		name     string
		event    FIMEvent
		isHighRisk     bool
		isUnknownSource bool
	}{
		{
			name:    "high risk event",
			event:   FIMEvent{RiskLevel: "HIGH"},
			isHighRisk:     true,
			isUnknownSource: false,
		},
		{
			name:    "critical risk event",
			event:   FIMEvent{RiskLevel: "CRITICAL"},
			isHighRisk:     true,
			isUnknownSource: false,
		},
		{
			name:    "low risk event",
			event:   FIMEvent{RiskLevel: "LOW"},
			isHighRisk:     false,
			isUnknownSource: false,
		},
		{
			name:    "unknown source",
			event:   FIMEvent{Classification: "UNKNOWN_SOURCE"},
			isHighRisk:     false,
			isUnknownSource: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.IsHighRisk() != tt.isHighRisk {
				t.Errorf("IsHighRisk() = %v, want %v", tt.event.IsHighRisk(), tt.isHighRisk)
			}
			if tt.event.IsUnknownSource() != tt.isUnknownSource {
				t.Errorf("IsUnknownSource() = %v, want %v", tt.event.IsUnknownSource(), tt.isUnknownSource)
			}
		})
	}
}

func TestAPIResponse(t *testing.T) {
	// Test success response
	resp := NewSuccessResponse(map[string]string{"key": "value"})
	if !resp.Success {
		t.Error("Expected Success to be true")
	}
	if resp.Error != "" {
		t.Errorf("Expected Error to be empty, got %s", resp.Error)
	}

	// Test error response
	errResp := NewErrorResponse("something went wrong")
	if errResp.Success {
		t.Error("Expected Success to be false")
	}
	if errResp.Error != "something went wrong" {
		t.Errorf("Expected Error 'something went wrong', got %s", errResp.Error)
	}
}

func TestScanResult(t *testing.T) {
	// Test success rate calculation
	tests := []struct {
		name     string
		result   ScanResult
		expected float64
	}{
		{
			name:     "100% success",
			result:   ScanResult{TotalFiles: 100, ProcessedFiles: 100},
			expected: 100.0,
		},
		{
			name:     "50% success",
			result:   ScanResult{TotalFiles: 100, ProcessedFiles: 50},
			expected: 50.0,
		},
		{
			name:     "zero files returns 100%",
			result:   ScanResult{TotalFiles: 0, ProcessedFiles: 0},
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate := tt.result.SuccessRate()
			if rate != tt.expected {
				t.Errorf("SuccessRate() = %v, want %v", rate, tt.expected)
			}
		})
	}

	// Test HasErrors
	resultWithErrors := ScanResult{ErrorFiles: 5}
	if !resultWithErrors.HasErrors() {
		t.Error("Expected HasErrors() to return true")
	}

	resultNoErrors := ScanResult{}
	if resultNoErrors.HasErrors() {
		t.Error("Expected HasErrors() to return false")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal path",
			input:    "/var/www/ojs  ",
			expected: "/var/www/ojs",
		},
		{
			name:     "already clean",
			input:    "/var/www/ojs",
			expected: "/var/www/ojs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSafePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "safe path with leading slash",
			path:     "/var/www/ojs",
			expected: true,
		},
		{
			name:     "safe path without leading slash",
			path:     "var/www/ojs",
			expected: true,
		},
		{
			name:     "path traversal attempt",
			path:     "/var/www/../../../etc/passwd",
			expected: false,
		},
		{
			name:     "simple traversal",
			path:     "../etc/passwd",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafePath(tt.path)
			if result != tt.expected {
				t.Errorf("SafePath(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFIMEventFilters(t *testing.T) {
	// Note: FIMEventFilters is in repository package
	// These tests verify the filter validation logic exists
	// Full integration tests would use repository.FIMEventFilters

	// Test that filters can be created and have expected defaults
	// when used with the repository package
	t.Run("filters interface exists", func(t *testing.T) {
		// This is a placeholder test - actual tests use repository package
		// See internal/domain/repository/fim_test.go
		t.Skip("FIMEventFilters is in repository package")
	})
}
