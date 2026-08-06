package audit

import (
	"strings"
	"testing"
)

func TestRuleString(t *testing.T) {
	rule := &Rule{
		Type:    RuleTypeFile,
		Path:    "/etc/passwd",
		Actions: []Action{ActionAlways},
		Key:     &Key{Name: "fim", Value: "critical"},
	}

	expected := "-w /etc/passwd -p war -k critical"
	got := rule.String()

	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestRuleSetAddFileWatch(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/etc/passwd", ActionRead)

	if len(rs.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rs.Rules))
	}

	if rs.Rules[0].Type != RuleTypeFile {
		t.Errorf("Expected RuleTypeFile, got %s", rs.Rules[0].Type)
	}

	if rs.Rules[0].Path != "/etc/passwd" {
		t.Errorf("Expected path /etc/passwd, got %s", rs.Rules[0].Path)
	}
}

func TestRuleSetAddSyscallRule(t *testing.T) {
	rs := NewRuleSet()
	rs.AddSyscallRule("execve").AddKey("test", "exec")

	if len(rs.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rs.Rules))
	}

	if rs.Rules[0].Syscall != "execve" {
		t.Errorf("Expected syscall execve, got %s", rs.Rules[0].Syscall)
	}

	if rs.Rules[0].Key == nil || rs.Rules[0].Key.Value != "exec" {
		t.Errorf("Expected key 'exec', got %v", rs.Rules[0].Key)
	}
}

func TestRuleSetAddExecWatch(t *testing.T) {
	rs := NewRuleSet()
	rs.AddExecWatch("/usr/bin/sudo").AddKey("privesc", "sudo")

	if len(rs.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rs.Rules))
	}

	if rs.Rules[0].Executable != "/usr/bin/sudo" {
		t.Errorf("Expected executable /usr/bin/sudo, got %s", rs.Rules[0].Executable)
	}
}

func TestRuleSetGenerateScript(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/etc/passwd", ActionAlways).
		AddKey("fim", "critical")

	script := rs.GenerateScript("test-service")

	if !strings.Contains(script, "# Service: test-service") {
		t.Error("Script should contain service name")
	}

	if !strings.Contains(script, "auditctl -D") {
		t.Error("Script should clear existing rules")
	}

	if !strings.Contains(script, "auditctl -w /etc/passwd") {
		t.Error("Script should contain file watch rule")
	}

	if !strings.Contains(script, "auditctl -l") {
		t.Error("Script should verify rules")
	}
}

func TestFIMRuleSet(t *testing.T) {
	rs := FIMRuleSet("/var/www/html")

	if len(rs.Rules) == 0 {
		t.Error("FIMRuleSet should create rules")
	}

	if len(rs.WatchPaths) == 0 {
		t.Error("FIMRuleSet should track watch paths")
	}

	// Should contain system files
	found := false
	for _, rule := range rs.Rules {
		if rule.Path == "/etc/passwd" {
			found = true
			break
		}
	}
	if !found {
		t.Error("FIMRuleSet should watch /etc/passwd")
	}
}

func TestComplianceRuleSet(t *testing.T) {
	rs := ComplianceRuleSet()

	if len(rs.Rules) == 0 {
		t.Error("ComplianceRuleSet should create rules")
	}

	// Should contain syscall rules
	found := false
	for _, rule := range rs.Rules {
		if rule.Syscall == "chmod" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ComplianceRuleSet should contain chmod syscall rules")
	}
}

func TestOJSRuleSet(t *testing.T) {
	rs := OJSRuleSet("/var/www/ojs")

	if len(rs.Rules) == 0 {
		t.Error("OJSRuleSet should create rules")
	}

	// Should contain config watch
	found := false
	for _, rule := range rs.Rules {
		if strings.Contains(rule.Path, "config.inc.php") {
			found = true
			break
		}
	}
	if !found {
		t.Error("OJSRuleSet should watch config.inc.php")
	}
}

func TestRuleSetValidate(t *testing.T) {
	// Empty rule set
	rs := NewRuleSet()
	err := rs.Validate()
	if err == nil {
		t.Error("Empty rule set should fail validation")
	}

	// Valid rule set
	rs.AddFileWatch("/etc/passwd")
	err = rs.Validate()
	if err != nil {
		t.Errorf("Valid rule set should pass validation: %v", err)
	}
}

func TestRuleSetValidateFilePath(t *testing.T) {
	rs := NewRuleSet()
	rs.Rules = append(rs.Rules, &Rule{
		Type: RuleTypeFile,
		Path: "",
	})

	err := rs.Validate()
	if err == nil {
		t.Error("File rule without path should fail validation")
	}
}

func TestRuleSetValidateSyscall(t *testing.T) {
	rs := NewRuleSet()
	rs.Rules = append(rs.Rules, &Rule{
		Type:    RuleTypeSyscall,
		Syscall: "",
	})

	err := rs.Validate()
	if err == nil {
		t.Error("Syscall rule without syscall should fail validation")
	}
}

func TestGenerateRulesFile(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/etc/passwd").AddKey("test", "passwd")

	rules := rs.GenerateRulesFile()

	if !strings.Contains(rules, "# FIM Audit Rules") {
		t.Error("Rules file should contain header")
	}

	if !strings.Contains(rules, "-D") {
		t.Error("Rules file should clear existing rules")
	}

	if !strings.Contains(rules, "/etc/passwd") {
		t.Error("Rules file should contain passwd watch")
	}
}

func TestRuleSetChaining(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/etc/passwd").
		AddKey("key1", "value1").
		AddField("arch", "b64")

	if len(rs.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rs.Rules))
	}

	rule := rs.Rules[0]
	if rule.Key == nil || rule.Key.Value != "value1" {
		t.Errorf("Expected key value1, got %v", rule.Key)
	}

	if rule.Fields["arch"] != "b64" {
		t.Errorf("Expected field arch=b64, got %v", rule.Fields)
	}
}

func TestMultiplePermissions(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/var/log/", ActionWrite, ActionAttr).
		AddKey("logs", "writable")

	if len(rs.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rs.Rules))
	}

	if len(rs.Rules[0].Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(rs.Rules[0].Actions))
	}
}

func TestWatchPathsTracking(t *testing.T) {
	rs := NewRuleSet()
	rs.AddFileWatch("/etc/passwd")
	rs.AddFileWatch("/etc/shadow")
	rs.AddFileWatch("/etc/group")

	if len(rs.WatchPaths) != 3 {
		t.Errorf("Expected 3 watch paths, got %d", len(rs.WatchPaths))
	}

	if rs.WatchPaths[0] != "/etc/passwd" {
		t.Errorf("Expected /etc/passwd, got %s", rs.WatchPaths[0])
	}
}
