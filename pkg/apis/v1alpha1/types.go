// Package v1alpha1 defines all Orca resource types.
package v1alpha1

import "time"

const (
	APIVersion = "orca.dev/v1alpha1"
)

// Resource kinds
const (
	KindProject   = "Project"
	KindAgentPod  = "AgentPod"
	KindAgentPool = "AgentPool"
	KindDevTask   = "DevTask"
)

// TypeMeta describes the API version and kind of a resource.
type TypeMeta struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
}

// ObjectMeta holds metadata common to all resources.
type ObjectMeta struct {
	Name      string            `json:"name" yaml:"name"`
	Project   string            `json:"project,omitempty" yaml:"project,omitempty"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	UID       string            `json:"uid,omitempty" yaml:"uid,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
}

// -------------------------------------------------------
// Project
// -------------------------------------------------------

// Project represents an isolation boundary (like K8s Namespace).
type Project struct {
	TypeMeta `json:",inline" yaml:",inline"`
	Metadata ObjectMeta  `json:"metadata" yaml:"metadata"`
	Spec     ProjectSpec `json:"spec" yaml:"spec"`
	Status   string      `json:"status,omitempty" yaml:"status,omitempty"`
}

type ProjectSpec struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
}

// -------------------------------------------------------
// AgentPod
// -------------------------------------------------------

// AgentPodPhase represents the lifecycle phase of an AgentPod.
type AgentPodPhase string

const (
	PodPending      AgentPodPhase = "Pending"
	PodStarting     AgentPodPhase = "Starting"
	PodReady        AgentPodPhase = "Ready"
	PodBusy         AgentPodPhase = "Busy"
	PodFailed       AgentPodPhase = "Failed"
	PodTerminating  AgentPodPhase = "Terminating"
	PodTerminated   AgentPodPhase = "Terminated"
)

// AgentPod represents a running AI agent instance.
type AgentPod struct {
	TypeMeta `json:",inline" yaml:",inline"`
	Metadata ObjectMeta   `json:"metadata" yaml:"metadata"`
	Spec     AgentPodSpec `json:"spec" yaml:"spec"`
	Status   AgentPodStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type AgentPodSpec struct {
	Model          string   `json:"model" yaml:"model"`
	SystemPrompt   string   `json:"systemPrompt,omitempty" yaml:"systemPrompt,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	MaxConcurrency int      `json:"maxConcurrency,omitempty" yaml:"maxConcurrency,omitempty"`
	MaxTokens      int      `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
	Tools          []string `json:"tools,omitempty" yaml:"tools,omitempty"`
	RestartPolicy  string   `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	// OwnerPool tracks which AgentPool created this pod (empty if standalone).
	OwnerPool string `json:"ownerPool,omitempty" yaml:"ownerPool,omitempty"`
}

type AgentPodStatus struct {
	Phase           AgentPodPhase `json:"phase" yaml:"phase"`
	ActiveTasks     int           `json:"activeTasks" yaml:"activeTasks"`
	CompletedTasks  int           `json:"completedTasks" yaml:"completedTasks"`
	FailedTasks     int           `json:"failedTasks" yaml:"failedTasks"`
	LastHeartbeat   time.Time     `json:"lastHeartbeat,omitempty" yaml:"lastHeartbeat,omitempty"`
	Message         string        `json:"message,omitempty" yaml:"message,omitempty"`
	StartedAt       time.Time     `json:"startedAt,omitempty" yaml:"startedAt,omitempty"`
}

// -------------------------------------------------------
// AgentPool (Deployment equivalent)
// -------------------------------------------------------

// AgentPool manages a group of identical AgentPods.
type AgentPool struct {
	TypeMeta `json:",inline" yaml:",inline"`
	Metadata ObjectMeta    `json:"metadata" yaml:"metadata"`
	Spec     AgentPoolSpec `json:"spec" yaml:"spec"`
	Status   AgentPoolStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type AgentPoolSpec struct {
	Replicas int               `json:"replicas" yaml:"replicas"`
	Selector map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Template AgentPodTemplate  `json:"template" yaml:"template"`
}

type AgentPodTemplate struct {
	Metadata ObjectMeta   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec     AgentPodSpec `json:"spec" yaml:"spec"`
}

type AgentPoolStatus struct {
	Replicas      int `json:"replicas" yaml:"replicas"`
	ReadyReplicas int `json:"readyReplicas" yaml:"readyReplicas"`
	BusyReplicas  int `json:"busyReplicas" yaml:"busyReplicas"`
}

// -------------------------------------------------------
// DevTask (Job equivalent)
// -------------------------------------------------------

// DevTaskPhase represents the lifecycle phase of a DevTask.
type DevTaskPhase string

const (
	TaskPending   DevTaskPhase = "Pending"
	TaskScheduled DevTaskPhase = "Scheduled"
	TaskRunning   DevTaskPhase = "Running"
	TaskSucceeded DevTaskPhase = "Succeeded"
	TaskFailed    DevTaskPhase = "Failed"
)

// DevTask represents a development task to be executed by an agent.
type DevTask struct {
	TypeMeta `json:",inline" yaml:",inline"`
	Metadata ObjectMeta  `json:"metadata" yaml:"metadata"`
	Spec     DevTaskSpec `json:"spec" yaml:"spec"`
	Status   DevTaskStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type DevTaskSpec struct {
	Prompt               string   `json:"prompt" yaml:"prompt"`
	RequiredCapabilities []string `json:"requiredCapabilities,omitempty" yaml:"requiredCapabilities,omitempty"`
	PreferredModel       string   `json:"preferredModel,omitempty" yaml:"preferredModel,omitempty"`
	MaxRetries           int      `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	TimeoutSeconds       int      `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
	DependsOn            []string `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`
}

type DevTaskStatus struct {
	Phase       DevTaskPhase `json:"phase" yaml:"phase"`
	AssignedPod string       `json:"assignedPod,omitempty" yaml:"assignedPod,omitempty"`
	Retries     int          `json:"retries" yaml:"retries"`
	Output      string       `json:"output,omitempty" yaml:"output,omitempty"`
	Error       string       `json:"error,omitempty" yaml:"error,omitempty"`
	StartedAt   time.Time    `json:"startedAt,omitempty" yaml:"startedAt,omitempty"`
	FinishedAt  time.Time    `json:"finishedAt,omitempty" yaml:"finishedAt,omitempty"`
}

// -------------------------------------------------------
// Watch types
// -------------------------------------------------------

// EventType represents the type of a watch event.
type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
)

// WatchEvent is emitted when a resource changes in the store.
type WatchEvent struct {
	Type     EventType
	Kind     string
	Key      string
	Object   interface{}
}

// -------------------------------------------------------
// Log entry
// -------------------------------------------------------

// LogEntry represents a single log line from an agent.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	PodName   string    `json:"podName"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}
