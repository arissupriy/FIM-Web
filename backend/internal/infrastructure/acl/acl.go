// Package acl provides ACL, xattr, and SELinux context monitoring.
package acl

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ACL represents an Access Control List entry.
type ACL struct {
	Path       string
	Type       ACLType // "access" or "default"
	Owner      string
	Group      string
	Entries    []ACLEntry
	RawOutput  string
}

// ACLType represents the type of ACL.
type ACLType string

const (
	ACLTypeAccess  ACLType = "access"
	ACLTypeDefault ACLType = "default"
)

// ACLEntry represents a single ACL entry.
type ACLEntry struct {
	Tag     ACLTag   // "user", "group", "mask", "other"
	Qual    string   // Named user/group name or UID/GID
	Perms   Permissions // Read/Write/Execute permissions
	RawLine string   // Original line from getfacl
}

// ACLTag represents the type of ACL entry.
type ACLTag string

const (
	ACLTagUser    ACLTag = "user"
	ACLTagGroup   ACLTag = "group"
	ACLTagMask    ACLTag = "mask"
	ACLTagOther   ACLTag = "other"
	ACLTagMaskACL ACLTag = "mask" // alias
)

// Permissions represents file permissions.
type Permissions struct {
	Read    bool
	Write   bool
	Execute bool
}

// String returns the permission string (rwx).
func (p Permissions) String() string {
	s := ""
	if p.Read {
		s += "r"
	} else {
		s += "-"
	}
	if p.Write {
		s += "w"
	} else {
		s += "-"
	}
	if p.Execute {
		s += "x"
	} else {
		s += "-"
	}
	return s
}

// ParsePermissions parses a permission string like "rw-" or "rwx".
func ParsePermissions(s string) Permissions {
	p := Permissions{}
	if len(s) >= 1 {
		p.Read = s[0] == 'r'
	}
	if len(s) >= 2 {
		p.Write = s[1] == 'w'
	}
	if len(s) >= 3 {
		p.Execute = s[2] == 'x'
	}
	return p
}

// Octal returns the octal representation.
func (p Permissions) Octal() string {
	v := 0
	if p.Read {
		v += 4
	}
	if p.Write {
		v += 2
	}
	if p.Execute {
		v += 1
	}
	return strconv.Itoa(v)
}

// GetACL retrieves the ACL for a file or directory.
func GetACL(path string) (*ACL, error) {
	cmd := exec.Command("getfacl", "-p", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getfacl %s: %w (%s)", path, err, stderr.String())
	}

	return ParseACLOutput(path, stdout.String())
}

// ParseACLOutput parses the output of getfacl.
func ParseACLOutput(path, output string) (*ACL, error) {
	acl := &ACL{
		Path:      path,
		Entries:   make([]ACLEntry, 0),
		RawOutput: output,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse owner/group comments
		if strings.HasPrefix(line, "# owner:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				acl.Owner = strings.TrimSpace(parts[1])
			}
			continue
		}
		if strings.HasPrefix(line, "# group:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				acl.Group = strings.TrimSpace(parts[1])
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Parse owner/group
		if strings.HasPrefix(line, "user:") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 2 {
				if acl.Owner == "" {
					acl.Owner = parts[1]
				}
				if len(parts) >= 3 {
					acl.Entries = append(acl.Entries, ACLEntry{
						Tag:     ACLTagUser,
						Qual:    parts[1],
						Perms:   ParsePermissions(parts[2]),
						RawLine: line,
					})
				}
			}
			continue
		}

		if strings.HasPrefix(line, "group:") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) >= 2 {
				if acl.Group == "" {
					acl.Group = parts[1]
				}
				if len(parts) >= 3 {
					acl.Entries = append(acl.Entries, ACLEntry{
						Tag:     ACLTagGroup,
						Qual:    parts[1],
						Perms:   ParsePermissions(parts[2]),
						RawLine: line,
					})
				}
			}
			continue
		}

		if strings.HasPrefix(line, "mask:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				acl.Entries = append(acl.Entries, ACLEntry{
					Tag:     ACLTagMask,
					Perms:   ParsePermissions(parts[1]),
					RawLine: line,
				})
			}
			continue
		}

		if strings.HasPrefix(line, "other:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				acl.Entries = append(acl.Entries, ACLEntry{
					Tag:     ACLTagOther,
					Perms:   ParsePermissions(parts[1]),
					RawLine: line,
				})
			}
			continue
		}

		if strings.HasPrefix(line, "default:") {
			acl.Type = ACLTypeDefault
			// Parse default entries
			defaultLine := strings.TrimPrefix(line, "default:")
			parts := strings.SplitN(defaultLine, ":", 3)
			if len(parts) >= 2 {
				tag := ACLTag(parts[0])
				if len(parts) >= 3 {
					acl.Entries = append(acl.Entries, ACLEntry{
						Tag:     tag,
						Qual:    parts[1],
						Perms:   ParsePermissions(parts[2]),
						RawLine: line,
					})
				}
			}
		}
	}

	return acl, nil
}

// HasExtendedACL checks if the file has extended ACL entries.
func (a *ACL) HasExtendedACL() bool {
	return len(a.Entries) > 3 // basic has owner, group, other
}

// GetNamedUsers returns all named user ACL entries.
func (a *ACL) GetNamedUsers() []ACLEntry {
	var entries []ACLEntry
	for _, e := range a.Entries {
		if e.Tag == ACLTagUser && e.Qual != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// GetNamedGroups returns all named group ACL entries.
func (a *ACL) GetNamedGroups() []ACLEntry {
	var entries []ACLEntry
	for _, e := range a.Entries {
		if e.Tag == ACLTagGroup && e.Qual != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// CompareACLs compares two ACLs and returns the differences.
func CompareACLs(before, after *ACL) []ACLChange {
	var changes []ACLChange

	// Build maps for comparison
	beforeMap := make(map[string]ACLEntry)
	afterMap := make(map[string]ACLEntry)

	for _, e := range before.Entries {
		key := fmt.Sprintf("%s:%s", e.Tag, e.Qual)
		beforeMap[key] = e
	}

	for _, e := range after.Entries {
		key := fmt.Sprintf("%s:%s", e.Tag, e.Qual)
		afterMap[key] = e
	}

	// Find removed entries
	for key, e := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			changes = append(changes, ACLChange{
				Type:      ChangeRemoved,
				Entry:     e,
				OldPerms:  &e.Perms,
				NewPerms:  nil,
			})
		}
	}

	// Find added and modified entries
	for key, e := range afterMap {
		beforeEntry, exists := beforeMap[key]
		if !exists {
			changes = append(changes, ACLChange{
				Type:    ChangeAdded,
				Entry:   e,
				NewPerms: &e.Perms,
			})
		} else if beforeEntry.Perms != e.Perms {
			changes = append(changes, ACLChange{
				Type:      ChangeModified,
				Entry:     e,
				OldPerms:  &beforeEntry.Perms,
				NewPerms:  &e.Perms,
			})
		}
	}

	return changes
}

// ChangeType represents the type of ACL change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

// ACLChange represents a change in an ACL entry.
type ACLChange struct {
	Type     ChangeType
	Entry    ACLEntry
	OldPerms *Permissions
	NewPerms *Permissions
}

// String returns a human-readable description of the change.
func (c ACLChange) String() string {
	switch c.Type {
	case ChangeAdded:
		return fmt.Sprintf("Added %s:%s %s", c.Entry.Tag, c.Entry.Qual, c.Entry.Perms.String())
	case ChangeRemoved:
		return fmt.Sprintf("Removed %s:%s", c.Entry.Tag, c.Entry.Qual)
	case ChangeModified:
		return fmt.Sprintf("Changed %s:%s from %s to %s", c.Entry.Tag, c.Entry.Qual, c.OldPerms.String(), c.NewPerms.String())
	default:
		return "Unknown change"
	}
}

// Monitor watches a path for ACL changes.
type Monitor struct {
	paths []string
}

// NewMonitor creates a new ACL monitor.
func NewMonitor(paths ...string) *Monitor {
	return &Monitor{paths: paths}
}

// Scan performs a full ACL scan on all monitored paths.
func (m *Monitor) Scan() (map[string]*ACL, error) {
	results := make(map[string]*ACL)

	for _, path := range m.paths {
		acl, err := GetACL(path)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", path, err)
		}
		results[path] = acl
	}

	return results, nil
}

// Snapshot creates a snapshot of current ACLs for comparison.
func (m *Monitor) Snapshot() (map[string]string, error) {
	acls, err := m.Scan()
	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]string)
	for path, acl := range acls {
		snapshot[path] = acl.RawOutput
	}

	return snapshot, nil
}

// DetectChanges compares a snapshot with current ACLs.
func (m *Monitor) DetectChanges(snapshot map[string]string) ([]ACLChangeResult, error) {
	current, err := m.Scan()
	if err != nil {
		return nil, err
	}

	var results []ACLChangeResult

	for path, currentACL := range current {
		snapshotOutput, wasMonitored := snapshot[path]
		if !wasMonitored {
			results = append(results, ACLChangeResult{
				Path:     path,
				HasACL:   true,
				NewACL:   currentACL,
			})
			continue
		}

		// Parse snapshot
		snapshotACL, err := ParseACLOutput(path, snapshotOutput)
		if err != nil {
			continue
		}

		// Compare
		changes := CompareACLs(snapshotACL, currentACL)
		if len(changes) > 0 {
			results = append(results, ACLChangeResult{
				Path:    path,
				Changes: changes,
				OldACL:  snapshotACL,
				NewACL:  currentACL,
			})
		}
	}

	// Check for removed ACLs
	for path := range snapshot {
		if _, exists := current[path]; !exists {
			results = append(results, ACLChangeResult{
				Path:    path,
				Removed: true,
			})
		}
	}

	return results, nil
}

// ACLChangeResult represents the result of detecting ACL changes.
type ACLChangeResult struct {
	Path    string
	Removed bool
	HasACL  bool
	Changes []ACLChange
	OldACL  *ACL
	NewACL  *ACL
}

// HasChanges returns true if there are any ACL changes.
func (r ACLChangeResult) HasChanges() bool {
	return r.Removed || len(r.Changes) > 0
}
