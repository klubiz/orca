// Package store provides persistence for Orca resources.
//
// Keys follow the convention "/{kind}/{project}/{name}", mirroring
// Kubernetes-style hierarchical addressing.
package store

import (
	"fmt"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// Store is the persistence interface for all Orca resources.
type Store interface {
	// Create stores a new object at the given key.
	// Returns an error if the key already exists.
	Create(key string, value interface{}) error

	// Get retrieves the object stored at key and deserialises it into target.
	// Returns ErrNotFound if the key does not exist.
	Get(key string, target interface{}) error

	// Update replaces the object at the given key.
	// Returns ErrNotFound if the key does not exist.
	Update(key string, value interface{}) error

	// Delete removes the object at the given key.
	// Returns ErrNotFound if the key does not exist.
	Delete(key string) error

	// List returns every object whose key starts with prefix.
	// factory is called once per result to create a zero-value pointer that
	// the stored JSON is unmarshalled into.
	List(prefix string, factory func() interface{}) ([]interface{}, error)

	// Watch returns a channel that emits events for every mutation whose key
	// starts with prefix. The returned cancel function removes the watcher
	// and closes the channel.
	Watch(prefix string) (<-chan v1alpha1.WatchEvent, func())

	// Close releases any resources held by the store (e.g. BoltDB file handle).
	Close() error
}

// Common sentinel errors.
var (
	ErrAlreadyExists = fmt.Errorf("key already exists")
	ErrNotFound      = fmt.Errorf("key not found")
)

// ResourceKey builds a canonical store key for a resource.
//
//	ResourceKey("AgentPod", "my-project", "worker-1")
//	=> "/AgentPod/my-project/worker-1"
func ResourceKey(kind, project, name string) string {
	return fmt.Sprintf("/%s/%s/%s", kind, project, name)
}
