// Package alert provides alert dispatching infrastructure.
package alert

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// Dispatcher handles alert dispatching and deduplication.
type Dispatcher struct {
	configRepo  repository.AlertConfigRepository
	historyRepo repository.AlertHistoryRepository
	channels    map[models.AlertChannel]Channel
	eventQueue  chan *models.FIMEvent
	stopCh      chan struct{}
}

// Channel is the interface for alert delivery channels.
type Channel interface {
	// Send sends an alert.
	Send(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) error
	// Name returns the channel name.
	Name() string
}

// NewDispatcher creates a new alert dispatcher.
func NewDispatcher(
	configRepo repository.AlertConfigRepository,
	historyRepo repository.AlertHistoryRepository,
) *Dispatcher {
	return &Dispatcher{
		configRepo:  configRepo,
		historyRepo: historyRepo,
		channels:    make(map[models.AlertChannel]Channel),
		eventQueue:  make(chan *models.FIMEvent, 100),
		stopCh:      make(chan struct{}),
	}
}

// RegisterChannel registers an alert channel.
func (d *Dispatcher) RegisterChannel(channel models.AlertChannel, ch Channel) {
	d.channels[channel] = ch
}

// Start starts the dispatcher.
func (d *Dispatcher) Start(ctx context.Context) {
	log.Println("Starting alert dispatcher...")
	go d.processLoop(ctx)
}

// Stop stops the dispatcher.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	log.Println("Alert dispatcher stopped")
}

// Dispatch queues an event for processing.
func (d *Dispatcher) Dispatch(event *models.FIMEvent) {
	select {
	case d.eventQueue <- event:
	default:
		log.Printf("Alert queue full, dropping event for %s", event.FilePath)
	}
}

// processLoop processes events from the queue.
func (d *Dispatcher) processLoop(ctx context.Context) {
	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		case event := <-d.eventQueue:
			d.processEvent(ctx, event)
		}
	}
}

// processEvent processes a single FIM event.
func (d *Dispatcher) processEvent(ctx context.Context, event *models.FIMEvent) {
	// Get enabled configs for this project
	configs, err := d.configRepo.ListEnabled(ctx, event.ProjectID)
	if err != nil {
		log.Printf("Failed to get alert configs for project %d: %v", event.ProjectID, err)
		return
	}

	for _, config := range configs {
		if d.shouldAlert(config, event) {
			// Check dedup
			if d.isDuplicate(ctx, config, event) {
				continue
			}
			// Dispatch alert
			d.dispatchAlert(ctx, config, event)
		}
	}
}

// shouldAlert checks if the event matches the config conditions.
func (d *Dispatcher) shouldAlert(config *models.AlertConfig, event *models.FIMEvent) bool {
	// Check risk level (minimum threshold)
	if !meetsRiskLevel(event.RiskLevel, string(config.RiskLevel)) {
		return false
	}

	// Parse conditions
	var conditions models.AlertCondition
	if config.Conditions != "" && config.Conditions != "{}" {
		if err := json.Unmarshal([]byte(config.Conditions), &conditions); err != nil {
			log.Printf("Failed to parse conditions: %v", err)
			// If conditions are invalid, still allow alert based on risk level
			return true
		}
	}

	// Check event type
	if len(conditions.EventTypes) > 0 {
		found := false
		for _, et := range conditions.EventTypes {
			if et == event.EventType || et == "*" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check file path patterns
	if len(conditions.FilePatterns) > 0 {
		found := false
		for _, pattern := range conditions.FilePatterns {
			if matchPattern(event.FilePath, pattern) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check classifications
	if len(conditions.Classifications) > 0 {
		found := false
		for _, c := range conditions.Classifications {
			if c == event.Classification || c == "*" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// meetsRiskLevel checks if event risk level meets or exceeds the threshold.
func meetsRiskLevel(eventLevel, threshold string) bool {
	levels := map[string]int{
		"LOW":      1,
		"MEDIUM":   2,
		"HIGH":     3,
		"CRITICAL": 4,
	}

	eventVal, ok1 := levels[eventLevel]
	thresholdVal, ok2 := levels[threshold]

	if !ok1 || !ok2 {
		return false
	}

	return eventVal >= thresholdVal
}

// isDuplicate checks if an alert was recently sent for the same file.
func (d *Dispatcher) isDuplicate(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) bool {
	// Use config's dedup window, default to 60 seconds
	dedupWindow := config.DedupWindow
	if dedupWindow == 0 {
		dedupWindow = 60
	}

	isDup, err := d.historyRepo.CheckDedup(ctx, event.ProjectID, event.FilePath, event.RiskLevel, dedupWindow)
	if err != nil {
		log.Printf("Failed to check dedup: %v", err)
		return false // Don't block on dedup errors
	}

	return isDup
}

// dispatchAlert dispatches an alert to the configured channel.
func (d *Dispatcher) dispatchAlert(ctx context.Context, config *models.AlertConfig, event *models.FIMEvent) {
	channel, ok := d.channels[config.Channel]
	if !ok {
		log.Printf("No channel registered for %s", config.Channel)
		return
	}

	// Create history entry
	history := &models.AlertHistory{
		AlertConfigID: config.ID,
		FIMEventID:    event.ID,
		ProjectID:     event.ProjectID,
		Channel:       config.Channel,
		Status:        models.AlertStatusPending,
		RetryCount:    0,
		MaxRetries:   3,
	}

	id, err := d.historyRepo.Create(ctx, history)
	if err != nil {
		log.Printf("Failed to create alert history: %v", err)
		return
	}
	history.ID = id

	// Dispatch to channel
	err = channel.Send(ctx, config, event)
	if err != nil {
		log.Printf("Failed to send alert via %s: %v", config.Channel, err)
		d.historyRepo.UpdateStatus(ctx, id, models.AlertStatusFailed, err.Error())
		return
	}

	// Mark as sent
	d.historyRepo.MarkSent(ctx, id)
	log.Printf("Alert sent via %s for %s", config.Channel, event.FilePath)
}

// matchPattern checks if a path matches a glob pattern.
func matchPattern(path, pattern string) bool {
	// Simple glob matching
	// Support * for any characters
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return path == pattern
	}

	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(path, part)
		if idx == -1 {
			return false
		}
		path = path[idx+len(part):]
	}
	return true
}

// DefaultDedupWindow is the default dedup window in seconds.
const DefaultDedupWindow = 60
