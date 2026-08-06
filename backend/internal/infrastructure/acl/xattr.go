package acl

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// XAttr represents extended attributes of a file.
type XAttr struct {
	Path    string
	Attrs   map[string]string
	RawList string
}

// GetXAttr retrieves extended attributes for a file.
func GetXAttr(path string) (*XAttr, error) {
	// First, list all attributes
	cmd := exec.Command("getfattr", "-d", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// getfattr returns exit code 0 even if no attrs
	if err := cmd.Run(); err != nil {
		// Check if it's "no attributes" error
		if !strings.Contains(stderr.String(), "No such attribute") &&
			!strings.Contains(stdout.String(), "# file:") {
			// Try alternate method
			return listXAttr(path)
		}
	}

	return parseXAttrOutput(path, stdout.String())
}

// listXAttr uses a different method to list attributes.
func listXAttr(path string) (*XAttr, error) {
	cmd := exec.Command("getfattr", "-h", "-m", "^security\\.|^system\\.", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// No attributes or error
		if strings.Contains(stderr.String(), "No such attribute") ||
			err.Error() == "exit status 1" {
			return &XAttr{Path: path, Attrs: make(map[string]string)}, nil
		}
	}

	xattr, _ := parseXAttrOutput(path, stdout.String())
	return xattr, nil
}

// parseXAttrOutput parses the output of getfattr.
func parseXAttrOutput(path, output string) (*XAttr, error) {
	xattr := &XAttr{
		Path:    path,
		Attrs:   make(map[string]string),
		RawList: output,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "# file:") {
			continue
		}

		// Parse key="value" format
		idx := strings.Index(line, "=")
		if idx != -1 {
			key := line[:idx]
			value := line[idx+1:]
			// Remove quotes if present
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}
			// Unescape
			value = unescapeXAttrValue(value)
			xattr.Attrs[key] = value
		}
	}

	return xattr, nil
}

// unescapeXAttrValue unescapes special characters in xattr values.
func unescapeXAttrValue(s string) string {
	// getfattr uses octal escapes for special chars
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+3 < len(s) {
			// Octal escape \NNN
			oct := s[i+1 : i+4]
			var b byte
			fmt.Sscanf(oct, "%o", &b)
			result.WriteByte(b)
			i += 4
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// GetAttr retrieves a specific extended attribute.
func GetAttr(path, name string) (string, error) {
	cmd := exec.Command("getfattr", "-h", "-n", name, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("getfattr %s.%s: %w", path, name, err)
	}

	// Parse output
	output := stdout.String()
	idx := strings.Index(output, "=")
	if idx == -1 {
		return "", nil
	}

	value := strings.TrimSpace(output[idx+1:])
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		value = value[1 : len(value)-1]
	}

	return unescapeXAttrValue(value), nil
}

// SetAttr sets an extended attribute.
func SetAttr(path, name, value string) error {
	cmd := exec.Command("setfattr", "-h", "-n", name, "-v", value, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setfattr %s.%s=%s: %w", path, name, value, err)
	}

	return nil
}

// RemoveAttr removes an extended attribute.
func RemoveAttr(path, name string) error {
	cmd := exec.Command("setfattr", "-h", "-x", name, path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("setfattr -x %s.%s: %w", path, name, err)
	}

	return nil
}

// CompareXAttrs compares two xattr snapshots and returns differences.
func CompareXAttrs(before, after *XAttr) []XAttrChange {
	var changes []XAttrChange

	// Find removed attributes
	for name, value := range before.Attrs {
		if _, exists := after.Attrs[name]; !exists {
			changes = append(changes, XAttrChange{
				Type:  ChangeRemoved,
				Name:  name,
				Old:   value,
				New:   "",
			})
		}
	}

	// Find added and modified attributes
	for name, newValue := range after.Attrs {
		oldValue, exists := before.Attrs[name]
		if !exists {
			changes = append(changes, XAttrChange{
				Type: ChangeAdded,
				Name: name,
				Old:  "",
				New:  newValue,
			})
		} else if oldValue != newValue {
			changes = append(changes, XAttrChange{
				Type: ChangeModified,
				Name: name,
				Old:  oldValue,
				New:  newValue,
			})
		}
	}

	return changes
}

// XAttrChange represents a change in an extended attribute.
type XAttrChange struct {
	Type ChangeType
	Name string
	Old  string
	New  string
}

// XAttrMonitor watches paths for xattr changes.
type XAttrMonitor struct {
	paths []string
}

// NewXAttrMonitor creates a new xattr monitor.
func NewXAttrMonitor(paths ...string) *XAttrMonitor {
	return &XAttrMonitor{paths: paths}
}

// Scan scans all paths for xattrs.
func (m *XAttrMonitor) Scan() (map[string]*XAttr, error) {
	results := make(map[string]*XAttr)

	for _, path := range m.paths {
		xattr, err := GetXAttr(path)
		if err != nil {
			// If the file doesn't exist or has no attrs, continue
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("scan %s: %w", path, err)
		}
		results[path] = xattr
	}

	return results, nil
}

// Snapshot creates a snapshot of current xattrs.
func (m *XAttrMonitor) Snapshot() (map[string]map[string]string, error) {
	xattrs, err := m.Scan()
	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]map[string]string)
	for path, xattr := range xattrs {
		snapshot[path] = xattr.Attrs
	}

	return snapshot, nil
}

// DetectChanges detects xattr changes from a snapshot.
func (m *XAttrMonitor) DetectChanges(snapshot map[string]map[string]string) ([]XAttrChangeResult, error) {
	current, err := m.Scan()
	if err != nil {
		return nil, err
	}

	var results []XAttrChangeResult

	for path, currentXAttr := range current {
		savedAttrs, wasMonitored := snapshot[path]
		if !wasMonitored {
			results = append(results, XAttrChangeResult{
				Path:   path,
				New:    currentXAttr,
				HasNew: true,
			})
			continue
		}

		// Compare
		var changes []XAttrChange
		for name, newValue := range currentXAttr.Attrs {
			oldValue, exists := savedAttrs[name]
			if !exists {
				changes = append(changes, XAttrChange{
					Type: ChangeAdded,
					Name: name,
					Old:  "",
					New:  newValue,
				})
			} else if oldValue != newValue {
				changes = append(changes, XAttrChange{
					Type: ChangeModified,
					Name: name,
					Old:  oldValue,
					New:  newValue,
				})
			}
		}

		// Check for removed attrs
		for name, oldValue := range savedAttrs {
			if _, exists := currentXAttr.Attrs[name]; !exists {
				changes = append(changes, XAttrChange{
					Type: ChangeRemoved,
					Name: name,
					Old:  oldValue,
					New:  "",
				})
			}
		}

		if len(changes) > 0 {
			results = append(results, XAttrChangeResult{
				Path:    path,
				Changes: changes,
				Old:     &XAttr{Path: path, Attrs: savedAttrs},
				New:     currentXAttr,
			})
		}
	}

	// Check for removed xattrs
	for path := range snapshot {
		if _, exists := current[path]; !exists {
			results = append(results, XAttrChangeResult{
				Path:     path,
				Removed: true,
			})
		}
	}

	return results, nil
}

// XAttrChangeResult represents xattr change detection results.
type XAttrChangeResult struct {
	Path     string
	Removed  bool
	HasNew   bool
	Changes  []XAttrChange
	Old      *XAttr
	New      *XAttr
}

// HasChanges returns true if there are any changes.
func (r XAttrChangeResult) HasChanges() bool {
	return r.Removed || r.HasNew || len(r.Changes) > 0
}

// SecurityAttrs returns security-related xattrs (SELinux, etc.).
func (x *XAttr) SecurityAttrs() map[string]string {
	security := make(map[string]string)
	prefixes := []string{"security.", "system.", "user."}

	for name, value := range x.Attrs {
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				security[name] = value
				break
			}
		}
	}

	return security
}
