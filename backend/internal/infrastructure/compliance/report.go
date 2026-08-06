// Package compliance provides compliance reporting and hash chain verification.
package compliance

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a file integrity monitoring event.
type Event struct {
	ID        string
	Timestamp time.Time
	EventType string
	RiskLevel string
	Hash      string
	Actor     *ActorInfo
	Target    *TargetInfo
}

// ActorInfo represents actor information.
type ActorInfo struct {
	Username string
}

// TargetInfo represents target information.
type TargetInfo struct {
	Path string
}

// ReportType represents the type of compliance report.
type ReportType string

const (
	ReportTypeSOC2    ReportType = "SOC2"
	ReportTypeNIST    ReportType = "NIST"
	ReportTypeSummary ReportType = "SUMMARY"
	ReportTypeEvents  ReportType = "EVENTS"
)

// ReportFormat represents the output format.
type ReportFormat string

const (
	FormatJSON ReportFormat = "json"
	FormatCSV  ReportFormat = "csv"
)

// Report represents a compliance report.
type Report struct {
	ID          string       `json:"id"`
	Type        ReportType   `json:"type"`
	GeneratedAt time.Time    `json:"generated_at"`
	Period      Period      `json:"period"`
	Summary     ReportSummary `json:"summary"`
	Findings    []Finding   `json:"findings,omitempty"`
	Metadata    Metadata    `json:"metadata"`
	HashChain   HashChain   `json:"hash_chain"`
}

// Period represents a time period.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ReportSummary contains summary statistics.
type ReportSummary struct {
	TotalEvents    int64            `json:"total_events"`
	CriticalCount int64            `json:"critical_count"`
	HighCount     int64            `json:"high_count"`
	MediumCount   int64            `json:"medium_count"`
	LowCount      int64            `json:"low_count"`
	ByType        map[string]int64 `json:"by_type"`
	ByUser        map[string]int64 `json:"by_user"`
	ByPath        map[string]int64 `json:"by_path"`
}

// Finding represents a compliance finding.
type Finding struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	EventID     string    `json:"event_id,omitempty"`
	Path        string    `json:"path,omitempty"`
	User        string    `json:"user,omitempty"`
	Controls    []string `json:"controls,omitempty"`
}

// Metadata contains report metadata.
type Metadata struct {
	ProjectID    int       `json:"project_id"`
	ProjectName string     `json:"project_name"`
	Framework   string     `json:"framework"`
	Version     string     `json:"version"`
	GeneratedBy string     `json:"generated_by"`
	ReportHash  string     `json:"report_hash"`
}

// HashChain provides tamper-evident chaining.
type HashChain struct {
	Head      string        `json:"head"`
	Length    int           `json:"length"`
	Entries   []HashEntry   `json:"entries,omitempty"`
	Verified  bool          `json:"verified"`
	VerifiedAt *time.Time   `json:"verified_at,omitempty"`
}

// HashEntry represents a single entry in the hash chain.
type HashEntry struct {
	Index       int       `json:"index"`
	Timestamp   time.Time `json:"timestamp"`
	DataHash    string    `json:"data_hash"`
	PreviousHash string   `json:"previous_hash"`
	Hash        string    `json:"hash"`
	EventData   string    `json:"event_data,omitempty"`
}

// NewHashChain creates a new hash chain.
func NewHashChain() *HashChain {
	return &HashChain{
		Head:    "",
		Length:  0,
		Entries: make([]HashEntry, 0),
		Verified: true,
	}
}

// Append adds an entry to the hash chain.
func (hc *HashChain) Append(timestamp time.Time, eventData string) error {
	entry := HashEntry{
		Index:     hc.Length,
		Timestamp: timestamp,
		DataHash:  HashData(eventData),
		PreviousHash: hc.Head,
	}

	// Hash: SHA256(index + timestamp + dataHash + previousHash)
	content := fmt.Sprintf("%d:%s:%s:%s", entry.Index, timestamp.Format(time.RFC3339Nano), entry.DataHash, entry.PreviousHash)
	entry.Hash = HashData(content)

	hc.Entries = append(hc.Entries, entry)
	hc.Head = entry.Hash
	hc.Length++

	return nil
}

// Verify checks the integrity of the hash chain.
func (hc *HashChain) Verify() bool {
	hc.Verified = true
	hc.VerifiedAt = nil

	if len(hc.Entries) == 0 {
		return true
	}

	var previousHash string
	for i, entry := range hc.Entries {
		if entry.Index != i {
			hc.Verified = false
			return false
		}

		if entry.PreviousHash != previousHash {
			hc.Verified = false
			return false
		}

		// Recalculate hash
		content := fmt.Sprintf("%d:%s:%s:%s", entry.Index, entry.Timestamp.Format(time.RFC3339Nano), entry.DataHash, entry.PreviousHash)
		expectedHash := HashData(content)
		if entry.Hash != expectedHash {
			hc.Verified = false
			return false
		}

		previousHash = entry.Hash
	}

	hc.Head = previousHash
	now := time.Now()
	hc.VerifiedAt = &now
	return true
}

// HashData computes SHA-256 hash of data.
func HashData(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// HashBytes computes SHA-256 hash of bytes.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// GenerateReport creates a compliance report.
func GenerateReport(reportType ReportType, period Period, events []Event) *Report {
	report := &Report{
		ID:          GenerateReportID(),
		Type:        reportType,
		GeneratedAt: time.Now(),
		Period:      period,
		Summary:     ReportSummary{
			ByType: make(map[string]int64),
			ByUser: make(map[string]int64),
			ByPath: make(map[string]int64),
		},
		Findings:  make([]Finding, 0),
		HashChain: *NewHashChain(),
	}

	// Process events
	for _, event := range events {
		report.processEvent(event)
	}

	// Generate findings based on framework
	switch reportType {
	case ReportTypeSOC2:
		report.generateSOC2Findings()
	case ReportTypeNIST:
		report.generateNISTFindings()
	}

	// Calculate report hash
	report.calculateReportHash()

	return report
}

// processEvent processes a single event.
func (r *Report) processEvent(event Event) {
	r.Summary.TotalEvents++

	// Count by severity
	switch event.RiskLevel {
	case "CRITICAL", "HIGH":
		r.Summary.HighCount++
	case "MEDIUM":
		r.Summary.MediumCount++
	case "LOW":
		r.Summary.LowCount++
	default:
		r.Summary.LowCount++
	}

	// Count by type
	r.Summary.ByType[event.EventType]++

	// Count by user
	if event.Actor != nil && event.Actor.Username != "" {
		r.Summary.ByUser[event.Actor.Username]++
	}

	// Count by path
	if event.Target != nil && event.Target.Path != "" {
		r.Summary.ByPath[event.Target.Path]++
	}

	// Add to hash chain
	if err := r.HashChain.Append(event.Timestamp, eventToJSON(event)); err == nil {
		// Entry added
	}
}

// generateSOC2Findings generates SOC2-specific findings.
func (r *Report) generateSOC2Findings() {
	r.Metadata.Framework = "SOC2"

	// CC6.1 - Logical access controls
	if r.Summary.HighCount > 10 {
		r.Findings = append(r.Findings, Finding{
			ID:          GenerateFindingID(),
			Timestamp:   time.Now(),
			Severity:    "HIGH",
			Title:       "CC6.1 - Multiple high-risk access events detected",
			Description: fmt.Sprintf("Found %d high-risk events in the reporting period.", r.Summary.HighCount),
			Controls:    []string{"CC6.1", "CC6.6"},
		})
	}

	// CC7.2 - System operations
	if r.Summary.TotalEvents > 1000 {
		r.Findings = append(r.Findings, Finding{
			ID:          GenerateFindingID(),
			Timestamp:   time.Now(),
			Severity:    "MEDIUM",
			Title:       "CC7.2 - High volume of system events",
			Description: fmt.Sprintf("Found %d total events - may indicate unusual activity.", r.Summary.TotalEvents),
			Controls:    []string{"CC7.2"},
		})
	}
}

// generateNISTFindings generates NIST CSF-specific findings.
func (r *Report) generateNISTFindings() {
	r.Metadata.Framework = "NIST CSF"

	// DE.CM-1 - Network monitoring
	if r.Summary.TotalEvents == 0 {
		r.Findings = append(r.Findings, Finding{
			ID:          GenerateFindingID(),
			Timestamp:   time.Now(),
			Severity:    "LOW",
			Title:       "DE.CM-1 - No events detected",
			Description: "No file integrity events were detected in the reporting period.",
			Controls:    []string{"DE.CM-1"},
		})
	}

	// PR.AC - Identity management
	if countUsersAbove(r.Summary.ByUser, 100) > 5 {
		r.Findings = append(r.Findings, Finding{
			ID:          GenerateFindingID(),
			Timestamp:   time.Now(),
			Severity:    "MEDIUM",
			Title:       "PR.AC - Multiple users with high activity",
			Description: "Several users have generated significant activity.",
			Controls:    []string{"PR.AC-1", "PR.AC-2"},
		})
	}
}

func countUsersAbove(userMap map[string]int64, threshold int64) int {
	count := 0
	for _, v := range userMap {
		if v > threshold {
			count++
		}
	}
	return count
}

// calculateReportHash calculates the report hash.
func (r *Report) calculateReportHash() {
	content := fmt.Sprintf("%s:%s:%s:%d", r.ID, r.Type, r.GeneratedAt.Format(time.RFC3339Nano), r.Summary.TotalEvents)
	r.Metadata.ReportHash = HashData(content)
}

// ToJSON serializes the report to JSON.
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToCSV exports events to CSV format.
func (r *Report) ToCSV(events []Event) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"Timestamp", "EventType", "RiskLevel", "User", "Path", "Hash"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write events
	for _, event := range events {
		row := []string{
			event.Timestamp.Format(time.RFC3339),
			event.EventType,
			event.RiskLevel,
			getActorUsername(event),
			getTargetPath(event),
			event.Hash,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}

func getActorUsername(event Event) string {
	if event.Actor != nil {
		return event.Actor.Username
	}
	return ""
}

func getTargetPath(event Event) string {
	if event.Target != nil {
		return event.Target.Path
	}
	return ""
}

func eventToJSON(event Event) string {
	data, _ := json.Marshal(event)
	return string(data)
}

// GenerateReportID generates a unique report ID.
func GenerateReportID() string {
	return fmt.Sprintf("RPT-%d-%s", time.Now().Unix(), HashData(time.Now().String())[:8])
}

// GenerateFindingID generates a unique finding ID.
func GenerateFindingID() string {
	return fmt.Sprintf("FND-%d-%s", time.Now().Unix(), HashData(time.Now().String())[:8])
}
