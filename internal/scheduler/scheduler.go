package scheduler

import (
	"fmt"
	"sort"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// Scheduler assigns DevTasks to AgentPods using Kubernetes-style
// predicate filtering and priority scoring.
type Scheduler struct {
	store      store.Store
	predicates []Predicate
	priorities []PriorityFunc
	logger     *zap.Logger
}

// scoreResult holds a pod and its total priority score.
type scoreResult struct {
	pod   *v1alpha1.AgentPod
	score int
}

// NewScheduler creates a Scheduler with default predicates and priorities.
func NewScheduler(s store.Store, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		store: s,
		predicates: []Predicate{
			PodInSameProject,
			PodIsReady,
			PodHasCapacity,
			PodMatchesCapability,
			PodMatchesModel,
		},
		priorities: []PriorityFunc{
			LeastLoaded,
			CapabilityMatch,
			ModelPreference,
		},
		logger: logger,
	}
}

// Schedule finds the best pod for a task.
//
//  1. List all AgentPods in the task's project.
//  2. Filter through all predicates (pod must pass ALL).
//  3. Score remaining pods through all priorities (sum scores).
//  4. Sort by total score descending.
//  5. Return the highest-scoring pod.
//
// Returns an error if no suitable pod is found.
func (s *Scheduler) Schedule(task *v1alpha1.DevTask) (*v1alpha1.AgentPod, error) {
	// 1. List all AgentPods in the task's project.
	prefix := fmt.Sprintf("/%s/%s/", v1alpha1.KindAgentPod, task.Metadata.Project)
	objects, err := s.store.List(prefix, func() interface{} {
		return &v1alpha1.AgentPod{}
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods for project %q: %w", task.Metadata.Project, err)
	}

	s.logger.Debug("scheduler: listed pods",
		zap.String("project", task.Metadata.Project),
		zap.Int("total", len(objects)),
	)

	// 2. Filter through all predicates.
	var feasible []*v1alpha1.AgentPod
	for _, obj := range objects {
		pod, ok := obj.(*v1alpha1.AgentPod)
		if !ok {
			continue
		}

		passed := true
		for _, pred := range s.predicates {
			if !pred(pod, task) {
				passed = false
				break
			}
		}
		if passed {
			feasible = append(feasible, pod)
		}
	}

	s.logger.Debug("scheduler: predicates applied",
		zap.Int("feasible", len(feasible)),
	)

	if len(feasible) == 0 {
		return nil, fmt.Errorf("no suitable pod found for task %q in project %q",
			task.Metadata.Name, task.Metadata.Project)
	}

	// 3. Score remaining pods through all priorities.
	results := make([]scoreResult, len(feasible))
	for i, pod := range feasible {
		total := 0
		for _, pf := range s.priorities {
			total += pf(pod, task)
		}
		results[i] = scoreResult{pod: pod, score: total}
	}

	// 4. Sort by total score descending.
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	best := results[0]
	s.logger.Info("scheduler: pod selected",
		zap.String("task", task.Metadata.Name),
		zap.String("pod", best.pod.Metadata.Name),
		zap.Int("score", best.score),
	)

	// 5. Return the highest-scoring pod.
	return best.pod, nil
}
