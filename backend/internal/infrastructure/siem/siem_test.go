package siem

import (
	"context"
	"testing"
	"time"
)

func TestCreateClient(t *testing.T) {
	tests := []struct {
		backend string
		config  *Config
		wantErr bool
	}{
		{"elasticsearch", &Config{Endpoint: "http://localhost:9200"}, false},
		{"splunk", &Config{Endpoint: "http://localhost:8088"}, false},
		{"syslog", &Config{Endpoint: "localhost"}, false},
		{"unknown", &Config{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			client, err := CreateClient(tt.backend, tt.config)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check client is created (name may vary by backend)
			if client.Name() == "" {
				t.Error("Client name should not be empty")
			}
		})
	}
}

func TestMockClient(t *testing.T) {
	client := NewMockClient()

	events := []Event{
		{Source: "test", EventType: "test_event"},
	}

	err := client.Send(context.Background(), events)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(client.GetSentEvents()) != 1 {
		t.Errorf("Expected 1 sent event, got %d", len(client.GetSentEvents()))
	}
}

func TestMockClient_Fail(t *testing.T) {
	client := NewMockClient()
	client.ShouldFail = true

	events := []Event{{Source: "test"}}

	err := client.Send(context.Background(), events)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestValidateEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   *Event
		wantErr bool
	}{
		{
			"valid",
			&Event{Source: "test", EventType: "event"},
			false,
		},
		{
			"nil",
			nil,
			true,
		},
		{
			"empty_source",
			&Event{Source: "", EventType: "event"},
			true,
		},
		{
			"empty_type",
			&Event{Source: "test", EventType: ""},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvent(tt.event)
			if tt.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestBatchEvents(t *testing.T) {
	events := make([]Event, 205)
	for i := range events {
		events[i] = Event{Source: "test"}
	}

	batches := BatchEvents(events, 100)
	if len(batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(batches))
	}

	if len(batches[0]) != 100 {
		t.Errorf("Expected 100 in first batch, got %d", len(batches[0]))
	}

	if len(batches[2]) != 5 {
		t.Errorf("Expected 5 in last batch, got %d", len(batches[2]))
	}
}

func TestBatchEvents_DefaultSize(t *testing.T) {
	events := make([]Event, 150)
	batches := BatchEvents(events, 0)
	if len(batches) != 2 {
		t.Errorf("Expected 2 batches with default size, got %d", len(batches))
	}
}

func TestSerializeEvents(t *testing.T) {
	events := []Event{
		{
			Timestamp: time.Now(),
			Source:    "test",
			EventType: "test_event",
		},
	}

	data, err := SerializeEvents(events)
	if err != nil {
		t.Fatalf("SerializeEvents failed: %v", err)
	}

	parsed, err := DeserializeEvents(data)
	if err != nil {
		t.Fatalf("DeserializeEvents failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Errorf("Expected 1 event, got %d", len(parsed))
	}

	if parsed[0].Source != "test" {
		t.Errorf("Expected source 'test', got '%s'", parsed[0].Source)
	}
}

func TestDeserializeEvents_Invalid(t *testing.T) {
	_, err := DeserializeEvents([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestCreateFIMEvent(t *testing.T) {
	actor := &ActorInfo{
		UserID:   "1000",
		Username: "www-data",
	}
	target := &TargetInfo{
		Path:    "/etc/passwd",
		FileType: "config",
	}
	changes := []ChangeInfo{
		{Field: "hash", OldValue: "abc", NewValue: "def"},
	}

	event := CreateFIMEvent("modified", "HIGH", "fsnotify", actor, target, changes)

	if event.Source != "fsnotify" {
		t.Errorf("Expected source 'fsnotify', got '%s'", event.Source)
	}

	if event.SourceType != "FIM" {
		t.Errorf("Expected source_type 'FIM', got '%s'", event.SourceType)
	}

	if event.RiskLevel != "HIGH" {
		t.Errorf("Expected risk_level 'HIGH', got '%s'", event.RiskLevel)
	}

	if event.Actor == nil || event.Actor.UserID != "1000" {
		t.Error("Actor not set correctly")
	}

	if event.Target == nil || event.Target.Path != "/etc/passwd" {
		t.Error("Target not set correctly")
	}

	if len(event.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(event.Changes))
	}
}

func TestCreateAuditEvent(t *testing.T) {
	actor := &ActorInfo{
		UserID:   "0",
		Username: "root",
	}
	rawData := map[string]interface{}{
		"syscall": 59,
		"exit":    0,
	}

	event := CreateAuditEvent("execve", "auditd", actor, nil, rawData)

	if event.SourceType != "AUDIT" {
		t.Errorf("Expected source_type 'AUDIT', got '%s'", event.SourceType)
	}

	if event.RiskLevel != "INFO" {
		t.Errorf("Expected risk_level 'INFO', got '%s'", event.RiskLevel)
	}

	if event.RawData == nil {
		t.Error("RawData should not be nil")
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	client1 := NewMockClient()
	client2 := NewMockClient()

	registry.Register("test1", client1)
	registry.Register("test2", client2)

	names := registry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 registered clients, got %d", len(names))
	}

	// Get existing
	got, ok := registry.Get("test1")
	if !ok {
		t.Error("Expected to get test1")
	}
	if got.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", got.Name())
	}

	// Get non-existing
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Should not find nonexistent client")
	}

	// Unregister
	registry.Unregister("test1")
	names = registry.List()
	if len(names) != 1 {
		t.Errorf("Expected 1 client after unregister, got %d", len(names))
	}
}

func TestRegistry_TestAll(t *testing.T) {
	registry := NewRegistry()

	registry.Register("ok", NewMockClient())
	registry.Register("fail", &MockClient{ShouldFail: true})

	results := registry.TestAll(context.Background())

	if results["ok"] != nil {
		t.Error("Expected ok test to pass")
	}

	if results["fail"] == nil {
		t.Error("Expected fail test to fail")
	}
}

func TestQueue(t *testing.T) {
	queue := NewQueue(10)

	// Test empty
	if !queue.IsEmpty() {
		t.Error("Expected queue to be empty")
	}

	// Enqueue
	for i := 0; i < 5; i++ {
		err := queue.Enqueue(Event{Source: "test"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	if queue.Len() != 5 {
		t.Errorf("Expected length 5, got %d", queue.Len())
	}

	if queue.IsFull() {
		t.Error("Queue should not be full yet")
	}

	// Fill queue
	for i := 0; i < 5; i++ {
		err := queue.Enqueue(Event{Source: "test"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	if !queue.IsFull() {
		t.Error("Queue should be full")
	}

	// Test full
	err := queue.Enqueue(Event{Source: "test"})
	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Dequeue
	event, err := queue.Dequeue()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if event.Source != "test" {
		t.Errorf("Expected source 'test', got '%s'", event.Source)
	}

	if queue.Len() != 9 {
		t.Errorf("Expected length 9, got %d", queue.Len())
	}

	// Clear
	queue.Clear()
	if !queue.IsEmpty() {
		t.Error("Queue should be empty after clear")
	}

	// Test empty dequeue
	_, err = queue.Dequeue()
	if err != ErrQueueEmpty {
		t.Errorf("Expected ErrQueueEmpty, got %v", err)
	}
}

func TestQueue_DequeueBatch(t *testing.T) {
	queue := NewQueue(10)

	for i := 0; i < 5; i++ {
		queue.Enqueue(Event{Source: "test"})
	}

	batch, err := queue.DequeueBatch(2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(batch) != 2 {
		t.Errorf("Expected batch of 2, got %d", len(batch))
	}

	if queue.Len() != 3 {
		t.Errorf("Expected length 3, got %d", queue.Len())
	}
}

func TestBuffer(t *testing.T) {
	buffer := NewBuffer(&BufferConfig{
		Size:    3,
		Timeout: time.Hour, // Don't auto-flush
		FlushFn: func(events []Event) error {
			return nil
		},
	})

	// Add events
	for i := 0; i < 2; i++ {
		buffer.Add(Event{Source: "test"})
	}

	// Buffer should have 2 events
	if buffer.Len() != 2 {
		t.Errorf("Expected 2 events in buffer, got %d", buffer.Len())
	}

	// Add one more to trigger auto-flush
	buffer.Add(Event{Source: "test3"})

	// Manual flush
	buffer.Flush()

	// Buffer should be empty after flush
	if buffer.Len() != 0 {
		t.Errorf("Expected 0 events after flush, got %d", buffer.Len())
	}

	buffer.Close()
}

func TestBuffer_AddBatch(t *testing.T) {
	buffer := NewBuffer(&BufferConfig{
		Size:    10, // Large enough to not auto-flush
		Timeout: time.Hour,
		FlushFn: func(events []Event) error {
			return nil
		},
	})

	buffer.AddBatch([]Event{
		{Source: "batch1"},
		{Source: "batch2"},
		{Source: "batch3"},
	})

	if buffer.Len() != 3 {
		t.Errorf("Expected 3 events in buffer, got %d", buffer.Len())
	}

	buffer.Close()
}

func TestSyslogChannel_Name(t *testing.T) {
	channel := NewSyslogChannel(DefaultConfig())
	if channel.Name() != "syslog" {
		t.Errorf("Expected 'syslog', got '%s'", channel.Name())
	}
}

func TestElasticsearchClient_Name(t *testing.T) {
	client := NewElasticsearchClient(DefaultConfig())
	if client.Name() != "elasticsearch" {
		t.Errorf("Expected 'elasticsearch', got '%s'", client.Name())
	}
}

func TestSplunkHECClient_Name(t *testing.T) {
	client := NewSplunkHECClient(DefaultConfig())
	if client.Name() != "splunk-hec" {
		t.Errorf("Expected 'splunk-hec', got '%s'", client.Name())
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected 30s timeout, got %v", config.Timeout)
	}
	if config.BufferSize != 100 {
		t.Errorf("Expected buffer size 100, got %d", config.BufferSize)
	}
	if config.RetryCount != 3 {
		t.Errorf("Expected retry count 3, got %d", config.RetryCount)
	}
}

func TestDefaultSIEMConfig(t *testing.T) {
	config := DefaultSIEMConfig()
	if !config.Enabled {
		t.Error("Expected SIEM to be enabled")
	}
	if config.BufferSize != 100 {
		t.Errorf("Expected buffer size 100, got %d", config.BufferSize)
	}
	if len(config.Backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(config.Backends))
	}
}

func TestSIEMService(t *testing.T) {
	config := DefaultSIEMConfig()
	service := NewSIEMService(config)

	// Initialize
	if err := service.Initialize(); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// Get config
	gotConfig := service.GetConfig()
	if gotConfig != config {
		t.Error("GetConfig should return original config")
	}

	// List backends
	names := service.registry.List()
	if len(names) < 1 {
		t.Error("Expected at least 1 registered backend")
	}
}

func TestQueueError(t *testing.T) {
	err := &QueueError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%s'", err.Error())
	}
}
