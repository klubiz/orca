package controller

import (
	"context"
	"fmt"
	"strings"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/agent"
	"github.com/klubi/orca/internal/scheduler"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// DevTaskController manages the task lifecycle.
type DevTaskController struct {
	store     store.Store
	scheduler *scheduler.Scheduler
	runtime   *agent.Runtime
	logger    *zap.Logger
}

// NewDevTaskController creates a new DevTaskController.
func NewDevTaskController(s store.Store, sched *scheduler.Scheduler, rt *agent.Runtime, logger *zap.Logger) *DevTaskController {
	return &DevTaskController{
		store:     s,
		scheduler: sched,
		runtime:   rt,
		logger:    logger,
	}
}

// Reconcile manages the task lifecycle:
//
//   - Pending:   Check dependencies, schedule if satisfied.
//   - Scheduled: Launch runtime.ExecuteTask() in a goroutine.
//   - Failed:    Retry if retries < maxRetries.
//   - Succeeded/Running: No action needed.
func (c *DevTaskController) Reconcile(ctx context.Context, key string) error {
	// If we received an AgentPod event, check if any pending tasks can now be scheduled.
	if strings.HasPrefix(key, "/"+v1alpha1.KindAgentPod+"/") {
		return c.reconcileFromPodEvent(ctx, key)
	}

	// 1. Get the DevTask from the store.
	var task v1alpha1.DevTask
	if err := c.store.Get(key, &task); err != nil {
		if err == store.ErrNotFound {
			c.logger.Debug("task not found, possibly deleted", zap.String("key", key))
			return nil
		}
		return fmt.Errorf("getting task %q: %w", key, err)
	}

	c.logger.Debug("reconciling dev task",
		zap.String("task", task.Metadata.Name),
		zap.String("phase", string(task.Status.Phase)),
	)

	switch task.Status.Phase {
	case v1alpha1.TaskPending:
		return c.reconcilePending(ctx, key, &task)

	case v1alpha1.TaskScheduled:
		return c.reconcileScheduled(ctx, key, &task)

	case v1alpha1.TaskFailed:
		return c.reconcileFailed(ctx, key, &task)

	case v1alpha1.TaskRunning, v1alpha1.TaskSucceeded:
		// No action needed.
		return nil

	default:
		// Unknown phase; treat as a no-op with a warning.
		c.logger.Warn("unknown task phase",
			zap.String("task", task.Metadata.Name),
			zap.String("phase", string(task.Status.Phase)),
		)
		return nil
	}
}

// reconcilePending checks dependencies and attempts to schedule the task.
func (c *DevTaskController) reconcilePending(ctx context.Context, key string, task *v1alpha1.DevTask) error {
	// Check dependencies: all dependsOn tasks must be Succeeded.
	if len(task.Spec.DependsOn) > 0 {
		for _, depName := range task.Spec.DependsOn {
			depKey := store.ResourceKey(v1alpha1.KindDevTask, task.Metadata.Project, depName)
			var depTask v1alpha1.DevTask
			if err := c.store.Get(depKey, &depTask); err != nil {
				if err == store.ErrNotFound {
					c.logger.Debug("dependency not found, waiting",
						zap.String("task", task.Metadata.Name),
						zap.String("dependency", depName),
					)
					return nil // Will be retried on next event
				}
				return fmt.Errorf("checking dependency %q for task %q: %w", depName, task.Metadata.Name, err)
			}

			if depTask.Status.Phase != v1alpha1.TaskSucceeded {
				c.logger.Debug("dependency not yet succeeded",
					zap.String("task", task.Metadata.Name),
					zap.String("dependency", depName),
					zap.String("depPhase", string(depTask.Status.Phase)),
				)
				return nil // Wait until dependency completes
			}
		}

		c.logger.Debug("all dependencies satisfied",
			zap.String("task", task.Metadata.Name),
		)
	}

	// Schedule: find a suitable pod.
	pod, err := c.scheduler.Schedule(task)
	if err != nil {
		c.logger.Warn("scheduling failed, will retry",
			zap.String("task", task.Metadata.Name),
			zap.Error(err),
		)
		// Return error to trigger requeue with backoff.
		return fmt.Errorf("scheduling task %q: %w", task.Metadata.Name, err)
	}

	// Transition to Scheduled.
	task.Status.Phase = v1alpha1.TaskScheduled
	task.Status.AssignedPod = pod.Metadata.Name

	if err := c.store.Update(key, task); err != nil {
		return fmt.Errorf("updating task %q to Scheduled: %w", task.Metadata.Name, err)
	}

	c.logger.Info("task scheduled",
		zap.String("task", task.Metadata.Name),
		zap.String("pod", pod.Metadata.Name),
	)

	return nil
}

// reconcileScheduled launches the task on its assigned pod.
func (c *DevTaskController) reconcileScheduled(ctx context.Context, key string, task *v1alpha1.DevTask) error {
	// Get the assigned pod.
	podKey := store.ResourceKey(v1alpha1.KindAgentPod, task.Metadata.Project, task.Status.AssignedPod)
	var pod v1alpha1.AgentPod
	if err := c.store.Get(podKey, &pod); err != nil {
		if err == store.ErrNotFound {
			// Assigned pod disappeared; reset to Pending for rescheduling.
			c.logger.Warn("assigned pod not found, resetting to Pending",
				zap.String("task", task.Metadata.Name),
				zap.String("pod", task.Status.AssignedPod),
			)
			task.Status.Phase = v1alpha1.TaskPending
			task.Status.AssignedPod = ""
			return c.store.Update(key, task)
		}
		return fmt.Errorf("getting assigned pod %q: %w", task.Status.AssignedPod, err)
	}

	c.logger.Info("launching task execution",
		zap.String("task", task.Metadata.Name),
		zap.String("pod", pod.Metadata.Name),
	)

	// Launch execution in a goroutine.
	// The runtime handles all phase transitions:
	//   Running -> Succeeded/Failed for the task
	//   Ready -> Busy -> Ready for the pod
	go func() {
		if err := c.runtime.ExecuteTask(ctx, task, &pod); err != nil {
			c.logger.Error("runtime.ExecuteTask returned error",
				zap.String("task", task.Metadata.Name),
				zap.Error(err),
			)
		}
	}()

	return nil
}

// reconcileFailed checks if the task can be retried.
func (c *DevTaskController) reconcileFailed(_ context.Context, key string, task *v1alpha1.DevTask) error {
	maxRetries := task.Spec.MaxRetries
	if maxRetries <= 0 {
		// No retries configured; leave as Failed.
		return nil
	}

	if task.Status.Retries >= maxRetries {
		c.logger.Info("task exceeded max retries",
			zap.String("task", task.Metadata.Name),
			zap.Int("retries", task.Status.Retries),
			zap.Int("maxRetries", maxRetries),
		)
		return nil
	}

	// Reset to Pending for retry.
	task.Status.Phase = v1alpha1.TaskPending
	task.Status.Retries++
	task.Status.AssignedPod = ""
	task.Status.Error = ""

	if err := c.store.Update(key, task); err != nil {
		return fmt.Errorf("resetting task %q for retry: %w", task.Metadata.Name, err)
	}

	c.logger.Info("task reset for retry",
		zap.String("task", task.Metadata.Name),
		zap.Int("retry", task.Status.Retries),
		zap.Int("maxRetries", maxRetries),
	)

	return nil
}

// reconcileFromPodEvent handles AgentPod events by re-evaluating pending tasks.
// When a pod becomes Ready, pending tasks may now be schedulable.
func (c *DevTaskController) reconcileFromPodEvent(ctx context.Context, podKey string) error {
	// Extract project from key: /AgentPod/{project}/{name}
	parts := strings.Split(strings.TrimPrefix(podKey, "/"), "/")
	if len(parts) < 3 {
		return nil
	}
	project := parts[1]

	// Check if the pod is Ready - only then do we need to check tasks.
	var pod v1alpha1.AgentPod
	if err := c.store.Get(podKey, &pod); err != nil {
		return nil // Pod gone, nothing to do.
	}
	if pod.Status.Phase != v1alpha1.PodReady {
		return nil // Pod not ready, no point scheduling.
	}

	// List all DevTasks in this project.
	prefix := fmt.Sprintf("/%s/%s/", v1alpha1.KindDevTask, project)
	objects, err := c.store.List(prefix, func() interface{} { return &v1alpha1.DevTask{} })
	if err != nil {
		return nil
	}

	// For each pending task, try to reconcile it.
	for _, obj := range objects {
		task, ok := obj.(*v1alpha1.DevTask)
		if !ok || task.Status.Phase != v1alpha1.TaskPending {
			continue
		}
		taskKey := store.ResourceKey(v1alpha1.KindDevTask, project, task.Metadata.Name)
		if err := c.reconcilePending(ctx, taskKey, task); err != nil {
			c.logger.Debug("pending task not yet schedulable",
				zap.String("task", task.Metadata.Name),
				zap.Error(err),
			)
		}
	}

	return nil
}
