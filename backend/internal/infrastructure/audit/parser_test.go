package audit

import (
	"strings"
	"testing"
	"time"
)

func TestParseEvent_Syscall(t *testing.T) {
	line := `type=SYSCALL msg=audit(1720000000.123:12345): arch=c000003e syscall=2 success=yes exit=0 a0=55a2b3c4d5e0 a1=0 a2=0 a3=0 items=1 ppid=1234 pid=5678 uid=0 gid=0 euid=0 suid=0 fsuid=0 egid=0 sgid=0 fsgid=0 tty=pts0 ses=1 comm=cat exe="/bin/cat" key="tmp_test"`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "SYSCALL" {
		t.Errorf("Expected type SYSCALL, got %s", event.Type)
	}

	if event.Serial != 12345 {
		t.Errorf("Expected serial 12345, got %d", event.Serial)
	}

	expectedTime := time.Unix(1720000000, 0)
	if !event.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, event.Timestamp)
	}

	if event.Syscall != 2 {
		t.Errorf("Expected syscall 2 (open), got %d", event.Syscall)
	}

	if event.ProcessID != 5678 {
		t.Errorf("Expected pid 5678, got %d", event.ProcessID)
	}

	if event.ParentPID != 1234 {
		t.Errorf("Expected ppid 1234, got %d", event.ParentPID)
	}

	if event.UserID != 0 {
		t.Errorf("Expected uid 0, got %d", event.UserID)
	}

	if event.Comm != "cat" {
		t.Errorf("Expected comm cat, got %s", event.Comm)
	}
}

func TestParseEvent_Execve(t *testing.T) {
	line := `type=EXECVE msg=audit(1720000001.456:12346): argc=3 a0="rm" a1="-rf" a2="/tmp/test"`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "EXECVE" {
		t.Errorf("Expected type EXECVE, got %s", event.Type)
	}

	if event.Serial != 12346 {
		t.Errorf("Expected serial 12346, got %d", event.Serial)
	}
}

func TestParseEvent_Path(t *testing.T) {
	line := `type=PATH msg=audit(1720000002.789:12347): item=0 name="/etc/passwd" inode=123456 dev=08:02 mode=0100644 ouid=0 ogid=0 rdev=00:00 nametype=NORMAL`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "PATH" {
		t.Errorf("Expected type PATH, got %s", event.Type)
	}

	if event.Path != "/etc/passwd" {
		t.Errorf("Expected path /etc/passwd, got %s", event.Path)
	}

	if event.Mode != "0100644" {
		t.Errorf("Expected mode 0100644, got %s", event.Mode)
	}
}

func TestParseEvent_ServiceStart(t *testing.T) {
	line := `type=SERVICE_START msg=audit(1720000003.001:12348): pid=1 uid=0 auid=4294967295 ses=4294967295 msg='unit=httpd.service commd=/usr/sbin/httpd'`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "SERVICE_START" {
		t.Errorf("Expected type SERVICE_START, got %s", event.Type)
	}

	if event.ProcessID != 1 {
		t.Errorf("Expected pid 1, got %d", event.ProcessID)
	}
}

func TestParseEvent_Login(t *testing.T) {
	line := `type=LOGIN msg=audit(1720000004.002:12349): pid=9999 uid=0 old auid=4294967295 new auid=1000 ses=2`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "LOGIN" {
		t.Errorf("Expected type LOGIN, got %s", event.Type)
	}

	if event.LoginUID != 1000 {
		t.Errorf("Expected auid 1000, got %d", event.LoginUID)
	}

	if event.SessionID != 2 {
		t.Errorf("Expected ses 2, got %d", event.SessionID)
	}
}

func TestParseEvent_EmptyLine(t *testing.T) {
	event, err := ParseEvent("")
	if err != nil {
		t.Fatalf("ParseEvent should not error on empty line: %v", err)
	}
	if event != nil {
		t.Error("Expected nil event on empty line")
	}
}

func TestParseEvent_Comment(t *testing.T) {
	event, err := ParseEvent("# This is a comment")
	if err != nil {
		t.Fatalf("ParseEvent should not error on comment: %v", err)
	}
	if event != nil {
		t.Error("Expected nil event on comment")
	}
}

func TestParseEvent_Whitespace(t *testing.T) {
	event, err := ParseEvent("   ")
	if err != nil {
		t.Fatalf("ParseEvent should not error on whitespace: %v", err)
	}
	if event != nil {
		t.Error("Expected nil event on whitespace")
	}
}

func TestParseEvent_CWD(t *testing.T) {
	line := `type=CWD msg=audit(1720000005.003:12350): cwd="/home/user"`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Directory != "/home/user" {
		t.Errorf("Expected cwd /home/user, got %s", event.Directory)
	}
}

func TestParseEvent_Network(t *testing.T) {
	line := `type=SOCKADDR msg=audit(1720000006.004:12351): saddr=02001030000000000000000000000000 laddr=127.0.0.1 lport=8080`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Addr != "127.0.0.1" {
		t.Errorf("Expected addr 127.0.0.1, got %s", event.Addr)
	}

	if event.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", event.Port)
	}
}

func TestParseEvent_Exit(t *testing.T) {
	line := `type=SYSCALL msg=audit(1720000007.005:12352): arch=c000003e syscall=59 success=yes exit=0 exit_group=0`

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Syscall != 59 {
		t.Errorf("Expected syscall 59 (execve), got %d", event.Syscall)
	}

	if event.ExitCode != 0 {
		t.Errorf("Expected exit 0, got %d", event.ExitCode)
	}
}

func TestParseEvent_GID(t *testing.T) {
	line := "type=GROUP msg=audit(1720000008.006:12353): pid=12345 uid=0 old auid=4294967295 new auid=0 ses=1 msg=op=modify-group"

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}
}

func TestGetSyscallName(t *testing.T) {
	tests := []struct {
		syscall int
		want    string
	}{
		{0, "read"},
		{1, "write"},
		{2, "open"},
		{59, "execve"},
		{80, "rename"},
		{81, "mkdir"},
		{82, "rmdir"},
		{85, "unlink"},
		{257, "openat"},
		{9999, "syscall_9999"},
	}

	for _, tt := range tests {
		got := GetSyscallName(tt.syscall)
		if got != tt.want {
			t.Errorf("GetSyscallName(%d) = %s, want %s", tt.syscall, got, tt.want)
		}
	}
}

func TestIsFileOperation(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{"SYSCALL", true},
		{"PATH", true},
		{"EXECVE", true},
		{"LOGIN", false},
		{"USER_LOGIN", false},
		{"SERVICE_START", false},
	}

	for _, tt := range tests {
		got := IsFileOperation(tt.eventType)
		if got != tt.want {
			t.Errorf("IsFileOperation(%s) = %v, want %v", tt.eventType, got, tt.want)
		}
	}
}

func TestFilterByTimeRange(t *testing.T) {
	events := []*Event{
		{Timestamp: time.Unix(1000, 0), Serial: 1},
		{Timestamp: time.Unix(2000, 0), Serial: 2},
		{Timestamp: time.Unix(3000, 0), Serial: 3},
		{Timestamp: time.Unix(4000, 0), Serial: 4},
	}

	start := time.Unix(1500, 0)
	end := time.Unix(3500, 0)

	filtered := FilterByTimeRange(events, start, end)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events, got %d", len(filtered))
	}

	if filtered[0].Serial != 2 || filtered[1].Serial != 3 {
		t.Errorf("Unexpected filtered serials")
	}
}

func TestFilterByPID(t *testing.T) {
	events := []*Event{
		{ProcessID: 100, Serial: 1},
		{ProcessID: 200, Serial: 2},
		{ProcessID: 100, Serial: 3},
		{ProcessID: 300, Serial: 4},
	}

	filtered := FilterByPID(events, 100)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events, got %d", len(filtered))
	}
}

func TestFilterByPath(t *testing.T) {
	events := []*Event{
		{Path: "/etc/passwd", Serial: 1},
		{Path: "/var/log/messages", Serial: 2},
		{Path: "/etc/shadow", Serial: 3},
	}

	filtered := FilterByPath(events, "/etc/")

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events, got %d", len(filtered))
	}
}

func TestFilterByType(t *testing.T) {
	events := []*Event{
		{Type: "SYSCALL", Serial: 1},
		{Type: "PATH", Serial: 2},
		{Type: "SYSCALL", Serial: 3},
	}

	filtered := FilterByType(events, "SYSCALL")

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events, got %d", len(filtered))
	}
}

func TestFilterByUser(t *testing.T) {
	events := []*Event{
		{UserID: 1000, Serial: 1},
		{LoginUID: 1000, Serial: 2},
		{UserID: 2000, Serial: 3},
	}

	filtered := FilterByUser(events, 1000)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events, got %d", len(filtered))
	}
}

func TestReader(t *testing.T) {
	input := "type=SYSCALL msg=audit(1720000000.001:1): syscall=2 success=yes exit=0\n" +
		"type=PATH msg=audit(1720000000.002:2): name=\"/etc/passwd\"\n" +
		"# comment\n" +
		"type=EXECVE msg=audit(1720000000.003:3): argc=2"

	reader := NewReader(strings.NewReader(input))

	var events []*Event
	for {
		event, err := reader.Read()
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("Read error: %v", err)
		}
		if event == nil {
			break
		}
		events = append(events, event)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	if events[0].Type != "SYSCALL" {
		t.Errorf("Expected SYSCALL, got %s", events[0].Type)
	}

	if events[1].Type != "PATH" {
		t.Errorf("Expected PATH, got %s", events[1].Type)
	}

	if events[2].Type != "EXECVE" {
		t.Errorf("Expected EXECVE, got %s", events[2].Type)
	}
}

func TestReader_Empty(t *testing.T) {
	reader := NewReader(strings.NewReader(""))

	event, err := reader.Read()
	if err == nil {
		t.Error("Expected EOF on empty input")
	}
	if event != nil {
		t.Error("Expected nil event on empty input")
	}
}

func TestReader_CommentsOnly(t *testing.T) {
	input := "# comment 1\n# comment 2\n   "

	reader := NewReader(strings.NewReader(input))

	event, err := reader.Read()
	if err == nil {
		t.Error("Expected EOF after comments")
	}
	if event != nil {
		t.Error("Expected nil event after comments")
	}
}

func TestParseJSON(t *testing.T) {
	// JSON format from auditd 2.8+
	line := "{\"type\":\"SYSCALL\",\"ts\":\"1720000000.000\",\"serial\":\"12345\",\"pid\":\"5678\",\"ppid\":\"1234\",\"uid\":\"0\",\"auid\":\"4294967295\",\"syscall\":\"2\",\"exit\":\"0\",\"success\":\"yes\"}"

	event, err := ParseEvent(line)
	if err != nil {
		t.Fatalf("ParseEvent JSON failed: %v", err)
	}

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != "SYSCALL" {
		t.Errorf("Expected type SYSCALL, got %s", event.Type)
	}

	if event.Serial != 12345 {
		t.Errorf("Expected serial 12345, got %d", event.Serial)
	}

	if event.ProcessID != 5678 {
		t.Errorf("Expected pid 5678, got %d", event.ProcessID)
	}

	if event.Syscall != 2 {
		t.Errorf("Expected syscall 2, got %d", event.Syscall)
	}
}
