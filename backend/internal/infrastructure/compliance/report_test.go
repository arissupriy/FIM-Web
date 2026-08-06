package compliance

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHashData(t *testing.T) {
	hash1 := HashData("test")
	hash2 := HashData("test")
	hash3 := HashData("different")

	if hash1 != hash2 {
		t.Error("Same input should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("Different input should produce different hash")
	}

	if len(hash1) != 64 { // SHA256 produces 64 hex chars
		t.Errorf("Expected 64 char hash, got %d", len(hash1))
	}
}

func TestHashBytes(t *testing.T) {
	data := []byte("test")
	hash := HashBytes(data)

	if len(hash) != 64 {
		t.Errorf("Expected 64 char hash, got %d", len(hash))
	}

	// Same bytes should produce same hash
	hash2 := HashBytes(data)
	if hash != hash2 {
		t.Error("Same input should produce same hash")
	}
}

func TestNewHashChain(t *testing.T) {
	hc := NewHashChain()

	if hc.Length != 0 {
		t.Errorf("Expected length 0, got %d", hc.Length)
	}

	if hc.Head != "" {
		t.Error("Expected empty head")
	}

	if !hc.Verified {
		t.Error("New chain should be verified")
	}
}

func TestHashChain_Append(t *testing.T) {
	hc := NewHashChain()

	err := hc.Append(time.Now(), "event1")
	if err != nil {
		t.Errorf("Append failed: %v", err)
	}

	if hc.Length != 1 {
		t.Errorf("Expected length 1, got %d", hc.Length)
	}

	if hc.Head == "" {
		t.Error("Head should not be empty after append")
	}

	// Append more
	hc.Append(time.Now(), "event2")
	hc.Append(time.Now(), "event3")

	if hc.Length != 3 {
		t.Errorf("Expected length 3, got %d", hc.Length)
	}
}

func TestHashChain_Verify(t *testing.T) {
	hc := NewHashChain()

	// Add entries
	hc.Append(time.Now(), "event1")
	hc.Append(time.Now(), "event2")
	hc.Append(time.Now(), "event3")

	// Verify should pass
	if !hc.Verify() {
		t.Error("Verify should pass for valid chain")
	}

	if !hc.Verified {
		t.Error("Chain should be marked as verified")
	}
}

func TestHashChain_Verify_Tampered(t *testing.T) {
	hc := NewHashChain()

	hc.Append(time.Now(), "event1")
	hc.Append(time.Now(), "event2")

	// Tamper with entry
	hc.Entries[0].DataHash = "tampered"

	// Verify should fail
	if hc.Verify() {
		t.Error("Verify should fail for tampered chain")
	}

	if hc.Verified {
		t.Error("Chain should not be verified after tampering")
	}
}

func TestGenerateReportID(t *testing.T) {
	id1 := GenerateReportID()
	id2 := GenerateReportID()

	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	if len(id1) < 10 {
		t.Error("ID should be reasonably long")
	}
}

func TestGenerateFindingID(t *testing.T) {
	id := GenerateFindingID()

	if len(id) < 10 {
		t.Error("Finding ID should be reasonably long")
	}
}

func TestReport_ToJSON(t *testing.T) {
	hc := NewHashChain()
	hc.Append(time.Now(), "test")

	report := &Report{
		ID:          "TEST-001",
		Type:        ReportTypeSOC2,
		GeneratedAt: time.Now(),
		Period: Period{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
		Summary: ReportSummary{
			TotalEvents: 100,
			ByType:      map[string]int64{"modified": 50, "created": 50},
		},
		HashChain: *hc,
	}

	data, err := report.ToJSON()
	if err != nil {
		t.Errorf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON should not be empty")
	}

	// Verify it's valid JSON
	var parsed Report
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Invalid JSON: %v", err)
	}

	if parsed.ID != "TEST-001" {
		t.Errorf("Expected ID TEST-001, got %s", parsed.ID)
	}
}

func TestReport_ToCSV(t *testing.T) {
	events := []Event{
		{
			ID:         "E001",
			Timestamp:  time.Now(),
			EventType:  "modified",
			RiskLevel:  "HIGH",
			Hash:       "abc123",
			Actor:      &ActorInfo{Username: "user1"},
			Target:     &TargetInfo{Path: "/etc/passwd"},
		},
		{
			ID:         "E002",
			Timestamp:  time.Now(),
			EventType:  "created",
			RiskLevel:  "LOW",
			Hash:       "def456",
			Actor:      &ActorInfo{Username: "user2"},
			Target:     &TargetInfo{Path: "/var/log/app.log"},
		},
	}

	report := &Report{}
	data, err := report.ToCSV(events)
	if err != nil {
		t.Errorf("ToCSV failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("CSV should not be empty")
	}
}
