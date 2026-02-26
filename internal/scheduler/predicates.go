package scheduler

import v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"

// Predicate is a filter function that returns true if a pod can accept the task.
type Predicate func(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool

// PodIsReady checks that the pod is in Ready phase (not Busy, Failed, etc.).
func PodIsReady(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool {
	return pod.Status.Phase == v1alpha1.PodReady
}

// PodHasCapacity checks that pod's ActiveTasks < MaxConcurrency.
// If MaxConcurrency is 0 or unset, treat as 1.
func PodHasCapacity(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool {
	max := pod.Spec.MaxConcurrency
	if max <= 0 {
		max = 1
	}
	return pod.Status.ActiveTasks < max
}

// PodMatchesCapability checks that the pod has all required capabilities of the task.
// If the task has no required capabilities, any pod matches.
func PodMatchesCapability(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool {
	if len(task.Spec.RequiredCapabilities) == 0 {
		return true
	}

	podCaps := make(map[string]struct{}, len(pod.Spec.Capabilities))
	for _, c := range pod.Spec.Capabilities {
		podCaps[c] = struct{}{}
	}

	for _, req := range task.Spec.RequiredCapabilities {
		if _, ok := podCaps[req]; !ok {
			return false
		}
	}
	return true
}

// PodMatchesModel checks that the pod's model matches the task's preferred model.
// If the task has no preferred model, any pod matches.
func PodMatchesModel(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool {
	if task.Spec.PreferredModel == "" {
		return true
	}
	return pod.Spec.Model == task.Spec.PreferredModel
}

// PodInSameProject checks that the pod's project matches the task's project.
func PodInSameProject(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) bool {
	return pod.Metadata.Project == task.Metadata.Project
}
