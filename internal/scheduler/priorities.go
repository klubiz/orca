package scheduler

import v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"

// PriorityFunc scores a pod for a task. Higher score = better match. Range: 0-100.
type PriorityFunc func(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) int

// LeastLoaded gives higher score to pods with fewer active tasks.
// Score = 100 - (activeTasks * 100 / maxConcurrency).
// If maxConcurrency is 0, treat as 1.
func LeastLoaded(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) int {
	max := pod.Spec.MaxConcurrency
	if max <= 0 {
		max = 1
	}

	active := pod.Status.ActiveTasks
	if active >= max {
		return 0
	}

	return 100 - (active * 100 / max)
}

// CapabilityMatch gives higher score for more matching capabilities.
// Score = (matchingCapabilities / totalRequiredCapabilities) * 50
// Plus bonus 50 if pod has extra capabilities beyond required.
// If no required capabilities, score = 50.
func CapabilityMatch(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) int {
	required := task.Spec.RequiredCapabilities
	if len(required) == 0 {
		return 50
	}

	podCaps := make(map[string]struct{}, len(pod.Spec.Capabilities))
	for _, c := range pod.Spec.Capabilities {
		podCaps[c] = struct{}{}
	}

	matching := 0
	for _, req := range required {
		if _, ok := podCaps[req]; ok {
			matching++
		}
	}

	score := matching * 50 / len(required)

	// Bonus 50 if the pod has capabilities beyond the required set.
	if len(pod.Spec.Capabilities) > len(required) {
		score += 50
	}

	if score > 100 {
		score = 100
	}
	return score
}

// ModelPreference gives 100 if model matches exactly, 50 if no preference, 0 if mismatch.
func ModelPreference(pod *v1alpha1.AgentPod, task *v1alpha1.DevTask) int {
	if task.Spec.PreferredModel == "" {
		return 50
	}
	if pod.Spec.Model == task.Spec.PreferredModel {
		return 100
	}
	return 0
}
