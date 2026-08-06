package siem

import (
	"context"
	"fmt"
)

// Registry manages SIEM client registrations.
type Registry struct {
	clients map[string]Client
}

// NewRegistry creates a new SIEM registry.
func NewRegistry() *Registry {
	return &Registry{
		clients: make(map[string]Client),
	}
}

// Register registers a SIEM client.
func (r *Registry) Register(name string, client Client) {
	r.clients[name] = client
}

// Get retrieves a registered client.
func (r *Registry) Get(name string) (Client, bool) {
	client, ok := r.clients[name]
	return client, ok
}

// List returns all registered client names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	return names
}

// Unregister removes a client from the registry.
func (r *Registry) Unregister(name string) {
	delete(r.clients, name)
}

// TestAll tests all registered clients.
func (r *Registry) TestAll(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for name, client := range r.clients {
		results[name] = client.Test(ctx)
	}

	return results
}

// CreateClient creates a SIEM client based on configuration.
func CreateClient(backend string, config *Config) (Client, error) {
	switch backend {
	case "elasticsearch":
		return NewElasticsearchClient(config), nil
	case "splunk":
		return NewSplunkHECClient(config), nil
	case "syslog":
		return NewSyslogChannel(config), nil
	default:
		return nil, fmt.Errorf("unknown SIEM backend: %s", backend)
	}
}

// SIEMConfig represents SIEM integration configuration.
type SIEMConfig struct {
	Enabled    bool              `json:"enabled"`
	Backends   []BackendConfig   `json:"backends"`
	BufferSize int               `json:"buffer_size"`
	FlushInterval string         `json:"flush_interval"`
	RetryCount int              `json:"retry_count"`
}

// BackendConfig represents a single SIEM backend configuration.
type BackendConfig struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"` // elasticsearch, splunk, syslog
	Enabled bool        `json:"enabled"`
	Config  *Config     `json:"config"`
}

// DefaultSIEMConfig returns default SIEM configuration.
func DefaultSIEMConfig() *SIEMConfig {
	return &SIEMConfig{
		Enabled:      true,
		BufferSize:   100,
		FlushInterval: "30s",
		RetryCount:   3,
		Backends: []BackendConfig{
			{
				Name:    "elasticsearch",
				Type:    "elasticsearch",
				Enabled: true,
				Config: &Config{
					Endpoint:   "http://localhost:9200",
					BufferSize: 100,
					RetryCount: 3,
					Index:      "ojs-monitor-fim",
				},
			},
			{
				Name:    "syslog",
				Type:    "syslog",
				Enabled: false,
				Config: &Config{
					Endpoint: "localhost",
					BufferSize: 100,
					RetryCount: 3,
				},
			},
		},
	}
}

// SIEMService manages SIEM integration.
type SIEMService struct {
	registry *Registry
	config   *SIEMConfig
	workers  map[string]*Worker
}

// NewSIEMService creates a new SIEM service.
func NewSIEMService(config *SIEMConfig) *SIEMService {
	return &SIEMService{
		registry: NewRegistry(),
		config:   config,
		workers:  make(map[string]*Worker),
	}
}

// Initialize initializes all configured backends.
func (s *SIEMService) Initialize() error {
	for _, backend := range s.config.Backends {
		if !backend.Enabled {
			continue
		}

		client, err := CreateClient(backend.Type, backend.Config)
		if err != nil {
			return fmt.Errorf("create %s client: %w", backend.Name, err)
		}

		s.registry.Register(backend.Name, client)
	}

	return nil
}

// Send sends an event to all enabled backends.
func (s *SIEMService) Send(ctx context.Context, event *Event) error {
	if err := ValidateEvent(event); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}

	var lastErr error
	for _, name := range s.registry.List() {
		client, _ := s.registry.Get(name)
		if client == nil {
			continue
		}

		if err := client.Send(ctx, []Event{*event}); err != nil {
			lastErr = fmt.Errorf("%s: %w", name, err)
		}
	}

	return lastErr
}

// SendBatch sends multiple events to all enabled backends.
func (s *SIEMService) SendBatch(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := ValidateEvent(&event); err != nil {
			return fmt.Errorf("validate event: %w", err)
		}
	}

	var lastErr error
	for _, name := range s.registry.List() {
		client, _ := s.registry.Get(name)
		if client == nil {
			continue
		}

		if err := client.Send(ctx, events); err != nil {
			lastErr = fmt.Errorf("%s: %w", name, err)
		}
	}

	return lastErr
}

// Test tests all backends.
func (s *SIEMService) Test(ctx context.Context) map[string]error {
	return s.registry.TestAll(ctx)
}

// GetConfig returns the current configuration.
func (s *SIEMService) GetConfig() *SIEMConfig {
	return s.config
}

// UpdateConfig updates the configuration.
func (s *SIEMService) UpdateConfig(config *SIEMConfig) error {
	// Stop existing workers
	for name := range s.workers {
		if worker, ok := s.workers[name]; ok {
			worker.Stop()
		}
	}
	s.workers = make(map[string]*Worker)

	// Update config
	s.config = config

	// Reinitialize
	return s.Initialize()
}

// Start starts background workers for async sending.
func (s *SIEMService) Start() error {
	for _, backend := range s.config.Backends {
		if !backend.Enabled {
			continue
		}

		client, ok := s.registry.Get(backend.Name)
		if !ok {
			continue
		}

		worker := NewWorker(&WorkerConfig{
			QueueSize: s.config.BufferSize,
			Workers:   2,
			Client:   client,
		})

		worker.Start()
		s.workers[backend.Name] = worker
	}

	return nil
}

// Stop stops all background workers.
func (s *SIEMService) Stop() error {
	for name, worker := range s.workers {
		if err := worker.Stop(); err != nil {
			return fmt.Errorf("stop %s: %w", name, err)
		}
	}
	return nil
}

// Enqueue queues an event for async sending.
func (s *SIEMService) Enqueue(event Event) error {
	// Send to first available worker
	for _, worker := range s.workers {
		return worker.Enqueue(event)
	}

	// No workers, try direct send
	return s.Send(context.Background(), &event)
}

// GetRegistry returns the client registry.
func (s *SIEMService) GetRegistry() *Registry {
	return s.registry
}
