package store

import (
	"testing"
	"time"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// newTestPod creates an AgentPod for testing with the given name and project.
func newTestPod(name, project, model string) *v1alpha1.AgentPod {
	return &v1alpha1.AgentPod{
		TypeMeta: v1alpha1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.KindAgentPod,
		},
		Metadata: v1alpha1.ObjectMeta{
			Name:    name,
			Project: project,
		},
		Spec: v1alpha1.AgentPodSpec{
			Model: model,
		},
	}
}

func TestCreate(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("test-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "test-pod")

	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	// Verify the resource exists by reading it back.
	var got v1alpha1.AgentPod
	if err := s.Get(key, &got); err != nil {
		t.Fatalf("unexpected error on Get after Create: %v", err)
	}
	if got.Metadata.Name != "test-pod" {
		t.Errorf("expected name test-pod, got %s", got.Metadata.Name)
	}
	if got.Metadata.Project != "default" {
		t.Errorf("expected project default, got %s", got.Metadata.Project)
	}
	if got.Spec.Model != "claude-sonnet" {
		t.Errorf("expected model claude-sonnet, got %s", got.Spec.Model)
	}
}

func TestCreateDuplicate(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("dup-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "dup-pod")

	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on first Create: %v", err)
	}

	// Creating the same key again must return ErrAlreadyExists.
	err := s.Create(key, pod)
	if err == nil {
		t.Fatal("expected ErrAlreadyExists, got nil")
	}
	if err != ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestGet(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("get-pod", "proj-a", "claude-opus")
	pod.Spec.MaxConcurrency = 5
	pod.Spec.Tools = []string{"bash", "editor"}
	key := ResourceKey(v1alpha1.KindAgentPod, "proj-a", "get-pod")

	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	var got v1alpha1.AgentPod
	if err := s.Get(key, &got); err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}

	if got.Metadata.Name != "get-pod" {
		t.Errorf("expected name get-pod, got %s", got.Metadata.Name)
	}
	if got.Metadata.Project != "proj-a" {
		t.Errorf("expected project proj-a, got %s", got.Metadata.Project)
	}
	if got.Spec.Model != "claude-opus" {
		t.Errorf("expected model claude-opus, got %s", got.Spec.Model)
	}
	if got.Spec.MaxConcurrency != 5 {
		t.Errorf("expected maxConcurrency 5, got %d", got.Spec.MaxConcurrency)
	}
	if len(got.Spec.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(got.Spec.Tools))
	} else {
		if got.Spec.Tools[0] != "bash" {
			t.Errorf("expected first tool bash, got %s", got.Spec.Tools[0])
		}
		if got.Spec.Tools[1] != "editor" {
			t.Errorf("expected second tool editor, got %s", got.Spec.Tools[1])
		}
	}
}

func TestGetNotFound(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	key := ResourceKey(v1alpha1.KindAgentPod, "default", "nonexistent")

	var got v1alpha1.AgentPod
	err := s.Get(key, &got)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdate(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("update-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "update-pod")

	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	// Update with a modified spec.
	updated := newTestPod("update-pod", "default", "claude-opus")
	updated.Spec.MaxConcurrency = 10
	updated.Spec.SystemPrompt = "You are a helpful assistant."

	if err := s.Update(key, updated); err != nil {
		t.Fatalf("unexpected error on Update: %v", err)
	}

	var got v1alpha1.AgentPod
	if err := s.Get(key, &got); err != nil {
		t.Fatalf("unexpected error on Get after Update: %v", err)
	}
	if got.Spec.Model != "claude-opus" {
		t.Errorf("expected model claude-opus after update, got %s", got.Spec.Model)
	}
	if got.Spec.MaxConcurrency != 10 {
		t.Errorf("expected maxConcurrency 10 after update, got %d", got.Spec.MaxConcurrency)
	}
	if got.Spec.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("expected systemPrompt to be set after update, got %q", got.Spec.SystemPrompt)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("ghost-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "ghost-pod")

	err := s.Update(key, pod)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	pod := newTestPod("delete-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "delete-pod")

	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	// Delete the resource.
	if err := s.Delete(key); err != nil {
		t.Fatalf("unexpected error on Delete: %v", err)
	}

	// Verify the resource is gone.
	var got v1alpha1.AgentPod
	err := s.Get(key, &got)
	if err == nil {
		t.Fatal("expected ErrNotFound after Delete, got nil")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	key := ResourceKey(v1alpha1.KindAgentPod, "default", "nonexistent")

	err := s.Delete(key)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	// Create pods in two different projects.
	pods := []struct {
		name    string
		project string
		model   string
	}{
		{"pod-1", "proj-a", "claude-sonnet"},
		{"pod-2", "proj-a", "claude-opus"},
		{"pod-3", "proj-b", "claude-haiku"},
		{"pod-4", "proj-b", "claude-sonnet"},
	}

	for _, p := range pods {
		pod := newTestPod(p.name, p.project, p.model)
		key := ResourceKey(v1alpha1.KindAgentPod, p.project, p.name)
		if err := s.Create(key, pod); err != nil {
			t.Fatalf("unexpected error creating %s: %v", p.name, err)
		}
	}

	factory := func() interface{} { return &v1alpha1.AgentPod{} }

	t.Run("list all AgentPods", func(t *testing.T) {
		prefix := "/" + v1alpha1.KindAgentPod + "/"
		results, err := s.List(prefix, factory)
		if err != nil {
			t.Fatalf("unexpected error on List: %v", err)
		}
		if len(results) != 4 {
			t.Fatalf("expected 4 results, got %d", len(results))
		}
	})

	t.Run("list AgentPods in proj-a", func(t *testing.T) {
		prefix := "/" + v1alpha1.KindAgentPod + "/proj-a/"
		results, err := s.List(prefix, factory)
		if err != nil {
			t.Fatalf("unexpected error on List: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results for proj-a, got %d", len(results))
		}

		// Verify all returned pods belong to proj-a.
		for _, r := range results {
			pod, ok := r.(*v1alpha1.AgentPod)
			if !ok {
				t.Fatal("expected result to be *v1alpha1.AgentPod")
			}
			if pod.Metadata.Project != "proj-a" {
				t.Errorf("expected project proj-a, got %s", pod.Metadata.Project)
			}
		}
	})

	t.Run("list AgentPods in proj-b", func(t *testing.T) {
		prefix := "/" + v1alpha1.KindAgentPod + "/proj-b/"
		results, err := s.List(prefix, factory)
		if err != nil {
			t.Fatalf("unexpected error on List: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results for proj-b, got %d", len(results))
		}
	})

	t.Run("list with no matching prefix", func(t *testing.T) {
		prefix := "/NonExistentKind/"
		results, err := s.List(prefix, factory)
		if err != nil {
			t.Fatalf("unexpected error on List: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})
}

func TestWatch(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	prefix := "/" + v1alpha1.KindAgentPod + "/"
	ch, cancel := s.Watch(prefix)
	defer cancel()

	key := ResourceKey(v1alpha1.KindAgentPod, "default", "watch-pod")

	// --- Create ---
	pod := newTestPod("watch-pod", "default", "claude-sonnet")
	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	evt := receiveEvent(t, ch, 2*time.Second)
	if evt.Type != v1alpha1.EventAdded {
		t.Errorf("expected event type ADDED, got %s", evt.Type)
	}
	if evt.Key != key {
		t.Errorf("expected event key %s, got %s", key, evt.Key)
	}
	if evt.Kind != v1alpha1.KindAgentPod {
		t.Errorf("expected event kind AgentPod, got %s", evt.Kind)
	}

	// --- Update ---
	updated := newTestPod("watch-pod", "default", "claude-opus")
	if err := s.Update(key, updated); err != nil {
		t.Fatalf("unexpected error on Update: %v", err)
	}

	evt = receiveEvent(t, ch, 2*time.Second)
	if evt.Type != v1alpha1.EventModified {
		t.Errorf("expected event type MODIFIED, got %s", evt.Type)
	}
	if evt.Key != key {
		t.Errorf("expected event key %s, got %s", key, evt.Key)
	}

	// --- Delete ---
	if err := s.Delete(key); err != nil {
		t.Fatalf("unexpected error on Delete: %v", err)
	}

	evt = receiveEvent(t, ch, 2*time.Second)
	if evt.Type != v1alpha1.EventDeleted {
		t.Errorf("expected event type DELETED, got %s", evt.Type)
	}
	if evt.Key != key {
		t.Errorf("expected event key %s, got %s", key, evt.Key)
	}
}

func TestWatchPrefixFiltering(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	// Watch only proj-a AgentPods.
	prefix := "/" + v1alpha1.KindAgentPod + "/proj-a/"
	ch, cancel := s.Watch(prefix)
	defer cancel()

	// Create a pod in proj-a (should trigger event).
	keyA := ResourceKey(v1alpha1.KindAgentPod, "proj-a", "pod-a")
	podA := newTestPod("pod-a", "proj-a", "claude-sonnet")
	if err := s.Create(keyA, podA); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	evt := receiveEvent(t, ch, 2*time.Second)
	if evt.Key != keyA {
		t.Errorf("expected event for %s, got %s", keyA, evt.Key)
	}

	// Create a pod in proj-b (should NOT trigger event on the proj-a watcher).
	keyB := ResourceKey(v1alpha1.KindAgentPod, "proj-b", "pod-b")
	podB := newTestPod("pod-b", "proj-b", "claude-haiku")
	if err := s.Create(keyB, podB); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	// Ensure no event is received for proj-b.
	select {
	case got := <-ch:
		t.Fatalf("unexpected event for proj-b watcher: %+v", got)
	case <-time.After(100 * time.Millisecond):
		// Expected: no event received.
	}
}

func TestWatchCancel(t *testing.T) {
	s := NewMemoryStore()
	defer s.Close()

	prefix := "/" + v1alpha1.KindAgentPod + "/"
	ch, cancel := s.Watch(prefix)

	// Cancel the watch.
	cancel()

	// The channel should be closed. Reading from a closed channel returns the
	// zero value immediately.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after cancel, but received a value")
		}
		// Channel is closed as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel to close after cancel")
	}

	// Mutations after cancel must not panic.
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "after-cancel")
	pod := newTestPod("after-cancel", "default", "claude-sonnet")
	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create after cancel: %v", err)
	}
}

func TestResourceKey(t *testing.T) {
	tests := []struct {
		kind    string
		project string
		name    string
		want    string
	}{
		{
			kind:    v1alpha1.KindAgentPod,
			project: "my-project",
			name:    "worker-1",
			want:    "/AgentPod/my-project/worker-1",
		},
		{
			kind:    v1alpha1.KindProject,
			project: "default",
			name:    "my-project",
			want:    "/Project/default/my-project",
		},
		{
			kind:    v1alpha1.KindAgentPool,
			project: "prod",
			name:    "pool-alpha",
			want:    "/AgentPool/prod/pool-alpha",
		},
		{
			kind:    v1alpha1.KindDevTask,
			project: "staging",
			name:    "task-42",
			want:    "/DevTask/staging/task-42",
		},
		{
			kind:    "CustomKind",
			project: "",
			name:    "unnamed",
			want:    "/CustomKind//unnamed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := ResourceKey(tc.kind, tc.project, tc.name)
			if got != tc.want {
				t.Errorf("ResourceKey(%q, %q, %q) = %q, want %q",
					tc.kind, tc.project, tc.name, got, tc.want)
			}
		})
	}
}

func TestClose(t *testing.T) {
	s := NewMemoryStore()

	// Create some data and a watcher.
	pod := newTestPod("close-pod", "default", "claude-sonnet")
	key := ResourceKey(v1alpha1.KindAgentPod, "default", "close-pod")
	if err := s.Create(key, pod); err != nil {
		t.Fatalf("unexpected error on Create: %v", err)
	}

	ch, _ := s.Watch("/" + v1alpha1.KindAgentPod + "/")

	// Close the store.
	if err := s.Close(); err != nil {
		t.Fatalf("unexpected error on Close: %v", err)
	}

	// The watcher channel should be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected watcher channel to be closed after store Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher channel to close after store Close")
	}

	// Data should be cleared; Get should return ErrNotFound.
	var got v1alpha1.AgentPod
	err := s.Get(key, &got)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after Close, got %v", err)
	}
}

// ---------- helpers ----------

// receiveEvent reads a single event from ch with a timeout. It fails the test
// if no event is received within the deadline.
func receiveEvent(t *testing.T, ch <-chan v1alpha1.WatchEvent, timeout time.Duration) v1alpha1.WatchEvent {
	t.Helper()
	select {
	case evt := <-ch:
		return evt
	case <-time.After(timeout):
		t.Fatal("timed out waiting for watch event")
		return v1alpha1.WatchEvent{} // unreachable, satisfies compiler
	}
}
