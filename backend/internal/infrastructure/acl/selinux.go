package acl

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// SELinuxContext represents a SELinux security context.
type SELinuxContext struct {
	User    string // SELinux user (e.g., "unconfined_u")
	Role    string // SELinux role (e.g., "system_r")
	Type    string // SELinux type (e.g., "httpd_sys_content_t")
	Level   string // MLS level (e.g., "s0" or "s0:c1.c2")
	Path    string
	RawLine string
}

// String returns the full SELinux context string.
func (s *SELinuxContext) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s:%s", s.User, s.Role, s.Type, s.Level)
}

// GetSELinuxContext retrieves the SELinux context for a file.
func GetSELinuxContext(path string) (*SELinuxContext, error) {
	// Try stat with -c format first
	cmd := exec.Command("stat", "-c", "%C", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// SELinux might not be enabled
		return nil, fmt.Errorf("stat -c %%C: %w", err)
	}

	context := strings.TrimSpace(stdout.String())
	if context == "" || context == "?" {
		return nil, fmt.Errorf("SELinux context not available for %s", path)
	}

	return ParseSELinuxContext(path, context)
}

// ParseSELinuxContext parses a SELinux context string.
func ParseSELinuxContext(path, context string) (*SELinuxContext, error) {
	parts := strings.Split(context, ":")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SELinux context: %s", context)
	}

	sc := &SELinuxContext{
		User:    parts[0],
		Role:    parts[1],
		Type:    parts[2],
		Level:   "",
		Path:    path,
		RawLine: context,
	}

	if len(parts) >= 4 {
		// Level can contain MLS components like "s0:c1,c2"
		sc.Level = strings.Join(parts[3:], ":")
	}

	return sc, nil
}

// GetSELinuxLabel retrieves the full SELinux label via getfattr.
func GetSELinuxLabel(path string) (*SELinuxContext, error) {
	// Try getfattr for SELinux labels
	cmd := exec.Command("getfattr", "-h", "-n", "security.selinux", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Fall back to stat
		return GetSELinuxContext(path)
	}

	// Parse output: security.selinux="user:role:type:level"
	output := strings.TrimSpace(stdout.String())
	re := regexp.MustCompile(`security\.selinux="([^"]+)"`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return GetSELinuxContext(path)
	}

	return ParseSELinuxContext(path, matches[1])
}

// CompareSELinuxContexts compares two SELinux contexts.
func CompareSELinuxContexts(before, after *SELinuxContext) *SELinuxChange {
	if before == nil && after == nil {
		return nil
	}

	if before == nil {
		return &SELinuxChange{
			Type:    ChangeAdded,
			Context: after,
		}
	}

	if after == nil {
		return &SELinuxChange{
			Type:     ChangeRemoved,
			Old:     before,
			Context: before,
		}
	}

	change := &SELinuxChange{
		Old:     before,
		New:     after,
		Context: after,
	}

	if before.User != after.User {
		change.UserChanged = true
		change.Changed = true
	}
	if before.Role != after.Role {
		change.RoleChanged = true
		change.Changed = true
	}
	if before.Type != after.Type {
		change.TypeChanged = true
		change.Changed = true
	}
	if before.Level != after.Level {
		change.LevelChanged = true
		change.Changed = true
	}

	if change.Changed {
		change.Type = ChangeModified
	}

	return change
}

// SELinuxChange represents a change in SELinux context.
type SELinuxChange struct {
	Type         ChangeType
	Changed      bool
	UserChanged  bool
	RoleChanged  bool
	TypeChanged  bool
	LevelChanged bool
	Old         *SELinuxContext
	New         *SELinuxContext
	Context     *SELinuxContext
}

// String returns a human-readable description of the change.
func (c *SELinuxChange) String() string {
	if c == nil {
		return ""
	}

	switch c.Type {
	case ChangeAdded:
		return fmt.Sprintf("SELinux context added: %s", c.Context)
	case ChangeRemoved:
		return fmt.Sprintf("SELinux context removed: %s", c.Old)
	case ChangeModified:
		changes := []string{}
		if c.UserChanged {
			changes = append(changes, fmt.Sprintf("user: %s->%s", c.Old.User, c.New.User))
		}
		if c.RoleChanged {
			changes = append(changes, fmt.Sprintf("role: %s->%s", c.Old.Role, c.New.Role))
		}
		if c.TypeChanged {
			changes = append(changes, fmt.Sprintf("type: %s->%s", c.Old.Type, c.New.Type))
		}
		if c.LevelChanged {
			changes = append(changes, fmt.Sprintf("level: %s->%s", c.Old.Level, c.New.Level))
		}
		return fmt.Sprintf("SELinux context changed: %s", strings.Join(changes, ", "))
	}

	return ""
}

// SELinuxMonitor monitors paths for SELinux context changes.
type SELinuxMonitor struct {
	paths []string
}

// NewSELinuxMonitor creates a new SELinux monitor.
func NewSELinuxMonitor(paths ...string) *SELinuxMonitor {
	return &SELinuxMonitor{paths: paths}
}

// Scan scans all paths for SELinux contexts.
func (m *SELinuxMonitor) Scan() (map[string]*SELinuxContext, error) {
	results := make(map[string]*SELinuxContext)

	for _, path := range m.paths {
		ctx, err := GetSELinuxContext(path)
		if err != nil {
			// File might not exist or SELinux not enabled
			if os.IsNotExist(err) {
				continue
			}
			// Non-critical error, continue
			continue
		}
		results[path] = ctx
	}

	return results, nil
}

// Snapshot creates a snapshot of current SELinux contexts.
func (m *SELinuxMonitor) Snapshot() (map[string]string, error) {
	contexts, err := m.Scan()
	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]string)
	for path, ctx := range contexts {
		snapshot[path] = ctx.String()
	}

	return snapshot, nil
}

// DetectChanges detects SELinux context changes.
func (m *SELinuxMonitor) DetectChanges(snapshot map[string]string) ([]SELinuxChangeResult, error) {
	current, err := m.Scan()
	if err != nil {
		return nil, err
	}

	var results []SELinuxChangeResult

	for path, ctx := range current {
		savedCtx, wasMonitored := snapshot[path]
		if !wasMonitored {
			results = append(results, SELinuxChangeResult{
				Path:     path,
				HasNew:   true,
				New:      ctx,
			})
			continue
		}

		// Parse saved context
		saved, err := ParseSELinuxContext(path, savedCtx)
		if err != nil {
			continue
		}

		// Compare
		change := CompareSELinuxContexts(saved, ctx)
		if change != nil && change.Changed {
			results = append(results, SELinuxChangeResult{
				Path:    path,
				Old:     saved,
				New:     ctx,
				Changes: []*SELinuxChange{change},
			})
		}
	}

	// Check for removed contexts
	for path := range snapshot {
		if _, exists := current[path]; !exists {
			results = append(results, SELinuxChangeResult{
				Path:     path,
				Removed: true,
			})
		}
	}

	return results, nil
}

// SELinuxChangeResult represents the result of SELinux change detection.
type SELinuxChangeResult struct {
	Path     string
	Removed  bool
	HasNew   bool
	Changes  []*SELinuxChange
	Old      *SELinuxContext
	New      *SELinuxContext
}

// HasChanges returns true if there are any changes.
func (r SELinuxChangeResult) HasChanges() bool {
	return r.Removed || r.HasNew || len(r.Changes) > 0
}

// GetSecurityContext retrieves SELinux context information.
func GetSecurityContext(path string) (*SecurityContext, error) {
	sc := &SecurityContext{Path: path}

	// Get SELinux context
	ctx, err := GetSELinuxContext(path)
	if err == nil && ctx != nil {
		sc.SELinux = ctx
	}

	// Get file capabilities (if any)
	caps, err := GetFileCapabilities(path)
	if err == nil && caps != nil {
		sc.Capabilities = caps
	}

	return sc, nil
}

// SecurityContext combines SELinux and other security attributes.
type SecurityContext struct {
	Path          string
	SELinux      *SELinuxContext
	Capabilities  *Capabilities
	ACLInfo      *ACL
	XAttrInfo    *XAttr
}

// Capabilities represents Linux file capabilities.
type Capabilities struct {
	Path    string
	Version int
	Effective []string
	Permitted []string
	Inheritable []string
}

// GetFileCapabilities retrieves file capabilities.
func GetFileCapabilities(path string) (*Capabilities, error) {
	cmd := exec.Command("getcap", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// No capabilities set
		return nil, nil
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, nil
	}

	return ParseCapabilities(path, output)
}

// ParseCapabilities parses getcap output.
func ParseCapabilities(path, output string) (*Capabilities, error) {
	caps := &Capabilities{
		Path:         path,
		Effective:    []string{},
		Permitted:    []string{},
		Inheritable:  []string{},
	}

	// Output format: path = capability_name
	parts := strings.SplitN(output, "=", 2)
	if len(parts) < 2 {
		return caps, nil
	}

	capStr := strings.TrimSpace(parts[1])
	capStr = strings.Trim(capStr, " ")

	// Split by space
	capsList := strings.Fields(capStr)
	for _, cap := range capsList {
		// Check for effective marker
		if strings.HasSuffix(cap, "+e") {
			caps.Effective = append(caps.Effective, strings.TrimSuffix(cap, "+e"))
		} else if strings.HasSuffix(cap, "+i") {
			caps.Inheritable = append(caps.Inheritable, strings.TrimSuffix(cap, "+i"))
		} else {
			caps.Permitted = append(caps.Permitted, cap)
		}
	}

	return caps, nil
}
