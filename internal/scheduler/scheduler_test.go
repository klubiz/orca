package scheduler

import (
	"testing"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

// podBuilder provides a fluent API for constructing test AgentPods.
type podBuilder struct {
	pod v1alpha1.AgentPod
}

func newPod(name, project string) *podBuilder {
	return &podBuilder{
		pod: v1alpha1.AgentPod{
			TypeMeta: v1alpha1.TypeMeta{
				APIVersion: v1alpha1.APIVersion,
				Kind:       v1alpha1.KindAgentPod,
			},
			Metadata: v1alpha1.ObjectMeta{
				Name:    name,
				Project: project,
			},
			Spec: v1alpha1.AgentPodSpec{
				Model:          "claude-3",
				MaxConcurrency: 1,
			},
			Status: v1alpha1.AgentPodStatus{
				Phase: v1alpha1.PodReady,
			},
		},
	}
}

func (b *podBuilder) phase(p v1alpha1.AgentPodPhase) *podBuilder {
	b.pod.Status.Phase = p
	return b
}

func (b *podBuilder) model(m string) *podBuilder {
	b.pod.Spec.Model = m
	return b
}

func (b *podBuilder) capabilities(caps ...string) *podBuilder {
	b.pod.Spec.Capabilities = caps
	return b
}

func (b *podBuilder) maxConcurrency(n int) *podBuilder {
	b.pod.Spec.MaxConcurrency = n
	return b
}

func (b *podBuilder) activeTasks(n int) *podBuilder {
	b.pod.Status.ActiveTasks = n
	return b
}

func (b *podBuilder) build() *v1alpha1.AgentPod {
	p := b.pod // copy
	return &p
}

// taskBuilder provides a fluent API for constructing test DevTasks.
type taskBuilder struct {
	task v1alpha1.DevTask
}

func newTask(name, project string) *taskBuilder {
	return &taskBuilder{
		task: v1alpha1.DevTask{
			TypeMeta: v1alpha1.TypeMeta{
				APIVersion: v1alpha1.APIVersion,
				Kind:       v1alpha1.KindDevTask,
			},
			Metadata: v1alpha1.ObjectMeta{
				Name:    name,
				Project: project,
			},
			Spec: v1alpha1.DevTaskSpec{
				Prompt: "test prompt",
			},
		},
	}
}

func (b *taskBuilder) requiredCapabilities(caps ...string) *taskBuilder {
	b.task.Spec.RequiredCapabilities = caps
	return b
}

func (b *taskBuilder) preferredModel(m string) *taskBuilder {
	b.task.Spec.PreferredModel = m
	return b
}

func (b *taskBuilder) build() *v1alpha1.DevTask {
	t := b.task // copy
	return &t
}

// addPodToStore is a convenience function that stores an AgentPod using the
// canonical key convention.
func addPodToStore(t *testing.T, s store.Store, pod *v1alpha1.AgentPod) {
	t.Helper()
	key := store.ResourceKey(v1alpha1.KindAgentPod, pod.Metadata.Project, pod.Metadata.Name)
	if err := s.Create(key, pod); err != nil {
		t.Fatalf("failed to add pod %q to store: %v", pod.Metadata.Name, err)
	}
}

// =========================================================================
// Predicate tests
// =========================================================================

func TestPodIsReady(t *testing.T) {
	task := newTask("task-1", "proj").build()

	tests := []struct {
		name  string
		phase v1alpha1.AgentPodPhase
		want  bool
	}{
		{"ready pod", v1alpha1.PodReady, true},
		{"pending pod", v1alpha1.PodPending, false},
		{"starting pod", v1alpha1.PodStarting, false},
		{"busy pod", v1alpha1.PodBusy, false},
		{"failed pod", v1alpha1.PodFailed, false},
		{"terminating pod", v1alpha1.PodTerminating, false},
		{"terminated pod", v1alpha1.PodTerminated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").phase(tt.phase).build()
			got := PodIsReady(pod, task)
			if got != tt.want {
				t.Errorf("PodIsReady() with phase %q = %v, want %v", tt.phase, got, tt.want)
			}
		})
	}
}

func TestPodHasCapacity(t *testing.T) {
	task := newTask("task-1", "proj").build()

	tests := []struct {
		name           string
		maxConcurrency int
		activeTasks    int
		want           bool
	}{
		{"has room", 4, 2, true},
		{"empty pod", 4, 0, true},
		{"full pod", 4, 4, false},
		{"over capacity", 4, 5, false},
		{"zero maxConcurrency treated as 1 with 0 active", 0, 0, true},
		{"zero maxConcurrency treated as 1 with 1 active", 0, 1, false},
		{"single slot available", 1, 0, true},
		{"single slot full", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").
				maxConcurrency(tt.maxConcurrency).
				activeTasks(tt.activeTasks).
				build()
			got := PodHasCapacity(pod, task)
			if got != tt.want {
				t.Errorf("PodHasCapacity(max=%d, active=%d) = %v, want %v",
					tt.maxConcurrency, tt.activeTasks, got, tt.want)
			}
		})
	}
}

func TestPodMatchesCapability(t *testing.T) {
	tests := []struct {
		name     string
		podCaps  []string
		taskCaps []string
		want     bool
	}{
		{"matching caps", []string{"go", "docker"}, []string{"go", "docker"}, true},
		{"superset of required", []string{"go", "docker", "k8s"}, []string{"go"}, true},
		{"missing one cap", []string{"go"}, []string{"go", "docker"}, false},
		{"no caps on pod, task requires", nil, []string{"go"}, false},
		{"no required caps", []string{"go"}, nil, true},
		{"both empty", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").capabilities(tt.podCaps...).build()
			task := newTask("t1", "proj").requiredCapabilities(tt.taskCaps...).build()
			got := PodMatchesCapability(pod, task)
			if got != tt.want {
				t.Errorf("PodMatchesCapability(pod=%v, task=%v) = %v, want %v",
					tt.podCaps, tt.taskCaps, got, tt.want)
			}
		})
	}
}

func TestPodMatchesModel(t *testing.T) {
	tests := []struct {
		name      string
		podModel  string
		taskModel string
		want      bool
	}{
		{"exact match", "claude-3", "claude-3", true},
		{"mismatch", "claude-3", "gpt-4", false},
		{"no preference", "claude-3", "", true},
		{"empty pod model with preference", "", "claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").model(tt.podModel).build()
			task := newTask("t1", "proj").preferredModel(tt.taskModel).build()
			got := PodMatchesModel(pod, task)
			if got != tt.want {
				t.Errorf("PodMatchesModel(pod=%q, task=%q) = %v, want %v",
					tt.podModel, tt.taskModel, got, tt.want)
			}
		})
	}
}

func TestPodInSameProject(t *testing.T) {
	tests := []struct {
		name       string
		podProject  string
		taskProject string
		want        bool
	}{
		{"same project", "alpha", "alpha", true},
		{"different project", "alpha", "beta", false},
		{"empty projects", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", tt.podProject).build()
			task := newTask("t1", tt.taskProject).build()
			got := PodInSameProject(pod, task)
			if got != tt.want {
				t.Errorf("PodInSameProject(pod=%q, task=%q) = %v, want %v",
					tt.podProject, tt.taskProject, got, tt.want)
			}
		})
	}
}

// =========================================================================
// Priority tests
// =========================================================================

func TestLeastLoaded(t *testing.T) {
	task := newTask("task-1", "proj").build()

	tests := []struct {
		name           string
		maxConcurrency int
		activeTasks    int
		wantScore      int
	}{
		{"empty pod max 4", 4, 0, 100},
		{"half loaded max 4", 4, 2, 50},
		{"3/4 loaded", 4, 3, 25},
		{"full pod", 4, 4, 0},
		{"over capacity", 4, 5, 0},
		{"zero max treated as 1, empty", 0, 0, 100},
		{"1 active, max 1", 1, 1, 0},
		{"1 active, max 10", 10, 1, 90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").
				maxConcurrency(tt.maxConcurrency).
				activeTasks(tt.activeTasks).
				build()
			got := LeastLoaded(pod, task)
			if got != tt.wantScore {
				t.Errorf("LeastLoaded(max=%d, active=%d) = %d, want %d",
					tt.maxConcurrency, tt.activeTasks, got, tt.wantScore)
			}
		})
	}

	// Verify relative ordering: fewer tasks -> higher score.
	podLight := newPod("light", "proj").maxConcurrency(10).activeTasks(1).build()
	podHeavy := newPod("heavy", "proj").maxConcurrency(10).activeTasks(8).build()
	scoreLight := LeastLoaded(podLight, task)
	scoreHeavy := LeastLoaded(podHeavy, task)
	if scoreLight <= scoreHeavy {
		t.Errorf("expected lighter pod to score higher: light=%d, heavy=%d", scoreLight, scoreHeavy)
	}
}

func TestCapabilityMatch(t *testing.T) {
	tests := []struct {
		name      string
		podCaps   []string
		taskCaps  []string
		wantScore int
	}{
		{"no required caps", []string{"go"}, nil, 50},
		{"exact match, no extra", []string{"go"}, []string{"go"}, 50},
		{"exact match with extra caps", []string{"go", "docker", "k8s"}, []string{"go"}, 100},
		{"all match plus extras", []string{"go", "docker", "k8s"}, []string{"go", "docker"}, 100},
		{"partial match no extras", []string{"go"}, []string{"go", "docker"}, 25},
		{"no match", []string{"python"}, []string{"go"}, 0},
		{"both empty", nil, nil, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").capabilities(tt.podCaps...).build()
			task := newTask("t1", "proj").requiredCapabilities(tt.taskCaps...).build()
			got := CapabilityMatch(pod, task)
			if got != tt.wantScore {
				t.Errorf("CapabilityMatch(pod=%v, task=%v) = %d, want %d",
					tt.podCaps, tt.taskCaps, got, tt.wantScore)
			}
		})
	}

	// Verify relative ordering: better match -> higher score.
	task := newTask("t1", "proj").requiredCapabilities("go", "docker").build()
	podFull := newPod("full", "proj").capabilities("go", "docker", "k8s").build()
	podPartial := newPod("partial", "proj").capabilities("go").build()
	scoreFull := CapabilityMatch(podFull, task)
	scorePartial := CapabilityMatch(podPartial, task)
	if scoreFull <= scorePartial {
		t.Errorf("expected full-match pod to score higher: full=%d, partial=%d", scoreFull, scorePartial)
	}
}

func TestModelPreference(t *testing.T) {
	tests := []struct {
		name      string
		podModel  string
		taskModel string
		wantScore int
	}{
		{"exact match", "claude-3", "claude-3", 100},
		{"no preference", "claude-3", "", 50},
		{"mismatch", "claude-3", "gpt-4", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("p1", "proj").model(tt.podModel).build()
			task := newTask("t1", "proj").preferredModel(tt.taskModel).build()
			got := ModelPreference(pod, task)
			if got != tt.wantScore {
				t.Errorf("ModelPreference(pod=%q, task=%q) = %d, want %d",
					tt.podModel, tt.taskModel, got, tt.wantScore)
			}
		})
	}
}

// =========================================================================
// Scheduler integration tests (using MemoryStore)
// =========================================================================

func newTestScheduler(t *testing.T) (*Scheduler, store.Store) {
	t.Helper()
	s := store.NewMemoryStore()
	logger := zap.NewNop()
	sched := NewScheduler(s, logger)
	return sched, s
}

func TestScheduleSuccess(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Pod A: ready, light load, has required capabilities, model matches.
	podA := newPod("pod-a", "proj").
		model("claude-3").
		capabilities("go", "docker").
		maxConcurrency(10).
		activeTasks(1).
		build()

	// Pod B: ready, heavier load, same caps and model.
	podB := newPod("pod-b", "proj").
		model("claude-3").
		capabilities("go", "docker").
		maxConcurrency(10).
		activeTasks(8).
		build()

	addPodToStore(t, s, podA)
	addPodToStore(t, s, podB)

	task := newTask("task-1", "proj").
		requiredCapabilities("go").
		preferredModel("claude-3").
		build()

	best, err := sched.Schedule(task)
	if err != nil {
		t.Fatalf("Schedule() returned unexpected error: %v", err)
	}

	// Pod A should win because it has fewer active tasks (higher LeastLoaded score).
	if best.Metadata.Name != "pod-a" {
		t.Errorf("Schedule() selected %q, want %q", best.Metadata.Name, "pod-a")
	}
}

func TestScheduleSelectsBestByModelPreference(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Pod A: model matches.
	podA := newPod("pod-a", "proj").
		model("claude-3").
		capabilities("go").
		maxConcurrency(10).
		activeTasks(5).
		build()

	// Pod B: model does NOT match, but lighter load.
	// This pod will be filtered out by the PodMatchesModel predicate.
	podB := newPod("pod-b", "proj").
		model("gpt-4").
		capabilities("go").
		maxConcurrency(10).
		activeTasks(0).
		build()

	addPodToStore(t, s, podA)
	addPodToStore(t, s, podB)

	task := newTask("task-1", "proj").
		requiredCapabilities("go").
		preferredModel("claude-3").
		build()

	best, err := sched.Schedule(task)
	if err != nil {
		t.Fatalf("Schedule() returned unexpected error: %v", err)
	}

	if best.Metadata.Name != "pod-a" {
		t.Errorf("Schedule() selected %q, want %q (model match)", best.Metadata.Name, "pod-a")
	}
}

func TestScheduleNoPods(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	task := newTask("task-1", "proj").build()

	_, err := sched.Schedule(task)
	if err == nil {
		t.Fatal("Schedule() expected error for empty store, got nil")
	}
}

func TestScheduleNoMatchingPods(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Pod exists but is not ready.
	podFailed := newPod("pod-failed", "proj").
		phase(v1alpha1.PodFailed).
		build()
	addPodToStore(t, s, podFailed)

	// Pod exists but wrong project.
	podWrongProj := newPod("pod-wrong", "other-proj").
		build()
	addPodToStore(t, s, podWrongProj)

	// Pod exists but lacks required capability.
	podNoCap := newPod("pod-nocap", "proj").
		capabilities("python").
		build()
	addPodToStore(t, s, podNoCap)

	task := newTask("task-1", "proj").
		requiredCapabilities("go").
		build()

	_, err := sched.Schedule(task)
	if err == nil {
		t.Fatal("Schedule() expected error when no pods match predicates, got nil")
	}
}

func TestScheduleNoMatchingPods_FullCapacity(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Pod is ready and in the right project, but at full capacity.
	podFull := newPod("pod-full", "proj").
		maxConcurrency(2).
		activeTasks(2).
		build()
	addPodToStore(t, s, podFull)

	task := newTask("task-1", "proj").build()

	_, err := sched.Schedule(task)
	if err == nil {
		t.Fatal("Schedule() expected error when only pod is at full capacity, got nil")
	}
}

func TestScheduleNoMatchingPods_ModelMismatch(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Pod is ready but uses a different model than the task prefers.
	pod := newPod("pod-wrong-model", "proj").
		model("gpt-4").
		build()
	addPodToStore(t, s, pod)

	task := newTask("task-1", "proj").
		preferredModel("claude-3").
		build()

	_, err := sched.Schedule(task)
	if err == nil {
		t.Fatal("Schedule() expected error when pod model does not match task preference, got nil")
	}
}

func TestScheduleMultipleFeasiblePods(t *testing.T) {
	sched, s := newTestScheduler(t)
	defer s.Close()

	// Three pods, all feasible but with different load profiles.
	// Pod C should win: lightest load.
	podA := newPod("pod-a", "proj").
		model("claude-3").
		capabilities("go").
		maxConcurrency(10).
		activeTasks(5).
		build()

	podB := newPod("pod-b", "proj").
		model("claude-3").
		capabilities("go").
		maxConcurrency(10).
		activeTasks(3).
		build()

	podC := newPod("pod-c", "proj").
		model("claude-3").
		capabilities("go").
		maxConcurrency(10).
		activeTasks(0).
		build()

	addPodToStore(t, s, podA)
	addPodToStore(t, s, podB)
	addPodToStore(t, s, podC)

	task := newTask("task-1", "proj").
		requiredCapabilities("go").
		preferredModel("claude-3").
		build()

	best, err := sched.Schedule(task)
	if err != nil {
		t.Fatalf("Schedule() returned unexpected error: %v", err)
	}

	if best.Metadata.Name != "pod-c" {
		t.Errorf("Schedule() selected %q, want %q (lightest load)", best.Metadata.Name, "pod-c")
	}
}
