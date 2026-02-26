package store

import (
	"encoding/json"
	"strings"
	"sync"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("resources")

// BoltStore persists resources to a BoltDB file on disk.
type BoltStore struct {
	db       *bolt.DB
	mu       sync.RWMutex   // protects watchers slice only
	watchers []*boltWatcher // in-memory watchers; same pattern as MemoryStore
}

type boltWatcher struct {
	prefix string
	ch     chan v1alpha1.WatchEvent
}

// NewBoltStore opens (or creates) a BoltDB database at path.
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Ensure the bucket exists.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

// ---------- CRUD ----------

func (b *BoltStore) Create(key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = b.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketName)
		if bkt.Get([]byte(key)) != nil {
			return ErrAlreadyExists
		}
		return bkt.Put([]byte(key), raw)
	})
	if err != nil {
		return err
	}

	b.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventAdded,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: value,
	})
	return nil
}

func (b *BoltStore) Get(key string, target interface{}) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketName)
		raw := bkt.Get([]byte(key))
		if raw == nil {
			return ErrNotFound
		}
		return json.Unmarshal(raw, target)
	})
}

func (b *BoltStore) Update(key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = b.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketName)
		if bkt.Get([]byte(key)) == nil {
			return ErrNotFound
		}
		return bkt.Put([]byte(key), raw)
	})
	if err != nil {
		return err
	}

	b.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventModified,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: value,
	})
	return nil
}

func (b *BoltStore) Delete(key string) error {
	var obj interface{}

	err := b.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketName)
		raw := bkt.Get([]byte(key))
		if raw == nil {
			return ErrNotFound
		}
		// Capture the object before deletion so watchers receive it.
		_ = json.Unmarshal(raw, &obj)
		return bkt.Delete([]byte(key))
	})
	if err != nil {
		return err
	}

	b.notify(v1alpha1.WatchEvent{
		Type:   v1alpha1.EventDeleted,
		Kind:   kindFromKey(key),
		Key:    key,
		Object: obj,
	})
	return nil
}

// ---------- List ----------

func (b *BoltStore) List(prefix string, factory func() interface{}) ([]interface{}, error) {
	var results []interface{}

	err := b.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(bucketName)
		c := bkt.Cursor()
		pfx := []byte(prefix)

		for k, v := c.Seek(pfx); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			obj := factory()
			if err := json.Unmarshal(v, obj); err != nil {
				return err
			}
			results = append(results, obj)
		}
		return nil
	})
	return results, err
}

// ---------- Watch ----------

func (b *BoltStore) Watch(prefix string) (<-chan v1alpha1.WatchEvent, func()) {
	w := &boltWatcher{
		prefix: prefix,
		ch:     make(chan v1alpha1.WatchEvent, 64),
	}

	b.mu.Lock()
	b.watchers = append(b.watchers, w)
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, existing := range b.watchers {
			if existing == w {
				b.watchers = append(b.watchers[:i], b.watchers[i+1:]...)
				close(w.ch)
				return
			}
		}
	}

	return w.ch, cancel
}

// ---------- Close ----------

func (b *BoltStore) Close() error {
	b.mu.Lock()
	for _, w := range b.watchers {
		close(w.ch)
	}
	b.watchers = nil
	b.mu.Unlock()

	return b.db.Close()
}

// ---------- internal ----------

func (b *BoltStore) notify(evt v1alpha1.WatchEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, w := range b.watchers {
		if strings.HasPrefix(evt.Key, w.prefix) {
			select {
			case w.ch <- evt:
			default:
				// Drop event if the watcher is not consuming fast enough.
			}
		}
	}
}
