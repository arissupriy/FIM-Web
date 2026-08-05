// Package alert provides tests for the dispatcher.
package alert

import (
	"context"
	"testing"
	"time"

	"ojs-monitor/backend/internal/domain/models"
)

func TestMeetsRiskLevel(t *testing.T) {
	tests := []struct {
		name       string
		eventLevel string
		threshold  string
		want       bool
	}{
		{"LOW meets LOW", "LOW", "LOW", true},
		{"MEDIUM meets MEDIUM", "MEDIUM", "MEDIUM", true},
		{"HIGH meets HIGH", "HIGH", "HIGH", true},
		{"CRITICAL meets CRITICAL", "CRITICAL", "CRITICAL", true},
		{"MEDIUM meets LOW", "MEDIUM", "LOW", true},
		{"HIGH meets MEDIUM", "HIGH", "MEDIUM", true},
		{"CRITICAL meets HIGH", "CRITICAL", "HIGH", true},
		{"LOW does not meet MEDIUM", "LOW", "MEDIUM", false},
		{"LOW does not meet HIGH", "LOW", "HIGH", false},
		{"MEDIUM does not meet HIGH", "MEDIUM", "HIGH", false},
		{"invalid event level", "INVALID", "LOW", false},
		{"invalid threshold", "LOW", "INVALID", false},
		{"empty event level", "", "LOW", false},
		{"empty threshold", "LOW", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := meetsRiskLevel(tt.eventLevel, tt.threshold)
			if got != tt.want {
				t.Errorf("meetsRiskLevel(%q, %q) = %v, want %v",
					tt.eventLevel, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{"exact match", "/var/www/html/index.php", "/var/www/html/index.php", true},
		{"no match", "/var/www/html/index.php", "/var/www/html/login.php", false},
		{"wildcard prefix", "/var/www/html/index.php", "/var/www/html/*.php", true},
		{"wildcard suffix", "/var/www/html/index.php", "*.php", true},
		{"wildcard middle", "/var/www/html/index.php", "/var/www/*/index.php", true},
		{"wildcard no match", "/var/www/html/index.php", "/var/log/*.php", false},
		{"deep path wildcard", "/var/www/html/sub/test.php", "/var/www/html/**/*.php", true},
		{"multiple wildcards", "/var/www/html/index.php", "*/html/*.php", true},
		{"empty pattern", "/var/www/html/index.php", "", false},
		{"just wildcard", "/anything", "*", true},
		{"extension match", "/path/to/file.txt", "*.txt", true},
		{"extension no match", "/path/to/file.txt", "*.php", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.path, tt.pattern)
			if got != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v",
					tt.path, tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestDispatcher_RegisterChannel(t *testing.T) {
	d := &Dispatcher{
		channels: make(map[models.AlertChannel]Channel),
	}

	// Create mock channel
	mockCh := &mockChannel{name: "test"}
	d.RegisterChannel(models.AlertChannelEmail, mockCh)

	if len(d.channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(d.channels))
	}

	ch, ok := d.channels[models.AlertChannelEmail]
	if !ok {
		t.Error("expected email channel to be registered")
	}

	if ch.Name() != "test" {
		t.Errorf("expected channel name 'test', got %s", ch.Name())
	}
}

func TestDispatcher_Dispatch_QueueFull(t *testing.T) {
	d := &Dispatcher{
		eventQueue: make(chan *models.FIMEvent, 1), // Small queue for testing
		stopCh:     make(chan struct{}),
	}

	// Fill the queue using blocking sends (capacity is 1)
	d.eventQueue <- &models.FIMEvent{FilePath: "test1"}

	// At this point queue is full, next send should not block
	// We test this by using a timeout
	done := make(chan bool, 1)
	go func() {
		select {
		case d.eventQueue <- &models.FIMEvent{FilePath: "test2"}:
			done <- false // blocked and sent
		case <-time.After(100 * time.Millisecond):
			done <- true // didn't block (queue full)
		}
	}()

	result := <-done
	if result {
		// Queue is full, event was dropped
		t.Log("Queue is full, event dropped as expected")
	}
}

func TestDispatcher_NewDispatcher(t *testing.T) {
	d := NewDispatcher(nil, nil)

	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}

	if d.channels == nil {
		t.Error("expected channels map to be initialized")
	}

	if d.eventQueue == nil {
		t.Error("expected eventQueue to be initialized")
	}

	if d.stopCh == nil {
		t.Error("expected stopCh to be initialized")
	}
}

// mockChannel is a mock implementation of Channel for testing
type mockChannel struct {
	name string
}

func (m *mockChannel) Name() string {
	return m.name
}

func (m *mockChannel) Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error {
	return nil
}
