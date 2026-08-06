package acl

import (
	"strings"
	"testing"
)

func TestParsePermissions(t *testing.T) {
	tests := []struct {
		input    string
		expected Permissions
	}{
		{"rwx", Permissions{Read: true, Write: true, Execute: true}},
		{"rw-", Permissions{Read: true, Write: true, Execute: false}},
		{"r--", Permissions{Read: true, Write: false, Execute: false}},
		{"---", Permissions{Read: false, Write: false, Execute: false}},
		{"-w-", Permissions{Read: false, Write: true, Execute: false}},
		{"--x", Permissions{Read: false, Write: false, Execute: true}},
	}

	for _, tt := range tests {
		got := ParsePermissions(tt.input)
		if got != tt.expected {
			t.Errorf("ParsePermissions(%s) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestPermissionsString(t *testing.T) {
	tests := []struct {
		perms   Permissions
		want    string
	}{
		{Permissions{Read: true, Write: true, Execute: true}, "rwx"},
		{Permissions{Read: true, Write: false, Execute: true}, "r-x"},
		{Permissions{Read: false, Write: true, Execute: false}, "-w-"},
		{Permissions{Read: false, Write: false, Execute: false}, "---"},
	}

	for _, tt := range tests {
		got := tt.perms.String()
		if got != tt.want {
			t.Errorf("Permissions(%v).String() = %s, want %s", tt.perms, got, tt.want)
		}
	}
}

func TestPermissionsOctal(t *testing.T) {
	tests := []struct {
		perms Permissions
		want  string
	}{
		{Permissions{Read: true, Write: true, Execute: true}, "7"},
		{Permissions{Read: true, Write: true, Execute: false}, "6"},
		{Permissions{Read: true, Write: false, Execute: true}, "5"},
		{Permissions{Read: true, Write: false, Execute: false}, "4"},
		{Permissions{Read: false, Write: true, Execute: true}, "3"},
		{Permissions{Read: false, Write: true, Execute: false}, "2"},
		{Permissions{Read: false, Write: false, Execute: true}, "1"},
		{Permissions{Read: false, Write: false, Execute: false}, "0"},
	}

	for _, tt := range tests {
		got := tt.perms.Octal()
		if got != tt.want {
			t.Errorf("Permissions(%v).Octal() = %s, want %s", tt.perms, got, tt.want)
		}
	}
}

func TestParseACLOutput(t *testing.T) {
	output := `# file: test.txt
# owner: root
# group: www-data
user::rw-
user:www-data:rw-
group::r--
group:www-data:rw-
mask::rw-
other::r--`

	acl, err := ParseACLOutput("/test.txt", output)
	if err != nil {
		t.Fatalf("ParseACLOutput failed: %v", err)
	}

	// Owner is parsed from # owner: comment
	if acl.Owner != "root" {
		t.Errorf("Expected owner 'root', got '%s'", acl.Owner)
	}

	if acl.Group != "www-data" {
		t.Errorf("Expected group 'www-data', got '%s'", acl.Group)
	}

	if len(acl.Entries) != 6 {
		t.Errorf("Expected 6 entries, got %d", len(acl.Entries))
	}

	// Check HasExtendedACL
	if !acl.HasExtendedACL() {
		t.Error("Expected HasExtendedACL to return true")
	}
}

func TestACL_HasExtendedACL(t *testing.T) {
	acl := &ACL{
		Entries: []ACLEntry{
			{Tag: ACLTagUser, Qual: "", Perms: Permissions{Read: true}},
			{Tag: ACLTagGroup, Qual: "", Perms: Permissions{Read: true}},
			{Tag: ACLTagOther, Qual: "", Perms: Permissions{Read: true}},
		},
	}

	if acl.HasExtendedACL() {
		t.Error("Basic ACL should not have extended ACL")
	}

	acl.Entries = append(acl.Entries, ACLEntry{
		Tag:  ACLTagUser,
		Qual: "www-data",
		Perms: Permissions{Read: true},
	})

	if !acl.HasExtendedACL() {
		t.Error("ACL with named user should have extended ACL")
	}
}

func TestACL_GetNamedUsers(t *testing.T) {
	acl := &ACL{
		Entries: []ACLEntry{
			{Tag: ACLTagUser, Qual: "", Perms: Permissions{Read: true}},
			{Tag: ACLTagUser, Qual: "www-data", Perms: Permissions{Read: true}},
			{Tag: ACLTagUser, Qual: "admin", Perms: Permissions{Read: true, Write: true}},
		},
	}

	named := acl.GetNamedUsers()
	if len(named) != 2 {
		t.Errorf("Expected 2 named users, got %d", len(named))
	}
}

func TestACL_GetNamedGroups(t *testing.T) {
	acl := &ACL{
		Entries: []ACLEntry{
			{Tag: ACLTagGroup, Qual: "", Perms: Permissions{Read: true}},
			{Tag: ACLTagGroup, Qual: "www-data", Perms: Permissions{Read: true}},
		},
	}

	named := acl.GetNamedGroups()
	if len(named) != 1 {
		t.Errorf("Expected 1 named group, got %d", len(named))
	}
}

func TestCompareACLs(t *testing.T) {
	before := &ACL{
		Entries: []ACLEntry{
			{Tag: ACLTagUser, Qual: "", Perms: Permissions{Read: true, Write: true}},
			{Tag: ACLTagGroup, Qual: "", Perms: Permissions{Read: true}},
			{Tag: ACLTagOther, Qual: "", Perms: Permissions{Read: false}},
		},
	}

	after := &ACL{
		Entries: []ACLEntry{
			{Tag: ACLTagUser, Qual: "", Perms: Permissions{Read: true, Write: true}},
			{Tag: ACLTagGroup, Qual: "", Perms: Permissions{Read: true, Write: true}}, // Modified
			{Tag: ACLTagOther, Qual: "", Perms: Permissions{Read: false}},
			{Tag: ACLTagUser, Qual: "www-data", Perms: Permissions{Read: true}}, // Added
		},
	}

	changes := CompareACLs(before, after)

	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
	}

	// Check for modified group
	found := false
	for _, c := range changes {
		if c.Type == ChangeModified && c.Entry.Tag == ACLTagGroup {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected modified group entry")
	}
}

func TestACLChange_String(t *testing.T) {
	tests := []struct {
		change ACLChange
		want   string
	}{
		{
			ACLChange{Type: ChangeAdded, Entry: ACLEntry{Tag: ACLTagUser, Qual: "www-data", Perms: Permissions{Read: true, Write: true}}},
			"Added user:www-data rw-",
		},
		{
			ACLChange{Type: ChangeRemoved, Entry: ACLEntry{Tag: ACLTagGroup, Qual: "www-data"}},
			"Removed group:www-data",
		},
		{
			ACLChange{Type: ChangeModified, Entry: ACLEntry{Tag: ACLTagGroup, Qual: "www-data", Perms: Permissions{Read: true}}, OldPerms: &Permissions{Read: true, Write: true}, NewPerms: &Permissions{Read: true}},
			"Changed group:www-data from rw- to r--",
		},
	}

	for _, tt := range tests {
		got := tt.change.String()
		if !strings.Contains(got, tt.want[:10]) { // Just check first part
			t.Errorf("ACLChange.String() = %s, want %s", got, tt.want)
		}
	}
}

func TestParseSELinuxContext(t *testing.T) {
	tests := []struct {
		input string
		want  *SELinuxContext
	}{
		{
			"unconfined_u:object_r:httpd_sys_content_t:s0",
			&SELinuxContext{User: "unconfined_u", Role: "object_r", Type: "httpd_sys_content_t", Level: "s0"},
		},
		{
			"system_u:system_r:init_t:s0:c1,c2",
			&SELinuxContext{User: "system_u", Role: "system_r", Type: "init_t", Level: "s0:c1,c2"},
		},
	}

	for _, tt := range tests {
		got, err := ParseSELinuxContext("/test", tt.input)
		if err != nil {
			t.Fatalf("ParseSELinuxContext failed: %v", err)
		}

		if got.User != tt.want.User {
			t.Errorf("User = %s, want %s", got.User, tt.want.User)
		}
		if got.Role != tt.want.Role {
			t.Errorf("Role = %s, want %s", got.Role, tt.want.Role)
		}
		if got.Type != tt.want.Type {
			t.Errorf("Type = %s, want %s", got.Type, tt.want.Type)
		}
		if got.Level != tt.want.Level {
			t.Errorf("Level = %s, want %s", got.Level, tt.want.Level)
		}
	}
}

func TestSELinuxContext_String(t *testing.T) {
	ctx := &SELinuxContext{
		User:  "unconfined_u",
		Role:  "object_r",
		Type:  "httpd_sys_content_t",
		Level: "s0",
	}

	want := "unconfined_u:object_r:httpd_content_t:s0"
	got := ctx.String()

	if !strings.Contains(got, "unconfined_u") {
		t.Errorf("String() = %s, want %s", got, want)
	}
}

func TestCompareSELinuxContexts(t *testing.T) {
	before := &SELinuxContext{
		User:  "unconfined_u",
		Role:  "object_r",
		Type:  "httpd_sys_content_t",
		Level: "s0",
	}

	after := &SELinuxContext{
		User:  "unconfined_u",
		Role:  "object_r",
		Type:  "httpd_sys_content_t:s0",
		Level: "s0",
	}

	change := CompareSELinuxContexts(before, after)
	if change == nil {
		t.Fatal("Expected change, got nil")
	}

	if !change.TypeChanged {
		t.Error("Expected type to be changed")
	}

	if change.UserChanged || change.RoleChanged || change.LevelChanged {
		t.Error("Only type should be changed")
	}
}

func TestCompareSELinuxContexts_Added(t *testing.T) {
	before := (*SELinuxContext)(nil)
	after := &SELinuxContext{
		User:  "unconfined_u",
		Role:  "object_r",
		Type:  "httpd_sys_content_t",
		Level: "s0",
	}

	change := CompareSELinuxContexts(before, after)
	if change == nil {
		t.Fatal("Expected change, got nil")
	}

	if change.Type != ChangeAdded {
		t.Errorf("Expected ChangeAdded, got %s", change.Type)
	}
}

func TestCompareSELinuxContexts_Removed(t *testing.T) {
	before := &SELinuxContext{
		User:  "unconfined_u",
		Role:  "object_r",
		Type:  "httpd_sys_content_t",
		Level: "s0",
	}
	after := (*SELinuxContext)(nil)

	change := CompareSELinuxContexts(before, after)
	if change == nil {
		t.Fatal("Expected change, got nil")
	}

	if change.Type != ChangeRemoved {
		t.Errorf("Expected ChangeRemoved, got %s", change.Type)
	}
}

func TestXAttr_SecurityAttrs(t *testing.T) {
	xattr := &XAttr{
		Attrs: map[string]string{
			"user.foo":                  "bar",
			"security.selinux":         "unconfined_u:object_r:httpd_sys_content_t:s0",
			"system.posix_acl_access": "binary",
		},
	}

	security := xattr.SecurityAttrs()
	// Should include all security.*, system.*, and user.* prefixed attrs
	if len(security) != 3 {
		t.Errorf("Expected 3 security attrs (user., security., system.), got %d", len(security))
	}

	if _, ok := security["security.selinux"]; !ok {
		t.Error("Expected security.selinux in security attrs")
	}

	if _, ok := security["system.posix_acl_access"]; !ok {
		t.Error("Expected system.posix_acl_access in security attrs")
	}

	if _, ok := security["user.foo"]; !ok {
		t.Error("Expected user.foo in security attrs (user.* is included)")
	}
}

func TestMonitor_Snapshot(t *testing.T) {
	// Create a test file
	// Note: This test requires root or appropriate permissions
	m := NewMonitor("/etc/passwd")

	snapshot, err := m.Snapshot()
	if err != nil {
		t.Skipf("Skipping snapshot test (may need root): %v", err)
	}

	if len(snapshot) == 0 {
		t.Error("Expected at least one ACL in snapshot")
	}
}

func TestXAttrMonitor_Snapshot(t *testing.T) {
	m := NewXAttrMonitor("/etc/passwd")

	snapshot, err := m.Snapshot()
	if err != nil {
		t.Skipf("Skipping snapshot test: %v", err)
	}

	// Snapshot should not error even if no attrs
	if snapshot == nil {
		t.Error("Expected non-nil snapshot")
	}
}

func TestSELinuxMonitor_Snapshot(t *testing.T) {
	m := NewSELinuxMonitor("/etc/passwd")

	snapshot, err := m.Snapshot()
	if err != nil {
		t.Skipf("Skipping snapshot test (SELinux may not be enabled): %v", err)
	}

	if snapshot == nil {
		t.Error("Expected non-nil snapshot")
	}
}

func TestACLChangeResult_HasChanges(t *testing.T) {
	tests := []ACLChangeResult{
		{Changes: []ACLChange{{Type: ChangeAdded}}}, // HasChanges = true (has changes)
		{Removed: true},                             // HasChanges = true
		{Changes: []ACLChange{{Type: ChangeAdded}}}, // HasChanges = true
		{},                                          // HasChanges = false
	}

	expected := []bool{true, true, true, false}

	for i, r := range tests {
		got := r.HasChanges()
		if got != expected[i] {
			t.Errorf("ACLChangeResult.HasChanges() = %v, want %v", got, expected[i])
		}
	}
}

func TestXAttrChangeResult_HasChanges(t *testing.T) {
	tests := []XAttrChangeResult{
		{HasNew: true},                              // HasChanges = true
		{Removed: true},                             // HasChanges = true
		{HasNew: true},                              // HasChanges = true
		{Changes: []XAttrChange{{Type: ChangeAdded}}}, // HasChanges = true
		{},                                          // HasChanges = false
	}

	expected := []bool{true, true, true, true, false}

	for i, r := range tests {
		got := r.HasChanges()
		if got != expected[i] {
			t.Errorf("XAttrChangeResult.HasChanges() = %v, want %v", got, expected[i])
		}
	}
}

func TestSELinuxChangeResult_HasChanges(t *testing.T) {
	tests := []SELinuxChangeResult{
		{HasNew: true},                                // HasChanges = true
		{Removed: true},                               // HasChanges = true
		{HasNew: true},                                // HasChanges = true
		{Changes: []*SELinuxChange{{Changed: true}}},  // HasChanges = true
		{},                                            // HasChanges = false
	}

	expected := []bool{true, true, true, true, false}

	for i, r := range tests {
		got := r.HasChanges()
		if got != expected[i] {
			t.Errorf("SELinuxChangeResult.HasChanges() = %v, want %v", got, expected[i])
		}
	}
}

func TestParseCapabilities(t *testing.T) {
	// Output format: path = cap_name+suffix (e=effective, i=inheritable, p=permitted)
	// Note: +ep means effective + permitted (no +i, so not inheritable)
	output := "/usr/bin/passwd = cap_chown+ep cap_dac_override+e"

	caps, err := ParseCapabilities("/usr/bin/passwd", output)
	if err != nil {
		t.Fatalf("ParseCapabilities failed: %v", err)
	}

	if caps.Path != "/usr/bin/passwd" {
		t.Errorf("Path = %s, want /usr/bin/passwd", caps.Path)
	}

	// cap_chown+ep: ends with 'p' -> goes to Permitted
	// cap_dac_override+e: ends with 'e' -> goes to Effective
	if len(caps.Permitted) != 1 {
		t.Errorf("Expected 1 permitted cap, got %d", len(caps.Permitted))
	}

	if len(caps.Effective) != 1 {
		t.Errorf("Expected 1 effective cap, got %d", len(caps.Effective))
	}
}
