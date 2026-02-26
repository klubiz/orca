package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/klubi/orca/internal/config"
	"github.com/klubi/orca/internal/store"
	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// Runtime manages the lifecycle of AgentPods and coordinates task
// execution via the Claude API Executor.
type Runtime struct {
	store    store.Store
	executor *Executor
	cfg      *config.Config
	logger   *zap.Logger
	mu       sync.Mutex
	// active tracks running agent goroutines by pod name.
	active map[string]context.CancelFunc
}

// NewRuntime creates a new agent Runtime.
func NewRuntime(s store.Store, executor *Executor, cfg *config.Config, logger *zap.Logger) *Runtime {
	return &Runtime{
		store:    s,
		executor: executor,
		cfg:      cfg,
		logger:   logger,
		active:   make(map[string]context.CancelFunc),
	}
}

// StartPod transitions an AgentPod from Pending to Ready.
// It updates the pod's phase through Starting -> Ready and records
// the start time and initial heartbeat.
func (r *Runtime) StartPod(ctx context.Context, pod *v1alpha1.AgentPod) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := store.ResourceKey(v1alpha1.KindAgentPod, pod.Metadata.Project, pod.Metadata.Name)

	r.logger.Info("starting pod",
		zap.String("pod", pod.Metadata.Name),
		zap.String("project", pod.Metadata.Project),
	)

	// Transition to Starting
	pod.Status.Phase = v1alpha1.PodStarting
	pod.Status.Message = "Initializing agent context"
	pod.Metadata.UpdatedAt = time.Now()
	if err := r.store.Update(key, pod); err != nil {
		return fmt.Errorf("failed to set pod Starting: %w", err)
	}

	// In a full implementation, this is where we would initialize the
	// agent's working directory, load tools, validate API keys, etc.

	// Create a cancellable context for this pod's lifetime
	_, cancel := context.WithCancel(ctx)
	r.active[pod.Metadata.Name] = cancel

	// Transition to Ready
	now := time.Now()
	pod.Status.Phase = v1alpha1.PodReady
	pod.Status.Message = ""
	pod.Status.StartedAt = now
	pod.Status.LastHeartbeat = now
	pod.Metadata.UpdatedAt = now
	if err := r.store.Update(key, pod); err != nil {
		return fmt.Errorf("failed to set pod Ready: %w", err)
	}

	r.logger.Info("pod is ready",
		zap.String("pod", pod.Metadata.Name),
		zap.String("model", pod.Spec.Model),
	)

	return nil
}

// StopPod gracefully terminates an AgentPod by cancelling its context
// and transitioning it through Terminating -> Terminated.
func (r *Runtime) StopPod(ctx context.Context, podName, project string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := store.ResourceKey(v1alpha1.KindAgentPod, project, podName)

	r.logger.Info("stopping pod",
		zap.String("pod", podName),
		zap.String("project", project),
	)

	// Load current pod state
	var pod v1alpha1.AgentPod
	if err := r.store.Get(key, &pod); err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// Transition to Terminating
	pod.Status.Phase = v1alpha1.PodTerminating
	pod.Status.Message = "Shutting down"
	pod.Metadata.UpdatedAt = time.Now()
	if err := r.store.Update(key, &pod); err != nil {
		return fmt.Errorf("failed to set pod Terminating: %w", err)
	}

	// Cancel the pod's context if it exists
	if cancel, ok := r.active[podName]; ok {
		cancel()
		delete(r.active, podName)
	}

	// Transition to Terminated
	pod.Status.Phase = v1alpha1.PodTerminated
	pod.Status.Message = "Stopped"
	pod.Metadata.UpdatedAt = time.Now()
	if err := r.store.Update(key, &pod); err != nil {
		return fmt.Errorf("failed to set pod Terminated: %w", err)
	}

	r.logger.Info("pod terminated", zap.String("pod", podName))

	return nil
}

// ExecuteTask runs a DevTask on a specific AgentPod by calling the
// Claude API through the Executor. It manages all state transitions
// for both the task and the pod through the store.
func (r *Runtime) ExecuteTask(ctx context.Context, task *v1alpha1.DevTask, pod *v1alpha1.AgentPod) error {
	taskKey := store.ResourceKey(v1alpha1.KindDevTask, task.Metadata.Project, task.Metadata.Name)
	podKey := store.ResourceKey(v1alpha1.KindAgentPod, pod.Metadata.Project, pod.Metadata.Name)

	r.logger.Info("executing task",
		zap.String("task", task.Metadata.Name),
		zap.String("pod", pod.Metadata.Name),
	)

	now := time.Now()

	// Mark task as Running
	task.Status.Phase = v1alpha1.TaskRunning
	task.Status.AssignedPod = pod.Metadata.Name
	task.Status.StartedAt = now
	task.Metadata.UpdatedAt = now
	if err := r.store.Update(taskKey, task); err != nil {
		return fmt.Errorf("failed to set task Running: %w", err)
	}

	// Mark pod as Busy and increment ActiveTasks
	r.mu.Lock()
	pod.Status.Phase = v1alpha1.PodBusy
	pod.Status.ActiveTasks++
	pod.Metadata.UpdatedAt = now
	if err := r.store.Update(podKey, pod); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to set pod Busy: %w", err)
	}
	r.mu.Unlock()

	// Build the execution request
	model := pod.Spec.Model
	if task.Spec.PreferredModel != "" {
		model = task.Spec.PreferredModel
	}

	maxTokens := pod.Spec.MaxTokens
	if maxTokens == 0 {
		maxTokens = r.cfg.Agent.DefaultMaxTokens
	}

	req := ExecutionRequest{
		Model:        model,
		SystemPrompt: pod.Spec.SystemPrompt,
		Prompt:       task.Spec.Prompt,
		MaxTokens:    maxTokens,
	}

	// Call the Claude API
	result, err := r.executor.Execute(ctx, req)

	finishedAt := time.Now()

	// Update task status based on the result
	if err != nil {
		r.logger.Error("task execution failed",
			zap.String("task", task.Metadata.Name),
			zap.Error(err),
		)
		task.Status.Phase = v1alpha1.TaskFailed
		task.Status.Error = err.Error()
		task.Status.FinishedAt = finishedAt
		task.Metadata.UpdatedAt = finishedAt
	} else {
		r.logger.Info("task execution succeeded",
			zap.String("task", task.Metadata.Name),
			zap.Int("tokensIn", result.TokensIn),
			zap.Int("tokensOut", result.TokensOut),
		)
		task.Status.Phase = v1alpha1.TaskSucceeded
		task.Status.Output = result.Output
		task.Status.FinishedAt = finishedAt
		task.Metadata.UpdatedAt = finishedAt
	}

	r.logger.Debug("writing task result to store",
		zap.String("task", task.Metadata.Name),
		zap.String("phase", string(task.Status.Phase)),
		zap.Int("outputLen", len(task.Status.Output)),
	)

	if storeErr := r.store.Update(taskKey, task); storeErr != nil {
		return fmt.Errorf("failed to update task status: %w", storeErr)
	}

	// Return pod to Ready and update counters
	r.mu.Lock()
	defer r.mu.Unlock()

	pod.Status.Phase = v1alpha1.PodReady
	pod.Status.ActiveTasks--
	if err != nil {
		pod.Status.FailedTasks++
	} else {
		pod.Status.CompletedTasks++
	}
	pod.Metadata.UpdatedAt = finishedAt
	if storeErr := r.store.Update(podKey, pod); storeErr != nil {
		return fmt.Errorf("failed to update pod status: %w", storeErr)
	}

	return nil
}

// Heartbeat updates the pod's last heartbeat timestamp in the store.
func (r *Runtime) Heartbeat(podName, project string) error {
	key := store.ResourceKey(v1alpha1.KindAgentPod, project, podName)

	var pod v1alpha1.AgentPod
	if err := r.store.Get(key, &pod); err != nil {
		return fmt.Errorf("failed to get pod for heartbeat: %w", err)
	}

	now := time.Now()
	pod.Status.LastHeartbeat = now
	pod.Metadata.UpdatedAt = now

	if err := r.store.Update(key, &pod); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return nil
}

// IsActive checks whether a pod is actively managed by this runtime.
func (r *Runtime) IsActive(podName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.active[podName]
	return ok
}
