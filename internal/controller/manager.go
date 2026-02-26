// Package controller implements Kubernetes-style reconciliation loops for Orca resources.
package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// Reconciler processes a single resource key.
type Reconciler interface {
	Reconcile(ctx context.Context, key string) error
}

// workItem represents an item in the work queue with backoff tracking.
type workItem struct {
	key       string
	attempts  int
	nextRetry time.Time
}

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 60 * time.Second
)

// WorkQueue is a rate-limited work queue with exponential backoff.
// It uses the K8s pattern of dirty/processing sets to ensure no events
// are lost while an item is being processed.
type WorkQueue struct {
	mu         sync.Mutex
	items      []workItem
	dirty      map[string]bool // items queued or needing re-queue
	processing map[string]bool // items currently being processed
	notify     chan struct{}
	closed     bool
}

// NewWorkQueue creates a new work queue.
func NewWorkQueue() *WorkQueue {
	return &WorkQueue{
		dirty:      make(map[string]bool),
		processing: make(map[string]bool),
		notify:     make(chan struct{}, 1),
	}
}

// Add enqueues an item. If the item is currently being processed,
// it marks it dirty so it will be re-queued when Done() is called.
func (q *WorkQueue) Add(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	// Mark as dirty. If it's currently being processed, Done() will re-queue it.
	q.dirty[key] = true

	// If already in the queue or being processed, don't add a duplicate item.
	if q.processing[key] {
		return
	}
	// Check if already in items.
	for _, item := range q.items {
		if item.key == key {
			return
		}
	}

	q.items = append(q.items, workItem{
		key:       key,
		attempts:  0,
		nextRetry: time.Time{}, // ready immediately
	})

	// Non-blocking notify.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Get returns the next ready item. It blocks until an item is available
// or the queue is closed. Returns ("", false) when closed.
func (q *WorkQueue) Get() (string, bool) {
	for {
		q.mu.Lock()

		if q.closed && len(q.items) == 0 {
			q.mu.Unlock()
			return "", false
		}

		// Find the first item whose nextRetry has passed.
		now := time.Now()
		for i, item := range q.items {
			if now.After(item.nextRetry) || now.Equal(item.nextRetry) {
				key := item.key
				// Remove from the items slice.
				q.items = append(q.items[:i], q.items[i+1:]...)
				// Mark as processing.
				q.processing[key] = true
				q.mu.Unlock()
				return key, true
			}
		}

		// If there are items but none ready, calculate the shortest wait.
		var sleepDuration time.Duration
		if len(q.items) > 0 {
			earliest := q.items[0].nextRetry
			for _, item := range q.items[1:] {
				if item.nextRetry.Before(earliest) {
					earliest = item.nextRetry
				}
			}
			sleepDuration = time.Until(earliest)
			if sleepDuration < 0 {
				sleepDuration = 0
			}
		}

		q.mu.Unlock()

		// Wait for notification or timeout.
		if sleepDuration > 0 {
			timer := time.NewTimer(sleepDuration)
			select {
			case <-q.notify:
				timer.Stop()
			case <-timer.C:
			}
		} else {
			// No items at all; block until notified.
			<-q.notify
		}
	}
}

// Done marks an item as done. If the item was re-dirtied during processing
// (i.e., a new event arrived while it was being reconciled), it is re-queued.
func (q *WorkQueue) Done(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, key)

	// If the key was re-dirtied while processing, re-add it to the queue.
	if q.dirty[key] {
		delete(q.dirty, key)
		// Re-add as a fresh item.
		q.dirty[key] = true
		q.items = append(q.items, workItem{
			key:       key,
			attempts:  0,
			nextRetry: time.Time{},
		})
		select {
		case q.notify <- struct{}{}:
		default:
		}
	}
}

// Requeue re-adds an item with exponential backoff (1s, 2s, 4s, ..., max 60s).
func (q *WorkQueue) Requeue(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	// Find the current attempt count for this key.
	attempts := 0
	for i, item := range q.items {
		if item.key == key {
			attempts = item.attempts
			q.items = append(q.items[:i], q.items[i+1:]...)
			break
		}
	}

	attempts++
	backoff := initialBackoff * (1 << (attempts - 1))
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	// Remove from processing since we're re-adding.
	delete(q.processing, key)
	q.dirty[key] = true
	q.items = append(q.items, workItem{
		key:       key,
		attempts:  attempts,
		nextRetry: time.Now().Add(backoff),
	})

	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Len returns the number of items in the queue.
func (q *WorkQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Close shuts down the queue, unblocking any pending Get calls.
func (q *WorkQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	close(q.notify)
}

// ---------------------------------------------------------------------------
// Controller Manager
// ---------------------------------------------------------------------------

// Manager coordinates multiple controllers.
type Manager struct {
	store       store.Store
	controllers map[string]*controllerRunner
	logger      *zap.Logger
}

type controllerRunner struct {
	name       string
	reconciler Reconciler
	queue      *WorkQueue
	watchKinds []string
	cancel     context.CancelFunc
}

// NewManager creates a new controller manager.
func NewManager(s store.Store, logger *zap.Logger) *Manager {
	return &Manager{
		store:       s,
		controllers: make(map[string]*controllerRunner),
		logger:      logger,
	}
}

// Register adds a controller that watches specific resource kinds.
func (m *Manager) Register(name string, reconciler Reconciler, watchKinds []string) {
	m.controllers[name] = &controllerRunner{
		name:       name,
		reconciler: reconciler,
		queue:      NewWorkQueue(),
		watchKinds: watchKinds,
	}
}

// Start begins all controllers. Each controller:
//  1. Starts a Watch on the store for its kinds
//  2. Feeds watch events into its WorkQueue
//  3. Runs a worker goroutine that processes items from the queue
func (m *Manager) Start(ctx context.Context) error {
	for name, cr := range m.controllers {
		cCtx, cancel := context.WithCancel(ctx)
		cr.cancel = cancel

		m.logger.Info("starting controller",
			zap.String("controller", name),
			zap.Strings("watchKinds", cr.watchKinds),
		)

		// Start a watcher for each kind this controller cares about.
		for _, kind := range cr.watchKinds {
			prefix := fmt.Sprintf("/%s/", kind)
			eventCh, cancelWatch := m.store.Watch(prefix)

			// Feed watch events into the controller's work queue.
			go m.watchLoop(cCtx, name, eventCh, cancelWatch, cr.queue)
		}

		// Start the worker goroutine.
		go m.workerLoop(cCtx, name, cr.reconciler, cr.queue)
	}

	return nil
}

// watchLoop reads events from a store watch channel and feeds them into the work queue.
func (m *Manager) watchLoop(ctx context.Context, controllerName string, eventCh <-chan v1alpha1.WatchEvent, cancelWatch func(), queue *WorkQueue) {
	defer cancelWatch()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			m.logger.Debug("watch event received",
				zap.String("controller", controllerName),
				zap.String("type", string(event.Type)),
				zap.String("kind", event.Kind),
				zap.String("key", event.Key),
			)
			queue.Add(event.Key)
		}
	}
}

// workerLoop processes items from the work queue using the reconciler.
func (m *Manager) workerLoop(ctx context.Context, controllerName string, reconciler Reconciler, queue *WorkQueue) {
	for {
		key, ok := queue.Get()
		if !ok {
			return
		}

		select {
		case <-ctx.Done():
			queue.Done(key)
			return
		default:
		}

		m.logger.Debug("reconciling",
			zap.String("controller", controllerName),
			zap.String("key", key),
		)

		if err := reconciler.Reconcile(ctx, key); err != nil {
			m.logger.Error("reconcile failed",
				zap.String("controller", controllerName),
				zap.String("key", key),
				zap.Error(err),
			)
			queue.Requeue(key)
		} else {
			queue.Done(key)
		}
	}
}

// Stop gracefully shuts down all controllers.
func (m *Manager) Stop() {
	for name, cr := range m.controllers {
		m.logger.Info("stopping controller", zap.String("controller", name))
		if cr.cancel != nil {
			cr.cancel()
		}
		cr.queue.Close()
	}
}
