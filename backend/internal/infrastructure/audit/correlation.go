package audit

import (
	"fmt"
	"strconv"
	"time"
)

// FIMEvent represents a file integrity monitoring event from the scanner.
type FIMEvent struct {
	ID           int64
	ProjectID    int
	Path         string
	EventType    string // created, modified, deleted, permission_changed
	OldHash      string
	NewHash      string
	OldPerm      string
	NewPerm      string
	OldOwner     string
	NewOwner     string
	OldGroup     string
	NewGroup     string
	Timestamp    time.Time
	ProcessID    uint32
	ProcessName  string
	UserID       string
	ActorID      string
	ActorName    string
	Source       string
	AuditCorrelation bool
	RiskLevel    string
	Description  string
}

// CorrelationResult represents the result of correlating FIM and audit events.
type CorrelationResult struct {
	FIMEvent      *FIMEvent
	AuditEvents   []*Event
	Correlated    bool
	Actor         *Actor
	Confidence    float64
	MatchedBy     MatchType
	Timestamp     time.Time
}

// Actor represents the actor responsible for an event.
type Actor struct {
	UserID       uint32
	Username     string
	LoginUID     uint32
	ProcessID    uint32
	ProcessName  string
	ParentPID    uint32
	SessionID    uint64
	Command      string
	TTY          string
	HostName     string
}

// MatchType indicates how the correlation was made.
type MatchType string

const (
	MatchByPID      MatchType = "pid"       // Matched by process ID
	MatchByPath     MatchType = "path"      // Matched by file path
	MatchByTime     MatchType = "time"      // Matched by timestamp
	MatchByInode    MatchType = "inode"     // Matched by inode
	MatchBySession  MatchType = "session"   // Matched by session ID
)

// Correlator handles FIM and audit event correlation.
type Correlator struct {
	// Lookup windows
	PIDWindow     time.Duration // Time window to look for related PID events
	TimeWindow    time.Duration // Time window for timestamp correlation
	SessionWindow time.Duration // Time window for session correlation

	// Username cache (optional)
	usernameCache map[uint32]string
}

// NewCorrelator creates a new FIM-Audit correlator.
func NewCorrelator() *Correlator {
	return &Correlator{
		PIDWindow:     5 * time.Second,
		TimeWindow:    2 * time.Second,
		SessionWindow: 10 * time.Minute,
		usernameCache: make(map[uint32]string),
	}
}

// SetUsernameCache sets a cache of user IDs to usernames.
func (c *Correlator) SetUsernameCache(cache map[uint32]string) {
	c.usernameCache = cache
}

// GetUsername returns the username for a user ID.
func (c *Correlator) GetUsername(uid uint32) string {
	if name, ok := c.usernameCache[uid]; ok {
		return name
	}
	return fmt.Sprintf("uid-%d", uid)
}

// CorrelateEvent correlates a FIM event with audit events.
func (c *Correlator) CorrelateEvent(fimEvent *FIMEvent, auditEvents []*Event) *CorrelationResult {
	result := &CorrelationResult{
		FIMEvent:  fimEvent,
		AuditEvents: make([]*Event, 0),
		Timestamp: time.Now(),
	}

	// Try different correlation methods
	if c.correlateByPID(fimEvent, auditEvents, result) {
		result.MatchedBy = MatchByPID
		result.Correlated = true
		result.Confidence = 0.95
		return result
	}

	if c.correlateByPath(fimEvent, auditEvents, result) {
		result.MatchedBy = MatchByPath
		result.Correlated = true
		result.Confidence = 0.85
		return result
	}

	if c.correlateByTime(fimEvent, auditEvents, result) {
		result.MatchedBy = MatchByTime
		result.Correlated = true
		result.Confidence = 0.6
		return result
	}

	return result
}

// correlateByPID tries to find audit events by process ID.
func (c *Correlator) correlateByPID(fimEvent *FIMEvent, auditEvents []*Event, result *CorrelationResult) bool {
	if fimEvent.ProcessID == 0 {
		return false
	}

	var matched []*Event
	for _, ae := range auditEvents {
		// Match by PID
		if ae.ProcessID == fimEvent.ProcessID {
			matched = append(matched, ae)
			continue
		}

		// Also check if this PID was mentioned in any audit event
		// This handles cases where we have parent process info
		if ae.ParentPID == fimEvent.ProcessID {
			matched = append(matched, ae)
		}
	}

	if len(matched) > 0 {
		result.AuditEvents = matched
		result.Actor = c.extractActor(matched)
		return true
	}

	return false
}

// correlateByPath tries to find audit events by file path.
func (c *Correlator) correlateByPath(fimEvent *FIMEvent, auditEvents []*Event, result *CorrelationResult) bool {
	if fimEvent.Path == "" {
		return false
	}

	var matched []*Event
	for _, ae := range auditEvents {
		if ae.Path == fimEvent.Path {
			matched = append(matched, ae)
		}
	}

	if len(matched) > 0 {
		result.AuditEvents = matched
		result.Actor = c.extractActor(matched)
		return true
	}

	return false
}

// correlateByTime tries to find audit events by timestamp proximity.
func (c *Correlator) correlateByTime(fimEvent *FIMEvent, auditEvents []*Event, result *CorrelationResult) bool {
	if fimEvent.Timestamp.IsZero() {
		return false
	}

	var matched []*Event
	for _, ae := range auditEvents {
		// Check if audit event is within time window
		diff := ae.Timestamp.Sub(fimEvent.Timestamp)
		if diff < 0 {
			diff = -diff
		}

		if diff <= c.TimeWindow {
			matched = append(matched, ae)
		}
	}

	if len(matched) > 0 {
		result.AuditEvents = matched
		result.Actor = c.extractActor(matched)
		return true
	}

	return false
}

// extractActor extracts actor information from audit events.
func (c *Correlator) extractActor(events []*Event) *Actor {
	if len(events) == 0 {
		return nil
	}

	actor := &Actor{}

	for _, e := range events {
		if e.UserID > 0 && actor.UserID == 0 {
			actor.UserID = e.UserID
			actor.Username = c.GetUsername(e.UserID)
		}
		if e.LoginUID > 0 && actor.LoginUID == 0 {
			actor.LoginUID = e.LoginUID
		}
		if e.ProcessID > 0 && actor.ProcessID == 0 {
			actor.ProcessID = e.ProcessID
		}
		if e.ProcessName != "" && actor.ProcessName == "" {
			actor.ProcessName = e.ProcessName
		}
		if e.ParentPID > 0 && actor.ParentPID == 0 {
			actor.ParentPID = e.ParentPID
		}
		if e.SessionID > 0 && actor.SessionID == 0 {
			actor.SessionID = e.SessionID
		}
		if e.Comm != "" && actor.Command == "" {
			actor.Command = e.Comm
		}
		if e.TTY != "" && actor.TTY == "" {
			actor.TTY = e.TTY
		}
		if e.HostName != "" && actor.HostName == "" {
			actor.HostName = e.HostName
		}
	}

	return actor
}

// CorrelateBatch correlates multiple FIM events with audit events.
func (c *Correlator) CorrelateBatch(fimEvents []*FIMEvent, auditEvents []*Event) []*CorrelationResult {
	results := make([]*CorrelationResult, 0, len(fimEvents))

	for _, fe := range fimEvents {
		result := c.CorrelateEvent(fe, auditEvents)
		results = append(results, result)
	}

	return results
}

// BuildPIDIndex builds an index of audit events by PID.
func BuildPIDIndex(events []*Event) map[uint32][]*Event {
	index := make(map[uint32][]*Event)

	for _, e := range events {
		if e.ProcessID > 0 {
			index[e.ProcessID] = append(index[e.ProcessID], e)
		}
	}

	return index
}

// BuildPathIndex builds an index of audit events by path.
func BuildPathIndex(events []*Event) map[string][]*Event {
	index := make(map[string][]*Event)

	for _, e := range events {
		if e.Path != "" {
			index[e.Path] = append(index[e.Path], e)
		}
	}

	return index
}

// BuildUserIndex builds an index of audit events by user ID.
func BuildUserIndex(events []*Event) map[uint32][]*Event {
	index := make(map[uint32][]*Event)

	for _, e := range events {
		if e.UserID > 0 {
			index[e.UserID] = append(index[e.UserID], e)
		}
		if e.LoginUID > 0 {
			index[e.LoginUID] = append(index[e.LoginUID], e)
		}
	}

	return index
}

// BuildSessionIndex builds an index of audit events by session ID.
func BuildSessionIndex(events []*Event) map[uint64][]*Event {
	index := make(map[uint64][]*Event)

	for _, e := range events {
		if e.SessionID > 0 {
			index[e.SessionID] = append(index[e.SessionID], e)
		}
	}

	return index
}

// IndexedCorrelator provides fast lookups using pre-built indexes.
type IndexedCorrelator struct {
	Correlator
	PIDIndex     map[uint32][]*Event
	PathIndex    map[string][]*Event
	UserIndex    map[uint32][]*Event
	SessionIndex map[uint64][]*Event
}

// NewIndexedCorrelator creates a correlator with pre-built indexes.
func NewIndexedCorrelator(events []*Event) *IndexedCorrelator {
	return &IndexedCorrelator{
		Correlator:   *NewCorrelator(),
		PIDIndex:     BuildPIDIndex(events),
		PathIndex:    BuildPathIndex(events),
		UserIndex:    BuildUserIndex(events),
		SessionIndex: BuildSessionIndex(events),
	}
}

// CorrelateEventWithIndex correlates using pre-built indexes.
func (ic *IndexedCorrelator) CorrelateEventWithIndex(fimEvent *FIMEvent) *CorrelationResult {
	result := &CorrelationResult{
		FIMEvent:     fimEvent,
		AuditEvents:  make([]*Event, 0),
		Timestamp:    time.Now(),
	}

	// Try PID lookup first (fastest)
	if fimEvent.ProcessID > 0 {
		if events, ok := ic.PIDIndex[fimEvent.ProcessID]; ok {
			result.AuditEvents = append(result.AuditEvents, events...)
			result.Actor = ic.extractActor(events)
			result.Correlated = true
			result.Confidence = 0.95
			result.MatchedBy = MatchByPID
			return result
		}
	}

	// Try path lookup
	if fimEvent.Path != "" {
		if events, ok := ic.PathIndex[fimEvent.Path]; ok {
			result.AuditEvents = append(result.AuditEvents, events...)
			result.Actor = ic.extractActor(events)
			result.Correlated = true
			result.Confidence = 0.85
			result.MatchedBy = MatchByPath
			return result
		}
	}

	// Try user lookup
	if fimEvent.UserID != "" {
		if uid, err := strconv.ParseUint(fimEvent.UserID, 10, 32); err == nil {
			if events, ok := ic.UserIndex[uint32(uid)]; ok {
				result.AuditEvents = append(result.AuditEvents, events...)
				result.Actor = ic.extractActor(events)
				result.Correlated = true
				result.Confidence = 0.7
				result.MatchedBy = MatchByPID
				return result
			}
		}
	}

	return result
}

// EnrichFIMEvent enriches a FIM event with audit correlation data.
func EnrichFIMEvent(fimEvent *FIMEvent, auditEvents []*Event) *FIMEvent {
	correlator := NewCorrelator()
	result := correlator.CorrelateEvent(fimEvent, auditEvents)

	if result.Correlated && result.Actor != nil {
		fimEvent.ActorID = fmt.Sprintf("%d", result.Actor.UserID)
		fimEvent.ActorName = result.Actor.Username
		fimEvent.ProcessID = result.Actor.ProcessID
		fimEvent.ProcessName = result.Actor.Command

		// Set source as auditd
		fimEvent.Source = "auditd"
		fimEvent.AuditCorrelation = true
	}

	return fimEvent
}

// GetActorSummary returns a summary of the actor's activities.
func GetActorSummary(events []*Event, actor *Actor) map[string]interface{} {
	summary := make(map[string]interface{})

	if actor == nil {
		return summary
	}

	summary["user_id"] = actor.UserID
	summary["username"] = actor.Username
	summary["login_uid"] = actor.LoginUID
	summary["process_id"] = actor.ProcessID
	summary["process_name"] = actor.ProcessName
	summary["session_id"] = actor.SessionID

	// Count events by type
	eventCounts := make(map[string]int)
	for _, e := range events {
		eventCounts[e.Type]++
	}
	summary["event_counts"] = eventCounts
	summary["total_events"] = len(events)

	// Find time range
	if len(events) > 0 {
		var earliest, latest time.Time
		for i, e := range events {
			if i == 0 || e.Timestamp.Before(earliest) {
				earliest = e.Timestamp
			}
			if i == 0 || e.Timestamp.After(latest) {
				latest = e.Timestamp
			}
		}
		summary["earliest_event"] = earliest
		summary["latest_event"] = latest
		summary["duration"] = latest.Sub(earliest)
	}

	return summary
}
