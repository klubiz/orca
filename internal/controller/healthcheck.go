package controller

import (
	"context"
	"fmt"
	"time"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/agent"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// HealthCheckController monitors agent pod health via heartbeats.
type HealthCheckController struct {
	store    store.Store
	runtime  *agent.Runtime
	interval time.Duration
	logger   *zap.Logger
}

// NewHealthCheckController creates a new HealthCheckController.
// The interval defines the expected heartbeat frequency. A pod is considered
// unhealthy if its last heartbeat is older than 3x the interval.
func NewHealthCheckController(s store.Store, rt *agent.Runtime, interval time.Duration, logger *zap.Logger) *HealthCheckController {
	return &HealthCheckController{
		store:    s,
		runtime:  rt,
		interval: interval,
		logger:   logger,
	}
}

// Reconcile checks pod health:
//
//  1. Get the AgentPod from the key.
//  2. If pod is Ready or Busy:
//     - Check LastHeartbeat. If older than 3x interval, mark as Failed.
//     - Otherwise, pod is healthy.
//  3. If pod is Failed and RestartPolicy is "Always":
//     - Reset to Pending for restart.
func (c *HealthCheckController) Reconcile(ctx context.Context, key string) error {
	var pod v1alpha1.AgentPod
	if err := c.store.Get(key, &pod); err != nil {
		if err == store.ErrNotFound {
			c.logger.Debug("pod not found, possibly deleted", zap.String("key", key))
			return nil
		}
		return fmt.Errorf("getting pod %q: %w", key, err)
	}

	c.logger.Debug("health check",
		zap.String("pod", pod.Metadata.Name),
		zap.String("phase", string(pod.Status.Phase)),
	)

	switch pod.Status.Phase {
	case v1alpha1.PodReady, v1alpha1.PodBusy:
		return c.checkHeartbeat(key, &pod)

	case v1alpha1.PodFailed:
		return c.checkRestart(key, &pod)

	default:
		// Other phases (Pending, Starting, Terminating, Terminated): no action.
		return nil
	}
}

// checkHeartbeat verifies the pod's last heartbeat is within the acceptable threshold.
// If the heartbeat is older than 3x the configured interval, the pod is marked as Failed.
func (c *HealthCheckController) checkHeartbeat(key string, pod *v1alpha1.AgentPod) error {
	threshold := 3 * c.interval
	deadline := time.Now().Add(-threshold)

	if pod.Status.LastHeartbeat.IsZero() {
		// No heartbeat recorded yet. If the pod has been running for longer
		// than the threshold, mark it as failed.
		if !pod.Status.StartedAt.IsZero() && pod.Status.StartedAt.Before(deadline) {
			return c.markFailed(key, pod, "no heartbeat received since start")
		}
		// Pod just started; give it time.
		return nil
	}

	if pod.Status.LastHeartbeat.Before(deadline) {
		elapsed := time.Since(pod.Status.LastHeartbeat)
		c.logger.Warn("pod heartbeat expired",
			zap.String("pod", pod.Metadata.Name),
			zap.Duration("elapsed", elapsed),
			zap.Duration("threshold", threshold),
		)
		return c.markFailed(key, pod, fmt.Sprintf("heartbeat expired: last seen %s ago", elapsed.Round(time.Second)))
	}

	// Pod is healthy.
	c.logger.Debug("pod healthy",
		zap.String("pod", pod.Metadata.Name),
		zap.Time("lastHeartbeat", pod.Status.LastHeartbeat),
	)
	return nil
}

// markFailed transitions a pod to the Failed phase.
func (c *HealthCheckController) markFailed(key string, pod *v1alpha1.AgentPod, message string) error {
	pod.Status.Phase = v1alpha1.PodFailed
	pod.Status.Message = message
	pod.Metadata.UpdatedAt = time.Now()

	if err := c.store.Update(key, pod); err != nil {
		return fmt.Errorf("marking pod %q as Failed: %w", pod.Metadata.Name, err)
	}

	c.logger.Info("pod marked as failed",
		zap.String("pod", pod.Metadata.Name),
		zap.String("reason", message),
	)

	return nil
}

// checkRestart resets a Failed pod to Pending if its RestartPolicy is "Always".
func (c *HealthCheckController) checkRestart(key string, pod *v1alpha1.AgentPod) error {
	if pod.Spec.RestartPolicy != "Always" {
		c.logger.Debug("pod failed but restart policy is not Always",
			zap.String("pod", pod.Metadata.Name),
			zap.String("restartPolicy", pod.Spec.RestartPolicy),
		)
		return nil
	}

	c.logger.Info("restarting failed pod",
		zap.String("pod", pod.Metadata.Name),
	)

	pod.Status.Phase = v1alpha1.PodPending
	pod.Status.Message = "restarting after failure"
	pod.Status.ActiveTasks = 0
	pod.Metadata.UpdatedAt = time.Now()

	if err := c.store.Update(key, pod); err != nil {
		return fmt.Errorf("resetting pod %q to Pending: %w", pod.Metadata.Name, err)
	}

	return nil
}
