package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/internal/agent"
	"github.com/klubi/orca/internal/store"
	"go.uber.org/zap"
)

// AgentPoolController manages the desired replica count for agent pools.
type AgentPoolController struct {
	store   store.Store
	runtime *agent.Runtime
	logger  *zap.Logger
}

// NewAgentPoolController creates a new AgentPoolController.
func NewAgentPoolController(s store.Store, rt *agent.Runtime, logger *zap.Logger) *AgentPoolController {
	return &AgentPoolController{
		store:   s,
		runtime: rt,
		logger:  logger,
	}
}

// Reconcile ensures the number of AgentPods matches the pool's desired replicas.
//
//  1. Get the AgentPool from the key.
//  2. List all AgentPods with matching ownerPool.
//  3. If actual < desired: create new pods.
//  4. If actual > desired: mark excess pods for termination.
//  5. Update pool status (Replicas, ReadyReplicas, BusyReplicas counts).
func (c *AgentPoolController) Reconcile(ctx context.Context, key string) error {
	// If we received an AgentPod event, find its owner pool and reconcile that instead.
	if strings.HasPrefix(key, "/"+v1alpha1.KindAgentPod+"/") {
		return c.reconcileFromPodEvent(ctx, key)
	}

	// 1. Get the AgentPool from the store.
	var pool v1alpha1.AgentPool
	if err := c.store.Get(key, &pool); err != nil {
		if err == store.ErrNotFound {
			c.logger.Debug("pool not found, possibly deleted", zap.String("key", key))
			return nil
		}
		return fmt.Errorf("getting pool %q: %w", key, err)
	}

	c.logger.Debug("reconciling agent pool",
		zap.String("pool", pool.Metadata.Name),
		zap.Int("desiredReplicas", pool.Spec.Replicas),
	)

	// 2. List all AgentPods in the same project.
	prefix := fmt.Sprintf("/%s/%s/", v1alpha1.KindAgentPod, pool.Metadata.Project)
	objects, err := c.store.List(prefix, func() interface{} {
		return &v1alpha1.AgentPod{}
	})
	if err != nil {
		return fmt.Errorf("listing pods for pool %q: %w", pool.Metadata.Name, err)
	}

	// Filter pods owned by this pool, excluding terminated/terminating ones.
	// Terminating pods are already on their way out and should not count
	// towards the actual replica count for scaling decisions.
	var ownedPods []*v1alpha1.AgentPod
	for _, obj := range objects {
		pod, ok := obj.(*v1alpha1.AgentPod)
		if !ok {
			continue
		}
		if pod.Spec.OwnerPool == pool.Metadata.Name &&
			pod.Status.Phase != v1alpha1.PodTerminated &&
			pod.Status.Phase != v1alpha1.PodTerminating {
			ownedPods = append(ownedPods, pod)
		}
	}

	actual := len(ownedPods)
	desired := pool.Spec.Replicas

	c.logger.Debug("pool replica count",
		zap.String("pool", pool.Metadata.Name),
		zap.Int("actual", actual),
		zap.Int("desired", desired),
	)

	// 3. Scale up: create new pods if actual < desired.
	if actual < desired {
		toCreate := desired - actual
		for i := 0; i < toCreate; i++ {
			if err := c.createPod(ctx, &pool); err != nil {
				return fmt.Errorf("creating pod for pool %q: %w", pool.Metadata.Name, err)
			}
		}
		c.logger.Info("scaled up pool",
			zap.String("pool", pool.Metadata.Name),
			zap.Int("created", toCreate),
		)
	}

	// 4. Scale down: mark excess pods for termination if actual > desired.
	if actual > desired {
		toTerminate := actual - desired
		terminated := 0
		// Prefer terminating pods that are not busy.
		for _, pod := range ownedPods {
			if terminated >= toTerminate {
				break
			}
			// Prefer terminating non-busy pods first.
			if pod.Status.Phase != v1alpha1.PodBusy {
				pod.Status.Phase = v1alpha1.PodTerminating
				pod.Status.Message = "scaling down"
				podKey := store.ResourceKey(v1alpha1.KindAgentPod, pod.Metadata.Project, pod.Metadata.Name)
				if err := c.store.Update(podKey, pod); err != nil {
					return fmt.Errorf("terminating pod %q: %w", pod.Metadata.Name, err)
				}
				terminated++
			}
		}
		// If we still need to terminate more, terminate busy pods.
		if terminated < toTerminate {
			for _, pod := range ownedPods {
				if terminated >= toTerminate {
					break
				}
				if pod.Status.Phase == v1alpha1.PodBusy {
					pod.Status.Phase = v1alpha1.PodTerminating
					pod.Status.Message = "scaling down"
					podKey := store.ResourceKey(v1alpha1.KindAgentPod, pod.Metadata.Project, pod.Metadata.Name)
					if err := c.store.Update(podKey, pod); err != nil {
						return fmt.Errorf("terminating pod %q: %w", pod.Metadata.Name, err)
					}
					terminated++
				}
			}
		}
		c.logger.Info("scaled down pool",
			zap.String("pool", pool.Metadata.Name),
			zap.Int("terminated", terminated),
		)
	}

	// 5. Update pool status with current counts.
	// Re-list pods to get accurate counts after mutations.
	objects, err = c.store.List(prefix, func() interface{} {
		return &v1alpha1.AgentPod{}
	})
	if err != nil {
		return fmt.Errorf("re-listing pods for pool %q status: %w", pool.Metadata.Name, err)
	}

	var replicas, ready, busy int
	for _, obj := range objects {
		pod, ok := obj.(*v1alpha1.AgentPod)
		if !ok {
			continue
		}
		if pod.Spec.OwnerPool != pool.Metadata.Name {
			continue
		}
		if pod.Status.Phase == v1alpha1.PodTerminated || pod.Status.Phase == v1alpha1.PodTerminating {
			continue
		}
		replicas++
		switch pod.Status.Phase {
		case v1alpha1.PodReady:
			ready++
		case v1alpha1.PodBusy:
			busy++
		}
	}

	// Re-read pool from store to pick up any Spec changes (e.g. scale API)
	// that occurred while we were reconciling. Only update Status on the fresh copy.
	var freshPool v1alpha1.AgentPool
	if err := c.store.Get(key, &freshPool); err != nil {
		if err == store.ErrNotFound {
			return nil
		}
		return fmt.Errorf("re-reading pool %q for status update: %w", pool.Metadata.Name, err)
	}

	// Only write if status actually changed to avoid an infinite event loop
	// (each Update triggers a MODIFIED event which re-triggers Reconcile).
	if freshPool.Status.Replicas == replicas &&
		freshPool.Status.ReadyReplicas == ready &&
		freshPool.Status.BusyReplicas == busy {
		return nil
	}

	freshPool.Status.Replicas = replicas
	freshPool.Status.ReadyReplicas = ready
	freshPool.Status.BusyReplicas = busy

	if err := c.store.Update(key, &freshPool); err != nil {
		return fmt.Errorf("updating pool %q status: %w", pool.Metadata.Name, err)
	}

	return nil
}

// reconcileFromPodEvent handles AgentPod events by finding the owner pool
// and delegating to the main Reconcile method. This avoids a TOCTOU race
// where a separate read-modify-write could overwrite Spec changes (e.g. replicas).
func (c *AgentPoolController) reconcileFromPodEvent(ctx context.Context, podKey string) error {
	var pod v1alpha1.AgentPod
	if err := c.store.Get(podKey, &pod); err != nil {
		if err == store.ErrNotFound {
			return nil
		}
		return err
	}
	if pod.Spec.OwnerPool == "" {
		return nil // Standalone pod, not managed by a pool.
	}
	// Delegate to the main Reconcile with the pool key so that
	// scale up/down and status update happen atomically on the latest state.
	poolKey := store.ResourceKey(v1alpha1.KindAgentPool, pod.Metadata.Project, pod.Spec.OwnerPool)
	return c.Reconcile(ctx, poolKey)
}

// createPod creates a new AgentPod from the pool's template.
func (c *AgentPoolController) createPod(_ context.Context, pool *v1alpha1.AgentPool) error {
	// Generate a short random suffix from UUID (first 8 chars).
	suffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	podName := fmt.Sprintf("%s-%s", pool.Metadata.Name, suffix)

	// Merge labels: pool selector labels + template labels.
	labels := make(map[string]string)
	for k, v := range pool.Spec.Selector {
		labels[k] = v
	}
	for k, v := range pool.Spec.Template.Metadata.Labels {
		labels[k] = v
	}

	pod := &v1alpha1.AgentPod{
		TypeMeta: v1alpha1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.KindAgentPod,
		},
		Metadata: v1alpha1.ObjectMeta{
			Name:      podName,
			Project:   pool.Metadata.Project,
			Labels:    labels,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Spec: v1alpha1.AgentPodSpec{
			Model:          pool.Spec.Template.Spec.Model,
			SystemPrompt:   pool.Spec.Template.Spec.SystemPrompt,
			Capabilities:   pool.Spec.Template.Spec.Capabilities,
			MaxConcurrency: pool.Spec.Template.Spec.MaxConcurrency,
			MaxTokens:      pool.Spec.Template.Spec.MaxTokens,
			Tools:          pool.Spec.Template.Spec.Tools,
			RestartPolicy:  pool.Spec.Template.Spec.RestartPolicy,
			OwnerPool:      pool.Metadata.Name,
		},
		Status: v1alpha1.AgentPodStatus{
			Phase: v1alpha1.PodPending,
		},
	}

	podKey := store.ResourceKey(v1alpha1.KindAgentPod, pool.Metadata.Project, podName)
	if err := c.store.Create(podKey, pod); err != nil {
		return fmt.Errorf("creating pod %q: %w", podName, err)
	}

	c.logger.Debug("created pod from pool template",
		zap.String("pod", podName),
		zap.String("pool", pool.Metadata.Name),
	)

	// Start the pod to transition it to Ready.
	if c.runtime != nil {
		go func() {
			if err := c.runtime.StartPod(context.Background(), pod); err != nil {
				c.logger.Error("failed to start pod",
					zap.String("pod", podName),
					zap.Error(err),
				)
			}
		}()
	}

	return nil
}
