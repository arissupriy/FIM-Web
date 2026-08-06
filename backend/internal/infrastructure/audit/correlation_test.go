package audit

import (
	"testing"
	"time"
)

func TestCorrelator_CorrelateByPID(t *testing.T) {
	c := NewCorrelator()

	fimEvent := &FIMEvent{
		Path:       "/etc/passwd",
		ProcessID:  1234,
		Timestamp:  time.Now(),
	}

	auditEvents := []*Event{
		{
			Type:       "SYSCALL",
			ProcessID:  1234,
			UserID:     1000,
			Timestamp:  time.Now(),
		},
		{
			Type:       "PATH",
			ProcessID:  1234,
			Path:       "/etc/passwd",
			Timestamp:  time.Now(),
		},
	}

	result := c.CorrelateEvent(fimEvent, auditEvents)

	if !result.Correlated {
		t.Error("Expected correlation by PID to succeed")
	}
}

func TestCorrelator_CorrelateByPath(t *testing.T) {
	c := NewCorrelator()

	fimEvent := &FIMEvent{
		Path:       "/etc/passwd",
		Timestamp:  time.Now(),
	}

	auditEvents := []*Event{
		{
			Type:       "PATH",
			Path:       "/etc/passwd",
			Timestamp:  time.Now(),
		},
	}

	result := c.CorrelateEvent(fimEvent, auditEvents)

	if !result.Correlated {
		t.Error("Expected correlation by path to succeed")
	}
}

func TestCorrelator_NoCorrelation(t *testing.T) {
	c := NewCorrelator()

	fimEvent := &FIMEvent{
		Path:       "/unknown/file",
		Timestamp:  time.Unix(1000, 0),
	}

	auditEvents := []*Event{
		{
			Type:       "SYSCALL",
			Path:       "/etc/passwd",
			ProcessID:  9999,
			Timestamp:  time.Unix(2000, 0), // Far from fim event
		},
	}

	result := c.CorrelateEvent(fimEvent, auditEvents)

	if result.Correlated {
		t.Error("Expected no correlation for unrelated events")
	}
}

func TestCorrelator_ExtractActor(t *testing.T) {
	c := NewCorrelator()

	events := []*Event{
		{
			UserID:       1000,
			LoginUID:     1000,
			ProcessID:    1234,
			ProcessName:  "bash",
			Comm:         "test-script",
			SessionID:    42,
			TTY:         "pts0",
			HostName:    "server1",
		},
	}

	actor := c.extractActor(events)

	if actor == nil {
		t.Fatal("Expected non-nil actor")
	}

	if actor.UserID != 1000 {
		t.Errorf("Expected user ID 1000, got %d", actor.UserID)
	}

	if actor.ProcessID != 1234 {
		t.Errorf("Expected process ID 1234, got %d", actor.ProcessID)
	}

	if actor.SessionID != 42 {
		t.Errorf("Expected session ID 42, got %d", actor.SessionID)
	}

	if actor.TTY != "pts0" {
		t.Errorf("Expected TTY pts0, got %s", actor.TTY)
	}
}

func TestCorrelator_UsernameCache(t *testing.T) {
	c := NewCorrelator()
	c.SetUsernameCache(map[uint32]string{
		0:    "root",
		1000: "www-data",
	})

	tests := []struct {
		uid      uint32
		expected string
	}{
		{0, "root"},
		{1000, "www-data"},
		{9999, "uid-9999"}, // Not in cache
	}

	for _, tt := range tests {
		got := c.GetUsername(tt.uid)
		if got != tt.expected {
			t.Errorf("GetUsername(%d) = %s, want %s", tt.uid, got, tt.expected)
		}
	}
}

func TestCorrelateBatch(t *testing.T) {
	c := NewCorrelator()

	fimEvents := []*FIMEvent{
		{Path: "/etc/passwd", ProcessID: 1234, Timestamp: time.Now()},
		{Path: "/etc/shadow", ProcessID: 5678, Timestamp: time.Now()},
	}

	auditEvents := []*Event{
		{ProcessID: 1234, UserID: 1000},
		{ProcessID: 5678, UserID: 1001},
	}

	results := c.CorrelateBatch(fimEvents, auditEvents)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Correlated {
			t.Errorf("Result %d should be correlated", i)
		}
	}
}

func TestBuildPIDIndex(t *testing.T) {
	events := []*Event{
		{ProcessID: 100, Type: "SYSCALL"},
		{ProcessID: 100, Type: "PATH"},
		{ProcessID: 200, Type: "EXECVE"},
	}

	index := BuildPIDIndex(events)

	if len(index[100]) != 2 {
		t.Errorf("Expected 2 events for PID 100, got %d", len(index[100]))
	}

	if len(index[200]) != 1 {
		t.Errorf("Expected 1 event for PID 200, got %d", len(index[200]))
	}
}

func TestBuildPathIndex(t *testing.T) {
	events := []*Event{
		{Path: "/etc/passwd", Type: "PATH"},
		{Path: "/etc/passwd", Type: "CWD"},
		{Path: "/etc/shadow", Type: "PATH"},
	}

	index := BuildPathIndex(events)

	if len(index["/etc/passwd"]) != 2 {
		t.Errorf("Expected 2 events for /etc/passwd, got %d", len(index["/etc/passwd"]))
	}
}

func TestBuildUserIndex(t *testing.T) {
	events := []*Event{
		{UserID: 1000, Type: "SYSCALL"},
		{LoginUID: 1000, Type: "LOGIN"},
		{UserID: 2000, Type: "EXECVE"},
	}

	index := BuildUserIndex(events)

	// Should have entries for both uid and auid
	if len(index[1000]) != 2 {
		t.Errorf("Expected 2 events for uid/auid 1000, got %d", len(index[1000]))
	}
}

func TestBuildSessionIndex(t *testing.T) {
	events := []*Event{
		{SessionID: 42, Type: "SYSCALL"},
		{SessionID: 42, Type: "PATH"},
		{SessionID: 99, Type: "EXECVE"},
	}

	index := BuildSessionIndex(events)

	if len(index[42]) != 2 {
		t.Errorf("Expected 2 events for session 42, got %d", len(index[42]))
	}
}

func TestIndexedCorrelator(t *testing.T) {
	auditEvents := []*Event{
		{ProcessID: 1234, UserID: 1000, Path: "/etc/passwd"},
		{ProcessID: 5678, UserID: 1001, Path: "/etc/shadow"},
	}

	ic := NewIndexedCorrelator(auditEvents)

	// Test PID lookup
	fimEvent := &FIMEvent{ProcessID: 1234}
	result := ic.CorrelateEventWithIndex(fimEvent)

	if !result.Correlated {
		t.Error("Expected correlation by PID")
	}

	if result.MatchedBy != MatchByPID {
		t.Errorf("Expected MatchByPID, got %s", result.MatchedBy)
	}

	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Confidence)
	}
}

func TestIndexedCorrelator_PathLookup(t *testing.T) {
	auditEvents := []*Event{
		{Path: "/etc/passwd", Type: "PATH"},
	}

	ic := NewIndexedCorrelator(auditEvents)

	fimEvent := &FIMEvent{Path: "/etc/passwd"}
	result := ic.CorrelateEventWithIndex(fimEvent)

	if !result.Correlated {
		t.Error("Expected correlation by path")
	}

	if result.MatchedBy != MatchByPath {
		t.Errorf("Expected MatchByPath, got %s", result.MatchedBy)
	}
}

func TestEnrichFIMEvent(t *testing.T) {
	fimEvent := &FIMEvent{
		Path:       "/etc/passwd",
		ProcessID:  1234,
		Timestamp:  time.Now(),
	}

	auditEvents := []*Event{
		{
			ProcessID:   1234,
			UserID:     1000,
			ProcessName: "vim",
			Comm:       "test-edit",
		},
	}

	enriched := EnrichFIMEvent(fimEvent, auditEvents)

	if enriched.ActorID != "1000" {
		t.Errorf("Expected ActorID '1000', got %s", enriched.ActorID)
	}

	// Comm is used over ProcessName
	if enriched.ProcessName != "test-edit" {
		t.Errorf("Expected ProcessName 'test-edit', got %s", enriched.ProcessName)
	}

	if enriched.Source != "auditd" {
		t.Errorf("Expected Source 'auditd', got %s", enriched.Source)
	}
}

func TestGetActorSummary(t *testing.T) {
	events := []*Event{
		{
			Type:       "SYSCALL",
			UserID:     1000,
			ProcessID:  1234,
			SessionID:  42,
			Timestamp:  time.Unix(1000, 0),
		},
		{
			Type:      "PATH",
			UserID:    1000,
			ProcessID: 1234,
			Timestamp: time.Unix(1005, 0),
		},
	}

	actor := &Actor{
		UserID:   1000,
		Username: "www-data",
		ProcessID: 1234,
	}

	summary := GetActorSummary(events, actor)

	if summary["user_id"].(uint32) != 1000 {
		t.Error("Expected user_id 1000")
	}

	if summary["total_events"].(int) != 2 {
		t.Error("Expected 2 total events")
	}

	duration := summary["duration"].(time.Duration)
	if duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", duration)
	}
}

func TestCorrelator_TimeWindow(t *testing.T) {
	c := NewCorrelator()
	c.TimeWindow = 2 * time.Second

	fimEvent := &FIMEvent{
		Path:       "/etc/passwd",
		Timestamp:  time.Unix(1000, 0),
	}

	auditEvents := []*Event{
		{
			Type:      "SYSCALL",
			Path:      "/etc/passwd",
			Timestamp: time.Unix(1001, 0), // 1 second later
		},
	}

	result := c.CorrelateEvent(fimEvent, auditEvents)

	if !result.Correlated {
		t.Error("Expected correlation within time window")
	}
}

func TestCorrelator_TimeWindow_Outside(t *testing.T) {
	c := NewCorrelator()
	c.TimeWindow = 2 * time.Second

	fimEvent := &FIMEvent{
		Path:       "/etc/passwd",
		Timestamp:  time.Unix(1000, 0),
	}

	auditEvents := []*Event{
		{
			Type:      "SYSCALL",
			Path:      "/etc/passwd",
			Timestamp: time.Unix(1005, 0), // 5 seconds later - outside window
		},
	}

	result := c.CorrelateEvent(fimEvent, auditEvents)

	// Time correlation shouldn't match since it's outside window
	// But path correlation should still work
	if result.MatchedBy != MatchByPath {
		t.Error("Expected path-based correlation")
	}
}
