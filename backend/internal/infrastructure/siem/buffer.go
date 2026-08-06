package siem

import (
	"context"
	"sync"
	"time"
)

// Buffer provides buffering for SIEM events.
type Buffer struct {
	mu       sync.Mutex
	events   []Event
	size     int
	flushFn  func([]Event) error
	ticker   *time.Ticker
	done     chan struct{}
	timeout  time.Duration
	maxSize  int
}

// BufferConfig configures the buffer.
type BufferConfig struct {
	Size       int           // Max events before flush
	Interval   time.Duration // Flush interval
	Timeout    time.Duration // Max time before forced flush
	FlushFn    func([]Event) error // Function to flush events
}

// DefaultBufferConfig returns default buffer configuration.
func DefaultBufferConfig(flushFn func([]Event) error) *BufferConfig {
	return &BufferConfig{
		Size:     100,
		Interval: 30 * time.Second,
		Timeout:  60 * time.Second,
		FlushFn:  flushFn,
	}
}

// NewBuffer creates a new event buffer.
func NewBuffer(config *BufferConfig) *Buffer {
	b := &Buffer{
		events:   make([]Event, 0, config.Size),
		size:     config.Size,
		flushFn:  config.FlushFn,
		timeout:  config.Timeout,
		maxSize:  config.Size,
		done:     make(chan struct{}),
	}

	if config.Interval > 0 {
		b.ticker = time.NewTicker(config.Interval)
		go b.run()
	}

	return b
}

// run periodically flushes the buffer.
func (b *Buffer) run() {
	for {
		select {
		case <-b.ticker.C:
			b.Flush()
		case <-b.done:
			b.ticker.Stop()
			return
		}
	}
}

// Add adds an event to the buffer.
func (b *Buffer) Add(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, event)

	// Flush if buffer is full
	if len(b.events) >= b.size {
		b.flushLocked()
	}
}

// AddBatch adds multiple events to the buffer.
func (b *Buffer) AddBatch(events []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, events...)

	// Flush if buffer is full
	if len(b.events) >= b.size {
		b.flushLocked()
	}
}

// Flush flushes all buffered events.
func (b *Buffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked()
}

// flushLocked flushes the buffer (must hold lock).
func (b *Buffer) flushLocked() {
	if len(b.events) == 0 {
		return
	}

	// Copy events
	events := make([]Event, len(b.events))
	copy(events, b.events)
	b.events = b.events[:0]

	// Flush asynchronously
	go func() {
		if err := b.flushFn(events); err != nil {
			// On error, put events back in buffer
			b.mu.Lock()
			b.events = append(events, b.events...)
			b.mu.Unlock()
		}
	}()
}

// Close stops the buffer and flushes remaining events.
func (b *Buffer) Close() error {
	close(b.done)
	b.Flush()
	return nil
}

// Len returns the current buffer length.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// Reset clears the buffer without flushing.
func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = b.events[:0]
}

// Queue provides a persistent queue for SIEM events.
type Queue struct {
	mu       sync.Mutex
	events   []Event
	maxSize  int
	diskPath string
}

// NewQueue creates a new event queue.
func NewQueue(maxSize int) *Queue {
	return &Queue{
		events:  make([]Event, 0, maxSize),
		maxSize: maxSize,
	}
}

// Enqueue adds an event to the queue.
func (q *Queue) Enqueue(event Event) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) >= q.maxSize {
		return ErrQueueFull
	}

	q.events = append(q.events, event)
	return nil
}

// Dequeue removes and returns the first event.
func (q *Queue) Dequeue() (Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return Event{}, ErrQueueEmpty
	}

	event := q.events[0]
	q.events = q.events[1:]
	return event, nil
}

// Peek returns the first event without removing it.
func (q *Queue) Peek() (Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return Event{}, ErrQueueEmpty
	}

	return q.events[0], nil
}

// DequeueBatch removes and returns multiple events.
func (q *Queue) DequeueBatch(n int) ([]Event, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return nil, ErrQueueEmpty
	}

	if n > len(q.events) {
		n = len(q.events)
	}

	events := make([]Event, n)
	copy(events, q.events[:n])
	q.events = q.events[n:]
	return events, nil
}

// Len returns the queue length.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.events)
}

// Cap returns the queue capacity.
func (q *Queue) Cap() int {
	return q.maxSize
}

// IsEmpty returns true if queue is empty.
func (q *Queue) IsEmpty() bool {
	return q.Len() == 0
}

// IsFull returns true if queue is full.
func (q *Queue) IsFull() bool {
	return q.Len() >= q.maxSize
}

// Clear removes all events from the queue.
func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.events = q.events[:0]
}

// Queue errors.
var (
	ErrQueueFull  = &QueueError{Message: "queue is full"}
	ErrQueueEmpty = &QueueError{Message: "queue is empty"}
)

// QueueError represents a queue error.
type QueueError struct {
	Message string
}

func (e *QueueError) Error() string {
	return e.Message
}

// Worker processes events from a queue and sends them to SIEM.
type Worker struct {
	queue   *Queue
	client  Client
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// WorkerConfig configures the worker.
type WorkerConfig struct {
	QueueSize int
	Workers   int
	Client    Client
}

// NewWorker creates a new event worker.
func NewWorker(config *WorkerConfig) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		queue:   NewQueue(config.QueueSize),
		client:  config.Client,
		workers: config.Workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the worker goroutines.
func (w *Worker) Start() {
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.run(i)
	}
}

// run processes events.
func (w *Worker) run(id int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			// Try to get events from queue
			events, err := w.queue.DequeueBatch(50)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Send to SIEM
			if err := w.client.Send(w.ctx, events); err != nil {
				// On error, put events back in queue
				for _, event := range events {
					w.queue.Enqueue(event)
				}
				time.Sleep(time.Second)
			}
		}
	}
}

// Stop stops the worker and waits for completion.
func (w *Worker) Stop() error {
	w.cancel()
	w.wg.Wait()

	// Flush remaining events
	events, err := w.queue.DequeueBatch(w.queue.Len())
	if err == nil && len(events) > 0 {
		return w.client.Send(context.Background(), events)
	}
	return nil
}

// Enqueue adds an event to the worker queue.
func (w *Worker) Enqueue(event Event) error {
	return w.queue.Enqueue(event)
}

// Len returns the queue length.
func (w *Worker) Len() int {
	return w.queue.Len()
}
