package audit

import (
	"fmt"
	"os/user"
	"strconv"
	"time"
)

// Attributor provides actor attribution from audit events.
type Attributor struct {
	uidCache map[uint32]*user.User
	gidCache map[uint32]*user.Group
}

// NewAttributor creates a new attribution service.
func NewAttributor() *Attributor {
	return &Attributor{
		uidCache: make(map[uint32]*user.User),
		gidCache: make(map[uint32]*user.Group),
	}
}

// Attribution represents actor attribution details.
type Attribution struct {
	UserID     uint32
	Username   string
	LoginUID   uint32
	LoginName  string
	GroupID    uint32
	GroupName  string
	ProcessID  uint32
	ParentPID  uint32
	SessionID  uint64
	TTY        string
	HostName   string
	RemoteAddr string
	Shell      string
	HomeDir    string
	Timestamp  time.Time
	Source     string // pam, audit, logind, etc.
}

// LookupUser looks up a user by UID.
func (a *Attributor) LookupUser(uid uint32) (*user.User, error) {
	if u, ok := a.uidCache[uid]; ok {
		return u, nil
	}

	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err == nil {
		a.uidCache[uid] = u
	}
	return u, err
}

// LookupGroup looks up a group by GID.
func (a *Attributor) LookupGroup(gid uint32) (*user.Group, error) {
	if g, ok := a.gidCache[gid]; ok {
		return g, nil
	}

	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err == nil {
		a.gidCache[gid] = g
	}
	return g, err
}

// LookupUsername looks up username for a UID, returns fallback if not found.
func (a *Attributor) LookupUsername(uid uint32) string {
	u, err := a.LookupUser(uid)
	if err != nil {
		return fmt.Sprintf("uid-%d", uid)
	}
	return u.Username
}

// LookupGroupname looks up group name for a GID, returns fallback if not found.
func (a *Attributor) LookupGroupname(gid uint32) string {
	g, err := a.LookupGroup(gid)
	if err != nil {
		return fmt.Sprintf("gid-%d", gid)
	}
	return g.Name
}

// AttributeEvent extracts attribution from an audit event.
func (a *Attributor) AttributeEvent(e *Event) *Attribution {
	if e == nil {
		return nil
	}

	attr := &Attribution{
		UserID:     e.UserID,
		LoginUID:   e.LoginUID,
		GroupID:    e.GID,
		ProcessID:  e.ProcessID,
		ParentPID:  e.ParentPID,
		SessionID:  e.SessionID,
		TTY:        e.TTY,
		HostName:   e.HostName,
		RemoteAddr: e.Addr,
		Timestamp:  e.Timestamp,
		Source:     "audit",
	}

	// Look up usernames
	if e.UserID > 0 {
		attr.Username = a.LookupUsername(e.UserID)
	}
	if e.LoginUID > 0 {
		attr.LoginName = a.LookupUsername(e.LoginUID)
	}
	if e.GID > 0 {
		attr.GroupName = a.LookupGroupname(e.GID)
	}

	return attr
}

// AttributeFIMEvent extracts attribution for a FIM event from audit logs.
func (a *Attributor) AttributeFIMEvent(fimEvent *FIMEvent, auditEvents []*Event) *Attribution {
	if fimEvent == nil {
		return nil
	}

	// Find matching audit events
	var matching []*Event

	for _, e := range auditEvents {
		// Match by PID
		if fimEvent.ProcessID > 0 && e.ProcessID == fimEvent.ProcessID {
			matching = append(matching, e)
			continue
		}

		// Match by path
		if fimEvent.Path != "" && e.Path == fimEvent.Path {
			matching = append(matching, e)
			continue
		}
	}

	if len(matching) == 0 {
		return nil
	}

	// Use the first match or aggregate
	return a.AttributeEvent(matching[0])
}

// AggregateAttribution aggregates attribution from multiple events.
func (a *Attributor) AggregateAttribution(events []*Event) *Attribution {
	if len(events) == 0 {
		return nil
	}

	// Use the first event as base
	attr := a.AttributeEvent(events[0])

	// If there are more events, try to fill in missing info
	for _, e := range events[1:] {
		if attr.UserID == 0 && e.UserID > 0 {
			attr.UserID = e.UserID
			attr.Username = a.LookupUsername(e.UserID)
		}
		if attr.LoginUID == 0 && e.LoginUID > 0 {
			attr.LoginUID = e.LoginUID
			attr.LoginName = a.LookupUsername(e.LoginUID)
		}
		if attr.ProcessID == 0 && e.ProcessID > 0 {
			attr.ProcessID = e.ProcessID
		}
		if attr.SessionID == 0 && e.SessionID > 0 {
			attr.SessionID = e.SessionID
		}
	}

	return attr
}

// SessionInfo contains information about a user session.
type SessionInfo struct {
	SessionID   uint64
	UserID      uint32
	Username    string
	LoginTime   time.Time
	TTY         string
	HostName    string
	RemoteAddr  string
	ProcessIDs  []uint32
	PIDToPath   map[uint32][]string // PID to files accessed
	IsActive    bool
}

// ParseLoginEvents extracts session information from LOGIN events.
func (a *Attributor) ParseLoginEvents(events []*Event) []*SessionInfo {
	sessions := make(map[uint64]*SessionInfo)

	for _, e := range events {
		if e.Type != "LOGIN" {
			continue
		}

		si := &SessionInfo{
			SessionID:  e.SessionID,
			UserID:     e.LoginUID,
			Username:   a.LookupUsername(e.LoginUID),
			LoginTime:  e.Timestamp,
			TTY:        e.TTY,
			HostName:   e.HostName,
			RemoteAddr: e.Addr,
			PIDToPath:  make(map[uint32][]string),
		}

		sessions[e.SessionID] = si
	}

	// Build PID lists from SYSCALL events
	for _, e := range events {
		if e.Type != "SYSCALL" || e.SessionID == 0 {
			continue
		}

		if si, ok := sessions[e.SessionID]; ok {
			// Add PID if not already present
			found := false
			for _, pid := range si.ProcessIDs {
				if pid == e.ProcessID {
					found = true
					break
				}
			}
			if !found {
				si.ProcessIDs = append(si.ProcessIDs, e.ProcessID)
			}

			// Track path access
			if e.Path != "" {
				si.PIDToPath[e.ProcessID] = append(si.PIDToPath[e.ProcessID], e.Path)
			}
		}
	}

	result := make([]*SessionInfo, 0, len(sessions))
	for _, si := range sessions {
		result = append(result, si)
	}

	return result
}

// GetProcessChain returns the process chain (ancestry) for a PID.
func (a *Attributor) GetProcessChain(pid uint32, auditEvents []*Event) []*Event {
	// Build PID -> ParentPID mapping
	pidMap := make(map[uint32]*Event)

	for _, e := range auditEvents {
		if e.ProcessID > 0 {
			pidMap[e.ProcessID] = e
		}
	}

	// Walk up the tree
	var chain []*Event
	current := pid

	for i := 0; i < 100; i++ { // Max depth
		if e, ok := pidMap[current]; ok {
			chain = append(chain, e)
			current = e.ParentPID
			if current == 0 {
				break
			}
		} else {
			break
		}
	}

	return chain
}

// WhoWasAtPath returns who was responsible for accessing a path.
func (a *Attributor) WhoWasAtPath(path string, auditEvents []*Event) *Attribution {
	// Filter events by path
	var matching []*Event
	for _, e := range auditEvents {
		if e.Path == path {
			matching = append(matching, e)
		}
	}

	if len(matching) == 0 {
		return nil
	}

	return a.AggregateAttribution(matching)
}

// WhoModifiedFile returns who modified a file based on audit events.
func (a *Attributor) WhoModifiedFile(path string, auditEvents []*Event) *Attribution {
	var relevant []*Event

	for _, e := range auditEvents {
		// Check PATH events for the specific file
		if e.Type == "PATH" && e.Path == path {
			relevant = append(relevant, e)
		}
	}

	if len(relevant) == 0 {
		return a.WhoWasAtPath(path, auditEvents)
	}

	return a.AggregateAttribution(relevant)
}

// FormatAttribution formats attribution as a human-readable string.
func FormatAttribution(attr *Attribution) string {
	if attr == nil {
		return "unknown"
	}

	parts := []string{}

	if attr.Username != "" {
		parts = append(parts, attr.Username)
	} else if attr.UserID > 0 {
		parts = append(parts, fmt.Sprintf("uid-%d", attr.UserID))
	}

	if attr.SessionID > 0 {
		parts = append(parts, fmt.Sprintf("(session %d)", attr.SessionID))
	}

	if attr.TTY != "" {
		parts = append(parts, fmt.Sprintf("on %s", attr.TTY))
	}

	if attr.RemoteAddr != "" {
		parts = append(parts, fmt.Sprintf("from %s", attr.RemoteAddr))
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return parts[0] + " " + joinStrings(parts[1:], " ")
}

// FormatActor formats an actor as a human-readable string.
func FormatActor(actor *Actor) string {
	if actor == nil {
		return "unknown"
	}

	parts := []string{}

	if actor.Username != "" {
		parts = append(parts, actor.Username)
	} else if actor.UserID > 0 {
		parts = append(parts, fmt.Sprintf("uid-%d", actor.UserID))
	}

	if actor.ProcessName != "" {
		parts = append(parts, fmt.Sprintf("via %s", actor.ProcessName))
	}

	if actor.TTY != "" {
		parts = append(parts, fmt.Sprintf("on %s", actor.TTY))
	}

	if actor.HostName != "" {
		parts = append(parts, fmt.Sprintf("from %s", actor.HostName))
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return parts[0]
}

// joinStrings joins strings with separator.
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

// GetLastLogin returns the most recent login for a user.
func (a *Attributor) GetLastLogin(uid uint32, auditEvents []*Event) *SessionInfo {
	var lastLogin *SessionInfo

	for _, e := range auditEvents {
		if e.Type == "LOGIN" && e.LoginUID == uid {
			if lastLogin == nil || e.Timestamp.After(lastLogin.LoginTime) {
				lastLogin = &SessionInfo{
					SessionID:  e.SessionID,
					UserID:     e.LoginUID,
					Username:   a.LookupUsername(uid),
					LoginTime:  e.Timestamp,
					TTY:        e.TTY,
					HostName:   e.HostName,
					RemoteAddr: e.Addr,
				}
			}
		}
	}

	return lastLogin
}

// IsPrivileged checks if the attribution represents a privileged user.
func (a *Attributor) IsPrivileged(attr *Attribution) bool {
	if attr == nil {
		return false
	}

	// UID 0 is root
	if attr.UserID == 0 {
		return true
	}

	// Check if user is in wheel or sudo group
	// This would require looking up the user's groups
	return false
}
