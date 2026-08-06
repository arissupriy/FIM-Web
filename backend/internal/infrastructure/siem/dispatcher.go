// Package siem provides SIEM integration for event forwarding.
package siem

import (
	"context"
	"log"
	"sync"
	"time"

	"ojs-monitor/backend/internal/domain/models"
)

// Global SIEM dispatcher for FIM events
var (
	globalSIEMDispatcher *FIMDispatcher
	siemOnce          sync.Once
)

// FIMDispatcher wraps the SIEM buffer/worker for FIM events.
type FIMDispatcher struct {
	buffer  *Buffer
	workers []*Worker
	mu      sync.RWMutex
	started bool
}

// SetGlobalDispatcher sets the global SIEM dispatcher.
func SetGlobalDispatcher(dispatcher *FIMDispatcher) {
	siemOnce.Do(func() {
		globalSIEMDispatcher = dispatcher
		log.Println("[SIEM] Global dispatcher set")
	})
}

// GetGlobalDispatcher returns the global SIEM dispatcher.
func GetGlobalDispatcher() *FIMDispatcher {
	return globalSIEMDispatcher
}

// NewFIMDispatcher creates a new FIM event dispatcher.
func NewFIMDispatcher(client Client, workers int) *FIMDispatcher {
	// Create flush function that sends to SIEM client
	flushFn := func(events []Event) error {
		if len(events) == 0 {
			return nil
		}
		return client.Send(context.Background(), events)
	}

	// Create buffer with 30 second interval
	bufferConfig := DefaultBufferConfig(flushFn)
	bufferConfig.Interval = 30 * time.Second
	bufferConfig.Size = 100
	buffer := NewBuffer(bufferConfig)

	d := &FIMDispatcher{
		buffer:  buffer,
		workers: make([]*Worker, 0, workers),
	}

	// Create worker pool for reliable delivery
	for i := 0; i < workers; i++ {
		workerConfig := &WorkerConfig{
			QueueSize: 1000,
			Workers:   1,
			Client:    client,
		}
		worker := NewWorker(workerConfig)
		worker.Start()
		d.workers = append(d.workers, worker)
	}

	return d
}

// Dispatch sends a FIM event to SIEM.
func (d *FIMDispatcher) Dispatch(event *models.FIMEvent) error {
	if event == nil {
		return nil
	}

	// Convert to SIEM event
	siemEvent := ConvertFIMEventToSIEM(event)

	// Add to buffer (will flush on size or interval)
	d.buffer.Add(siemEvent)

	// Also enqueue to worker for reliable delivery
	if len(d.workers) > 0 {
		d.workers[0].Enqueue(siemEvent)
	}

	return nil
}

// Flush forces a buffer flush.
func (d *FIMDispatcher) Flush() {
	d.buffer.Flush()
}

// Stop stops all workers and flushes remaining events.
func (d *FIMDispatcher) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	// Flush buffer first
	d.buffer.Flush()

	// Stop all workers
	var lastErr error
	for _, worker := range d.workers {
		if err := worker.Stop(); err != nil {
			lastErr = err
		}
	}

	d.started = false
	log.Println("[SIEM] Dispatcher stopped")
	return lastErr
}

// Len returns the number of buffered events.
func (d *FIMDispatcher) Len() int {
	return d.buffer.Len()
}

// ConvertFIMEventToSIEM converts a FIM event to a SIEM event.
func ConvertFIMEventToSIEM(fimEvent *models.FIMEvent) Event {
	event := Event{
		Timestamp:  time.Now(),
		Source:    fimEvent.Source,
		SourceType: "FIM",
		EventType: fimEvent.EventType,
		RiskLevel: fimEvent.RiskLevel,
	}

	// Convert actor
	if fimEvent.ActorType != "" || fimEvent.ActorID != "" || fimEvent.ActorName != "" {
		event.Actor = &ActorInfo{
			UserID:      fimEvent.ActorID,
			Username:    fimEvent.ActorName,
			ProcessName: fimEvent.ActorType,
		}
	}

	// Convert target
	if fimEvent.FilePath != "" {
		event.Target = &TargetInfo{
			Path: fimEvent.FilePath,
		}
	}

	// Add metadata
	event.Metadata = make(map[string]interface{})
	event.Metadata["project_id"] = fimEvent.ProjectID
	event.Metadata["classification"] = fimEvent.Classification
	event.Metadata["details"] = fimEvent.Details

	return event
}

// DispatchFIMEvent is a convenience function for global dispatch.
func DispatchFIMEvent(event *models.FIMEvent) error {
	if globalSIEMDispatcher == nil {
		return nil // SIEM not configured
	}
	return globalSIEMDispatcher.Dispatch(event)
}
