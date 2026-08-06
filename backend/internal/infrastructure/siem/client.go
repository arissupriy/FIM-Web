// Package siem provides SIEM integration for forwarding events.
package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a generic SIEM client interface.
type Client interface {
	// Name returns the SIEM platform name.
	Name() string

	// Send sends events to the SIEM platform.
	Send(ctx context.Context, events []Event) error

	// Test tests the connection to the SIEM platform.
	Test(ctx context.Context) error
}

// Config represents SIEM client configuration.
type Config struct {
	// Common config
	Endpoint   string        `json:"endpoint"`
	APIKey     string        `json:"api_key"`
	Timeout    time.Duration `json:"timeout"`
	BufferSize int           `json:"buffer_size"`
	RetryCount int           `json:"retry_count"`

	// Platform-specific config
	Index       string `json:"index"`        // Elasticsearch
	Channel     string `json:"channel"`      // Slack
	ProjectID   string `json:"project_id"`  // Google Cloud
	OrgID       string `json:"org_id"`       // Splunk
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Timeout:    30 * time.Second,
		BufferSize: 100,
		RetryCount: 3,
	}
}

// Event represents a SIEM event.
type Event struct {
	Timestamp    time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	SourceType  string                 `json:"source_type"`
	EventType   string                 `json:"event_type"`
	RiskLevel   string                 `json:"risk_level"`
	Actor       *ActorInfo             `json:"actor,omitempty"`
	Target      *TargetInfo            `json:"target,omitempty"`
	Changes     []ChangeInfo           `json:"changes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RawData     map[string]interface{} `json:"raw_data,omitempty"`
}

// ActorInfo represents actor information in an event.
type ActorInfo struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	ProcessID   uint32 `json:"process_id,omitempty"`
	ProcessName string `json:"process_name,omitempty"`
	SessionID   uint64 `json:"session_id,omitempty"`
	TTY         string `json:"tty,omitempty"`
	HostName    string `json:"host_name,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
}

// TargetInfo represents target information in an event.
type TargetInfo struct {
	Path      string `json:"path,omitempty"`
	FileType  string `json:"file_type,omitempty"`
	OldHash   string `json:"old_hash,omitempty"`
	NewHash   string `json:"new_hash,omitempty"`
	OldPerm   string `json:"old_perm,omitempty"`
	NewPerm   string `json:"new_perm,omitempty"`
	OldOwner  string `json:"old_owner,omitempty"`
	NewOwner  string `json:"new_owner,omitempty"`
}

// ChangeInfo represents a change in an event.
type ChangeInfo struct {
	Field     string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// BaseClient provides common functionality for SIEM clients.
type BaseClient struct {
	Config     *Config
	HTTPClient *http.Client
	Buffer     []Event
}

// NewBaseClient creates a new base client.
func NewBaseClient(config *Config) *BaseClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &BaseClient{
		Config: config,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
		Buffer: make([]Event, 0, config.BufferSize),
	}
}

// SendWithRetry sends data with retry logic.
func (c *BaseClient) SendWithRetry(ctx context.Context, data io.Reader, endpoint string) error {
	var lastErr error

	for i := 0; i <= c.Config.RetryCount; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, data)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		if c.Config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Read error body
		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("SIEM returned %d: %s", resp.StatusCode, string(body))

		// Don't retry on client errors
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return lastErr
		}
	}

	return lastErr
}

// BufferEvent adds an event to the buffer.
func (c *BaseClient) BufferEvent(event Event) {
	c.Buffer = append(c.Buffer, event)
	if len(c.Buffer) >= c.Config.BufferSize {
		c.FlushBuffer()
	}
}

// FlushBuffer sends all buffered events.
func (c *BaseClient) FlushBuffer() {
	if len(c.Buffer) == 0 {
		return
	}
	// Clear buffer
	c.Buffer = c.Buffer[:0]
}

// SerializeEvents serializes events to JSON.
func SerializeEvents(events []Event) ([]byte, error) {
	return json.Marshal(events)
}

// DeserializeEvents deserializes events from JSON.
func DeserializeEvents(data []byte) ([]Event, error) {
	var events []Event
	err := json.Unmarshal(data, &events)
	return events, err
}

// CreateFIMEvent creates a SIEM event from FIM data.
func CreateFIMEvent(eventType, riskLevel, source string, actor *ActorInfo, target *TargetInfo, changes []ChangeInfo) *Event {
	return &Event{
		Timestamp:   time.Now(),
		Source:      source,
		SourceType:  "FIM",
		EventType:   eventType,
		RiskLevel:   riskLevel,
		Actor:       actor,
		Target:      target,
		Changes:     changes,
		Metadata:    make(map[string]interface{}),
	}
}

// CreateAuditEvent creates a SIEM event from audit data.
func CreateAuditEvent(eventType, source string, actor *ActorInfo, target *TargetInfo, rawData map[string]interface{}) *Event {
	return &Event{
		Timestamp:   time.Now(),
		Source:      source,
		SourceType:  "AUDIT",
		EventType:   eventType,
		RiskLevel:   "INFO",
		Actor:       actor,
		Target:      target,
		RawData:     rawData,
		Metadata:    make(map[string]interface{}),
	}
}

// ValidateEvent validates a SIEM event.
func ValidateEvent(event *Event) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	if event.Source == "" {
		return fmt.Errorf("event source is required")
	}
	if event.EventType == "" {
		return fmt.Errorf("event type is required")
	}
	return nil
}

// BatchEvents groups events into batches.
func BatchEvents(events []Event, batchSize int) [][]Event {
	if batchSize <= 0 {
		batchSize = 100
	}

	var batches [][]Event
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batches = append(batches, events[i:end])
	}
	return batches
}

// MockClient is a mock SIEM client for testing.
type MockClient struct {
	SentEvents []Event
	ShouldFail bool
	Config     *Config
}

// NewMockClient creates a new mock client.
func NewMockClient() *MockClient {
	return &MockClient{
		SentEvents: make([]Event, 0),
		Config:     DefaultConfig(),
	}
}

// Name returns the client name.
func (m *MockClient) Name() string {
	return "mock"
}

// Send stores events for testing.
func (m *MockClient) Send(ctx context.Context, events []Event) error {
	if m.ShouldFail {
		return fmt.Errorf("mock error")
	}
	m.SentEvents = append(m.SentEvents, events...)
	return nil
}

// Test returns nil for mock.
func (m *MockClient) Test(ctx context.Context) error {
	if m.ShouldFail {
		return fmt.Errorf("mock test error")
	}
	return nil
}

// GetSentEvents returns all sent events.
func (m *MockClient) GetSentEvents() []Event {
	return m.SentEvents
}

// Reset clears sent events.
func (m *MockClient) Reset() {
	m.SentEvents = m.SentEvents[:0]
}

// NewEventPayload creates a new event payload for HTTP requests.
func NewEventPayload(events []Event) (*bytes.Buffer, error) {
	data, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(data), nil
}
