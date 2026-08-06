package audit

import (
	"fmt"
	"strings"
)

// RuleType represents the type of audit rule.
type RuleType string

const (
	RuleTypeFile     RuleType = "-w" // Watch file/directory
	RuleTypeSyscall  RuleType = "-a" // Add syscall rule
	RuleTypeField    RuleType = "-F" // Add filter field
	RuleTypeAction   RuleType = "-D" // Delete all rules
)

// Action represents what to do when rule matches.
type Action string

const (
	ActionAlways Action = "-F perm=war" // Always audit read/write/attr
	ActionRead   Action = "-F perm=r"
	ActionWrite  Action = "-F perm=w"
	ActionExec   Action = "-F perm=x"
	ActionAttr   Action = "-F perm=a"
)

// Key represents the key/label for a rule.
type Key struct {
	Name  string
	Value string
}

// Rule represents an audit rule.
type Rule struct {
	Type      RuleType
	Path      string      // For file rules
	Syscall   string      // For syscall rules
	Actions   []Action    // Filter actions
	Key       *Key        // Optional key
	Fields    map[string]string // Additional fields
	Executable string     // For execve rules
}

// RuleSet represents a collection of audit rules.
type RuleSet struct {
	Rules    []*Rule
	WatchPaths []string
}

// NewRuleSet creates a new rule set.
func NewRuleSet() *RuleSet {
	return &RuleSet{
		Rules:      make([]*Rule, 0),
		WatchPaths: make([]string, 0),
	}
}

// AddFileWatch adds a file/directory watch rule.
func (rs *RuleSet) AddFileWatch(path string, perm ...Action) *RuleSet {
	rule := &Rule{
		Type: RuleTypeFile,
		Path: path,
	}
	if len(perm) == 0 {
		// Default: watch all permission changes
		rule.Actions = []Action{ActionAlways}
	} else {
		rule.Actions = perm
	}
	rs.Rules = append(rs.Rules, rule)
	rs.WatchPaths = append(rs.WatchPaths, path)
	return rs
}

// AddSyscallRule adds a syscall rule.
func (rs *RuleSet) AddSyscallRule(syscall string, actions ...Action) *RuleSet {
	rule := &Rule{
		Type:    RuleTypeSyscall,
		Syscall: syscall,
		Actions: actions,
	}
	rs.Rules = append(rs.Rules, rule)
	return rs
}

// AddKey adds a key/label to the last added rule.
func (rs *RuleSet) AddKey(name, value string) *RuleSet {
	if len(rs.Rules) > 0 {
		rs.Rules[len(rs.Rules)-1].Key = &Key{Name: name, Value: value}
	}
	return rs
}

// AddField adds a filter field.
func (rs *RuleSet) AddField(name, value string) *RuleSet {
	if len(rs.Rules) > 0 {
		rule := rs.Rules[len(rs.Rules)-1]
		if rule.Fields == nil {
			rule.Fields = make(map[string]string)
		}
		rule.Fields[name] = value
	}
	return rs
}

// AddExecWatch adds a watch for execution of a specific program.
func (rs *RuleSet) AddExecWatch(path string) *RuleSet {
	rule := &Rule{
		Type:       RuleTypeSyscall,
		Syscall:    "execve",
		Executable: path,
		Actions:    []Action{ActionExec},
	}
	rs.Rules = append(rs.Rules, rule)
	return rs
}

// String converts a rule to auditctl command format.
func (r *Rule) String() string {
	var parts []string

	switch r.Type {
	case RuleTypeFile:
		parts = append(parts, "-w", r.Path)
		for _, a := range r.Actions {
			parts = append(parts, "-p")
			perm := string(a)
			// Extract permission from "-F perm=war"
			if idx := strings.Index(perm, "="); idx != -1 {
				parts = append(parts, perm[idx+1:])
			} else {
				parts = append(parts, perm)
			}
		}
		if r.Key != nil {
			parts = append(parts, "-k", r.Key.Value)
		}

	case RuleTypeSyscall:
		for _, a := range r.Actions {
			parts = append(parts, "-a")
			parts = append(parts, strings.Fields(string(a))...)
		}
		if r.Syscall != "" {
			parts = append(parts, "-S", r.Syscall)
		}
		if r.Executable != "" {
			parts = append(parts, "-F", "path="+r.Executable)
		}
		if r.Key != nil {
			parts = append(parts, "-k", r.Key.Value)
		}

	case RuleTypeField:
		for name, value := range r.Fields {
			parts = append(parts, "-F", name+"="+value)
		}
	}

	return strings.Join(parts, " ")
}

// String converts the rule set to auditctl commands.
func (rs *RuleSet) String() string {
	var lines []string

	for _, rule := range rs.Rules {
		cmd := rule.String()
		if cmd != "" {
			lines = append(lines, cmd)
		}
	}

	return strings.Join(lines, "\n")
}

// GenerateScript generates a shell script to apply rules.
func (rs *RuleSet) GenerateScript(serviceName string) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Generated audit rules for FIM monitoring\n")
	sb.WriteString("# Service: " + serviceName + "\n\n")

	sb.WriteString("# Clear existing FIM rules\n")
	sb.WriteString("auditctl -D 2>/dev/null || true\n\n")

	sb.WriteString("# Add FIM watch rules\n")
	for _, rule := range rs.Rules {
		cmd := rule.String()
		if cmd != "" {
			sb.WriteString("auditctl " + cmd + "\n")
		}
	}

	sb.WriteString("\n# Verify rules\n")
	sb.WriteString("auditctl -l\n")

	return sb.String()
}

// FIMRuleSet creates a standard FIM audit rule set.
func FIMRuleSet(basePath string) *RuleSet {
	rs := NewRuleSet()

	// Watch critical system files
	criticalFiles := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/group",
		"/etc/gshadow",
		"/etc/sudoers",
		"/etc/sudoers.d/",
	}

	for _, f := range criticalFiles {
		rs.AddFileWatch(f, ActionAlways).AddKey("fim", "critical-system")
	}

	// Watch configuration directories
	configDirs := []string{
		"/etc/httpd/",
		"/etc/nginx/",
		"/etc/php/",
		"/etc/mysql/",
		"/etc/postgresql/",
	}

	for _, d := range configDirs {
		rs.AddFileWatch(d, ActionAlways).AddKey("fim", "config")
	}

	// Watch application-specific paths
	if basePath != "" {
		rs.AddFileWatch(basePath, ActionWrite, ActionAttr).
			AddKey("fim", "application")
	}

	// Watch log directories
	rs.AddFileWatch("/var/log/", ActionWrite).
		AddKey("fim", "logs")

	// Monitor execution of privilege escalation tools
	privescTools := []string{
		"/usr/bin/sudo",
		"/usr/bin/su",
		"/bin/bash",
		"/bin/sh",
	}

	for _, tool := range privescTools {
		rs.AddExecWatch(tool).AddKey("fim", "privesc")
	}

	return rs
}

// ComplianceRuleSet creates audit rules for compliance monitoring.
func ComplianceRuleSet() *RuleSet {
	rs := NewRuleSet()

	// SOC2/Japanese compliance: Monitor all privileged access
	rs.AddSyscallRule("execve").
		AddField("auid", ">=1000").
		AddField("euid", "0").
		AddKey("compliance", "privileged-exec")

	// Monitor file permission changes
	rs.AddSyscallRule("chmod").AddKey("compliance", "perm-change")
	rs.AddSyscallRule("chown").AddKey("compliance", "owner-change")
	rs.AddSyscallRule("fchmod").AddKey("compliance", "perm-change")
	rs.AddSyscallRule("fchown").AddKey("compliance", "owner-change")

	// Monitor user/group modifications
	rs.AddSyscallRule("useradd").AddKey("compliance", "user-mod")
	rs.AddSyscallRule("userdel").AddKey("compliance", "user-mod")
	rs.AddSyscallRule("groupadd").AddKey("compliance", "group-mod")
	rs.AddSyscallRule("groupdel").AddKey("compliance", "group-mod")

	// Monitor SSH connections
	rs.AddFileWatch("/etc/ssh/sshd_config", ActionAlways).
		AddKey("compliance", "ssh-config")
	rs.AddSyscallRule("open").AddField("path", "/var/log/btmp").
		AddKey("compliance", "login-attempt")

	return rs
}

// Validate checks if the rule set is valid.
func (rs *RuleSet) Validate() error {
	if len(rs.Rules) == 0 {
		return fmt.Errorf("rule set is empty")
	}

	for i, rule := range rs.Rules {
		if rule.Type == RuleTypeFile && rule.Path == "" {
			return fmt.Errorf("rule %d: file path required", i)
		}
		if rule.Type == RuleTypeSyscall && rule.Syscall == "" && rule.Executable == "" {
			return fmt.Errorf("rule %d: syscall or executable required", i)
		}
	}

	return nil
}

// GenerateRulesFile generates the contents for /etc/audit/rules.d/fim.rules.
func (rs *RuleSet) GenerateRulesFile() string {
	var sb strings.Builder

	sb.WriteString("# FIM Audit Rules\n")
	sb.WriteString("# Generated by OJS Monitor\n")
	sb.WriteString("#\n\n")

	sb.WriteString("# Delete existing rules first\n")
	sb.WriteString("-D\n\n")

	sb.WriteString("# Buffer size\n")
	sb.WriteString("-b 8192\n\n")

	sb.WriteString("# Failure mode: 0=silent, 1=printk, 2=panic\n")
	sb.WriteString("--failure=1\n\n")

	sb.WriteString("# Ignore errors\n")
	sb.WriteString("##\n\n")

	sb.WriteString("# FIM Watch Rules\n")
	for _, rule := range rs.Rules {
		cmd := rule.String()
		if cmd != "" {
			sb.WriteString(cmd + "\n")
		}
	}

	return sb.String()
}

// OJSRuleSet creates audit rules specifically for OJS installations.
func OJSRuleSet(ojsPath string) *RuleSet {
	rs := NewRuleSet()

	if ojsPath == "" {
		ojsPath = "/var/www/ojs"
	}

	// Watch OJS application files
	rs.AddFileWatch(ojsPath+"/config.inc.php", ActionAlways).
		AddKey("ojs", "config")

	rs.AddFileWatch(ojsPath+"/public/", ActionWrite, ActionAttr).
		AddKey("ojs", "public-upload")

	rs.AddFileWatch(ojsPath+"/files/", ActionWrite, ActionAttr).
		AddKey("ojs", "user-uploads")

	rs.AddFileWatch(ojsPath+"/cache/", ActionWrite, ActionAttr).
		AddKey("ojs", "cache")

	rs.AddFileWatch(ojsPath+"/plugins/", ActionWrite, ActionAttr).
		AddKey("ojs", "plugins")

	rs.AddFileWatch(ojsPath+"/locale/", ActionWrite, ActionAttr).
		AddKey("ojs", "locale")

	rs.AddFileWatch(ojsPath+"/templates/", ActionWrite, ActionAttr).
		AddKey("ojs", "templates")

	// Monitor PHP execution in OJS context
	rs.AddSyscallRule("execve").
		AddField("path", ojsPath).
		AddKey("ojs", "php-exec")

	return rs
}
