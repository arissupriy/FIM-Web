package audit

import (
	"testing"
	"time"
)

func TestAttributor_AttributeEvent(t *testing.T) {
	a := NewAttributor()

	e := &Event{
		UserID:     1000,
		LoginUID:   1000,
		ProcessID:  1234,
		ParentPID:  1000,
		SessionID:  42,
		TTY:        "pts0",
		HostName:   "server1",
		Addr:       "192.168.1.100",
		Timestamp:  time.Now(),
	}

	attr := a.AttributeEvent(e)

	if attr == nil {
		t.Fatal("Expected non-nil attribution")
	}

	if attr.UserID != 1000 {
		t.Errorf("Expected UserID 1000, got %d", attr.UserID)
	}

	if attr.LoginUID != 1000 {
		t.Errorf("Expected LoginUID 1000, got %d", attr.LoginUID)
	}

	if attr.ProcessID != 1234 {
		t.Errorf("Expected ProcessID 1234, got %d", attr.ProcessID)
	}

	if attr.SessionID != 42 {
		t.Errorf("Expected SessionID 42, got %d", attr.SessionID)
	}

	if attr.TTY != "pts0" {
		t.Errorf("Expected TTY pts0, got %s", attr.TTY)
	}
}

func TestAttributor_AttributeEvent_Nil(t *testing.T) {
	a := NewAttributor()

	attr := a.AttributeEvent(nil)

	if attr != nil {
		t.Error("Expected nil attribution for nil event")
	}
}

func TestAttributor_AggregateAttribution(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{ProcessID: 100, UserID: 0, LoginUID: 0},
		{ProcessID: 100, UserID: 1000, LoginUID: 1000},
		{ProcessID: 200, UserID: 0},
	}

	attr := a.AggregateAttribution(events)

	if attr == nil {
		t.Fatal("Expected non-nil attribution")
	}

	if attr.UserID != 1000 {
		t.Errorf("Expected UserID 1000, got %d", attr.UserID)
	}

	if attr.LoginUID != 1000 {
		t.Errorf("Expected LoginUID 1000, got %d", attr.LoginUID)
	}

	if attr.ProcessID != 100 {
		t.Errorf("Expected ProcessID 100, got %d", attr.ProcessID)
	}
}

func TestAttributor_AggregateAttribution_Empty(t *testing.T) {
	a := NewAttributor()

	attr := a.AggregateAttribution([]*Event{})

	if attr != nil {
		t.Error("Expected nil attribution for empty events")
	}
}

func TestAttributor_AttributeFIMEvent(t *testing.T) {
	a := NewAttributor()

	fimEvent := &FIMEvent{
		Path:      "/etc/passwd",
		ProcessID: 1234,
	}

	auditEvents := []*Event{
		{ProcessID: 1234, UserID: 1000, Path: "/etc/passwd"},
		{ProcessID: 1234, LoginUID: 1000, Path: "/etc/passwd"},
	}

	attr := a.AttributeFIMEvent(fimEvent, auditEvents)

	if attr == nil {
		t.Fatal("Expected non-nil attribution")
	}

	if attr.UserID != 1000 {
		t.Errorf("Expected UserID 1000, got %d", attr.UserID)
	}
}

func TestAttributor_AttributeFIMEvent_NoMatch(t *testing.T) {
	a := NewAttributor()

	fimEvent := &FIMEvent{
		Path:      "/unknown",
		ProcessID: 9999,
	}

	auditEvents := []*Event{
		{ProcessID: 1234, UserID: 1000},
	}

	attr := a.AttributeFIMEvent(fimEvent, auditEvents)

	if attr != nil {
		t.Error("Expected nil attribution when no match")
	}
}

func TestAttributor_GetProcessChain(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{ProcessID: 100, ParentPID: 50, Type: "SYSCALL"},
		{ProcessID: 50, ParentPID: 10, Type: "SYSCALL"},
		{ProcessID: 10, ParentPID: 1, Type: "SYSCALL"},
		{ProcessID: 1, ParentPID: 0, Type: "SYSCALL"},
	}

	chain := a.GetProcessChain(100, events)

	if len(chain) != 4 {
		t.Errorf("Expected chain length 4, got %d", len(chain))
	}

	if chain[0].ProcessID != 100 {
		t.Errorf("Expected first PID 100, got %d", chain[0].ProcessID)
	}

	if chain[3].ProcessID != 1 {
		t.Errorf("Expected last PID 1, got %d", chain[3].ProcessID)
	}
}

func TestAttributor_GetProcessChain_NoParent(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{ProcessID: 100, ParentPID: 50, Type: "SYSCALL"},
	}

	chain := a.GetProcessChain(100, events)

	if len(chain) != 1 {
		t.Errorf("Expected chain length 1, got %d", len(chain))
	}
}

func TestAttributor_WhoWasAtPath(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Path: "/etc/passwd", UserID: 1000, ProcessID: 1234},
		{Path: "/etc/passwd", UserID: 0, ProcessID: 5678},
	}

	attr := a.WhoWasAtPath("/etc/passwd", events)

	if attr == nil {
		t.Fatal("Expected non-nil attribution")
	}

	// Should use the first match (1000)
	if attr.UserID != 1000 {
		t.Errorf("Expected UserID 1000, got %d", attr.UserID)
	}
}

func TestAttributor_WhoWasAtPath_NotFound(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Path: "/etc/passwd", UserID: 1000},
	}

	attr := a.WhoWasAtPath("/etc/shadow", events)

	if attr != nil {
		t.Error("Expected nil attribution for non-matching path")
	}
}

func TestAttributor_WhoModifiedFile(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Type: "PATH", Path: "/etc/passwd", UserID: 1000},
		{Type: "SYSCALL", Path: "/etc/passwd", UserID: 2000},
	}

	attr := a.WhoModifiedFile("/etc/passwd", events)

	if attr == nil {
		t.Fatal("Expected non-nil attribution")
	}

	// PATH event should be used
	if attr.UserID != 1000 {
		t.Errorf("Expected UserID 1000, got %d", attr.UserID)
	}
}

func TestAttributor_ParseLoginEvents(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Type: "LOGIN", SessionID: 42, LoginUID: 1000, TTY: "pts0", Timestamp: time.Unix(1000, 0)},
		{Type: "SYSCALL", SessionID: 42, ProcessID: 1234, Path: "/etc/passwd"},
		{Type: "PATH", SessionID: 42, ProcessID: 1234, Path: "/etc/passwd"},
		{Type: "LOGIN", SessionID: 99, LoginUID: 2000, TTY: "pts1", Timestamp: time.Unix(2000, 0)},
	}

	sessions := a.ParseLoginEvents(events)

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Find session 42
	var session42 *SessionInfo
	for _, s := range sessions {
		if s.SessionID == 42 {
			session42 = s
			break
		}
	}

	if session42 == nil {
		t.Fatal("Session 42 not found")
	}

	if len(session42.ProcessIDs) != 1 {
		t.Errorf("Expected 1 PID, got %d", len(session42.ProcessIDs))
	}

	if session42.ProcessIDs[0] != 1234 {
		t.Errorf("Expected PID 1234, got %d", session42.ProcessIDs[0])
	}
}

func TestAttributor_GetLastLogin(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Type: "LOGIN", LoginUID: 1000, SessionID: 1, Timestamp: time.Unix(1000, 0)},
		{Type: "LOGIN", LoginUID: 1000, SessionID: 2, Timestamp: time.Unix(2000, 0)},
	}

	login := a.GetLastLogin(1000, events)

	if login == nil {
		t.Fatal("Expected non-nil login")
	}

	if login.SessionID != 2 {
		t.Errorf("Expected session 2 (most recent), got %d", login.SessionID)
	}
}

func TestAttributor_GetLastLogin_NotFound(t *testing.T) {
	a := NewAttributor()

	events := []*Event{
		{Type: "LOGIN", LoginUID: 2000},
	}

	login := a.GetLastLogin(1000, events)

	if login != nil {
		t.Error("Expected nil for non-existent user")
	}
}

func TestFormatAttribution(t *testing.T) {
	attr := &Attribution{
		UserID:     1000,
		Username:   "www-data",
		SessionID:  42,
		TTY:        "pts0",
		RemoteAddr: "192.168.1.1",
	}

	result := FormatAttribution(attr)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Should contain username
	if result == "unknown" {
		t.Error("Should not be 'unknown'")
	}
}

func TestFormatAttribution_Nil(t *testing.T) {
	result := FormatAttribution(nil)

	if result != "unknown" {
		t.Errorf("Expected 'unknown', got %s", result)
	}
}

func TestFormatActor(t *testing.T) {
	actor := &Actor{
		UserID:      1000,
		Username:    "www-data",
		ProcessName: "nginx",
		TTY:         "pts0",
		HostName:    "server1",
	}

	result := FormatActor(actor)

	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatActor_Nil(t *testing.T) {
	result := FormatActor(nil)

	if result != "unknown" {
		t.Errorf("Expected 'unknown', got %s", result)
	}
}

func TestFormatActor_UIDOnly(t *testing.T) {
	actor := &Actor{
		UserID: 9999,
	}

	result := FormatActor(actor)

	// Should contain uid-9999
	if result == "unknown" {
		t.Error("Should contain uid fallback")
	}
}
