package store

import (
	"encoding/json"
	"strings"
	"sync"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// watcher is an internal subscription to store mutations.
type watcher struct {
	prefix string
	ch     chan v1alpha1.WatchEvent
}

// MemoryStore is a thread-safe, in-memory Store backed by a simple map.
// Useful for unit tests and short-lived processes.
type MemoryStore struct {
	mu       sync.RWMutex
	data     map[string][]byte // key -> JSON bytes
	watchers []*watcher
}

// NewMemoryStore creates a ready-to-use in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

// ---------- CRUD ----------

func (m *MemoryStore) Create(key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; exists {
		return ErrAlreadyExists
	}
	m.data[key] = raw

	m.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventAdded,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: value,
	})
	return nil
}

func (m *MemoryStore) Get(key string, target interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	raw, ok := m.data[key]
	if !ok {
		return ErrNotFound
	}
	return json.Unmarshal(raw, target)
}

func (m *MemoryStore) Update(key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; !exists {
		return ErrNotFound
	}
	m.data[key] = raw

	m.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventModified,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: value,
	})
	return nil
}

func (m *MemoryStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	raw, exists := m.data[key]
	if !exists {
		return ErrNotFound
	}
	delete(m.data, key)

	// Deserialise the old value so watchers receive the deleted object.
	var obj interface{}
	_ = json.Unmarshal(raw, &obj)

	m.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventDeleted,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: obj,
	})
	return nil
}

// ---------- List ----------

func (m *MemoryStore) List(prefix string, factory func() interface{}) ([]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []interface{}
	for k, raw := range m.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		obj := factory()
		if err := json.Unmarshal(raw, obj); err != nil {
			return nil, err
		}
		results = append(results, obj)
	}
	return results, nil
}

// ---------- Watch ----------

func (m *MemoryStore) Watch(prefix string) (<-chan v1alpha1.WatchEvent, func()) {
	w := &watcher{
		prefix: prefix,
		ch:     make(chan v1alpha1.WatchEvent, 64),
	}

	m.mu.Lock()
	m.watchers = append(m.watchers, w)
	m.mu.Unlock()

	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, existing := range m.watchers {
			if existing == w {
				m.watchers = append(m.watchers[:i], m.watchers[i+1:]...)
				close(w.ch)
				return
			}
		}
	}

	return w.ch, cancel
}

// ---------- Close ----------

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.watchers {
		close(w.ch)
	}
	m.watchers = nil
	m.data = make(map[string][]byte)
	return nil
}

// ---------- internal ----------

// notify sends the event to every watcher whose prefix matches.
// Must be called while m.mu is held (at least read-locked, but callers
// already hold a write lock during mutations).
func (m *MemoryStore) notify(evt v1alpha1.WatchEvent) {
	for _, w := range m.watchers {
		if strings.HasPrefix(evt.Key, w.prefix) {
			select {
			case w.ch <- evt:
			default:
				// Drop event if the watcher is not consuming fast enough.
			}
		}
	}
}

// kindFromKey extracts the Kind segment from a "/{kind}/{project}/{name}" key.
func kindFromKey(key string) string {
	parts := strings.SplitN(strings.TrimPrefix(key, "/"), "/", 3)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
