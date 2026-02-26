package manifest

import (
	"os"
	"testing"

	"github.com/klubi/orca/pkg/apis/v1alpha1"
)

func TestParseProject(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: test-project
  labels:
    env: dev
spec:
  description: "A test project"
  path: "/tmp/test"
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	proj, ok := resources[0].(*v1alpha1.Project)
	if !ok {
		t.Fatalf("expected *v1alpha1.Project, got %T", resources[0])
	}
	if proj.APIVersion != "orca.dev/v1alpha1" {
		t.Errorf("expected apiVersion orca.dev/v1alpha1, got %s", proj.APIVersion)
	}
	if proj.Kind != "Project" {
		t.Errorf("expected kind Project, got %s", proj.Kind)
	}
	if proj.Metadata.Name != "test-project" {
		t.Errorf("expected name test-project, got %s", proj.Metadata.Name)
	}
	if proj.Metadata.Labels["env"] != "dev" {
		t.Errorf("expected label env=dev, got %s", proj.Metadata.Labels["env"])
	}
	if proj.Spec.Description != "A test project" {
		t.Errorf("expected description 'A test project', got %s", proj.Spec.Description)
	}
	if proj.Spec.Path != "/tmp/test" {
		t.Errorf("expected path /tmp/test, got %s", proj.Spec.Path)
	}
}

func TestParseAgentPod(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: AgentPod
metadata:
  name: coder-1
  project: my-project
spec:
  model: claude-sonnet-4-20250514
  systemPrompt: "You are a coding assistant."
  capabilities:
    - code-review
    - refactor
  maxConcurrency: 5
  maxTokens: 4096
  tools:
    - bash
    - editor
  restartPolicy: OnFailure
  ownerPool: pool-1
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	pod, ok := resources[0].(*v1alpha1.AgentPod)
	if !ok {
		t.Fatalf("expected *v1alpha1.AgentPod, got %T", resources[0])
	}
	if pod.Metadata.Name != "coder-1" {
		t.Errorf("expected name coder-1, got %s", pod.Metadata.Name)
	}
	if pod.Metadata.Project != "my-project" {
		t.Errorf("expected project my-project, got %s", pod.Metadata.Project)
	}
	if pod.Spec.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", pod.Spec.Model)
	}
	if pod.Spec.SystemPrompt != "You are a coding assistant." {
		t.Errorf("expected systemPrompt 'You are a coding assistant.', got %s", pod.Spec.SystemPrompt)
	}
	if len(pod.Spec.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(pod.Spec.Capabilities))
	}
	if pod.Spec.Capabilities[0] != "code-review" {
		t.Errorf("expected capability[0] code-review, got %s", pod.Spec.Capabilities[0])
	}
	if pod.Spec.Capabilities[1] != "refactor" {
		t.Errorf("expected capability[1] refactor, got %s", pod.Spec.Capabilities[1])
	}
	if pod.Spec.MaxConcurrency != 5 {
		t.Errorf("expected maxConcurrency 5, got %d", pod.Spec.MaxConcurrency)
	}
	if pod.Spec.MaxTokens != 4096 {
		t.Errorf("expected maxTokens 4096, got %d", pod.Spec.MaxTokens)
	}
	if len(pod.Spec.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(pod.Spec.Tools))
	}
	if pod.Spec.Tools[0] != "bash" {
		t.Errorf("expected tool[0] bash, got %s", pod.Spec.Tools[0])
	}
	if pod.Spec.RestartPolicy != "OnFailure" {
		t.Errorf("expected restartPolicy OnFailure, got %s", pod.Spec.RestartPolicy)
	}
	if pod.Spec.OwnerPool != "pool-1" {
		t.Errorf("expected ownerPool pool-1, got %s", pod.Spec.OwnerPool)
	}
}

func TestParseAgentPool(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: AgentPool
metadata:
  name: review-pool
  project: my-project
spec:
  replicas: 3
  selector:
    role: reviewer
  template:
    metadata:
      labels:
        role: reviewer
    spec:
      model: claude-sonnet-4-20250514
      capabilities:
        - code-review
      maxConcurrency: 2
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	pool, ok := resources[0].(*v1alpha1.AgentPool)
	if !ok {
		t.Fatalf("expected *v1alpha1.AgentPool, got %T", resources[0])
	}
	if pool.Metadata.Name != "review-pool" {
		t.Errorf("expected name review-pool, got %s", pool.Metadata.Name)
	}
	if pool.Metadata.Project != "my-project" {
		t.Errorf("expected project my-project, got %s", pool.Metadata.Project)
	}
	if pool.Spec.Replicas != 3 {
		t.Errorf("expected replicas 3, got %d", pool.Spec.Replicas)
	}
	if pool.Spec.Selector["role"] != "reviewer" {
		t.Errorf("expected selector role=reviewer, got %s", pool.Spec.Selector["role"])
	}
	if pool.Spec.Template.Metadata.Labels["role"] != "reviewer" {
		t.Errorf("expected template label role=reviewer, got %s", pool.Spec.Template.Metadata.Labels["role"])
	}
	if pool.Spec.Template.Spec.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected template model claude-sonnet-4-20250514, got %s", pool.Spec.Template.Spec.Model)
	}
	if len(pool.Spec.Template.Spec.Capabilities) != 1 {
		t.Fatalf("expected 1 capability in template, got %d", len(pool.Spec.Template.Spec.Capabilities))
	}
	if pool.Spec.Template.Spec.Capabilities[0] != "code-review" {
		t.Errorf("expected capability code-review, got %s", pool.Spec.Template.Spec.Capabilities[0])
	}
	if pool.Spec.Template.Spec.MaxConcurrency != 2 {
		t.Errorf("expected template maxConcurrency 2, got %d", pool.Spec.Template.Spec.MaxConcurrency)
	}
}

func TestParseDevTask(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: fix-auth-bug
  project: my-project
  labels:
    priority: high
    component: auth
spec:
  prompt: "Fix the authentication bug in login handler"
  requiredCapabilities:
    - debugging
    - code-review
  preferredModel: claude-sonnet-4-20250514
  maxRetries: 3
  timeoutSeconds: 600
  dependsOn:
    - write-tests
    - setup-env
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	task, ok := resources[0].(*v1alpha1.DevTask)
	if !ok {
		t.Fatalf("expected *v1alpha1.DevTask, got %T", resources[0])
	}
	if task.Metadata.Name != "fix-auth-bug" {
		t.Errorf("expected name fix-auth-bug, got %s", task.Metadata.Name)
	}
	if task.Metadata.Project != "my-project" {
		t.Errorf("expected project my-project, got %s", task.Metadata.Project)
	}
	if task.Metadata.Labels["priority"] != "high" {
		t.Errorf("expected label priority=high, got %s", task.Metadata.Labels["priority"])
	}
	if task.Metadata.Labels["component"] != "auth" {
		t.Errorf("expected label component=auth, got %s", task.Metadata.Labels["component"])
	}
	if task.Spec.Prompt != "Fix the authentication bug in login handler" {
		t.Errorf("expected prompt 'Fix the authentication bug in login handler', got %s", task.Spec.Prompt)
	}
	if len(task.Spec.RequiredCapabilities) != 2 {
		t.Fatalf("expected 2 requiredCapabilities, got %d", len(task.Spec.RequiredCapabilities))
	}
	if task.Spec.RequiredCapabilities[0] != "debugging" {
		t.Errorf("expected requiredCapability[0] debugging, got %s", task.Spec.RequiredCapabilities[0])
	}
	if task.Spec.RequiredCapabilities[1] != "code-review" {
		t.Errorf("expected requiredCapability[1] code-review, got %s", task.Spec.RequiredCapabilities[1])
	}
	if task.Spec.PreferredModel != "claude-sonnet-4-20250514" {
		t.Errorf("expected preferredModel claude-sonnet-4-20250514, got %s", task.Spec.PreferredModel)
	}
	if task.Spec.MaxRetries != 3 {
		t.Errorf("expected maxRetries 3, got %d", task.Spec.MaxRetries)
	}
	if task.Spec.TimeoutSeconds != 600 {
		t.Errorf("expected timeoutSeconds 600, got %d", task.Spec.TimeoutSeconds)
	}
	if len(task.Spec.DependsOn) != 2 {
		t.Fatalf("expected 2 dependsOn, got %d", len(task.Spec.DependsOn))
	}
	if task.Spec.DependsOn[0] != "write-tests" {
		t.Errorf("expected dependsOn[0] write-tests, got %s", task.Spec.DependsOn[0])
	}
	if task.Spec.DependsOn[1] != "setup-env" {
		t.Errorf("expected dependsOn[1] setup-env, got %s", task.Spec.DependsOn[1])
	}
}

func TestParseMultiDocument(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: multi-project
spec:
  description: "Project in multi-doc"
---
apiVersion: orca.dev/v1alpha1
kind: AgentPod
metadata:
  name: multi-pod
spec:
  model: claude-sonnet-4-20250514
---
apiVersion: orca.dev/v1alpha1
kind: DevTask
metadata:
  name: multi-task
spec:
  prompt: "Do something"
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// First resource: Project
	proj, ok := resources[0].(*v1alpha1.Project)
	if !ok {
		t.Fatalf("expected resource[0] to be *v1alpha1.Project, got %T", resources[0])
	}
	if proj.Metadata.Name != "multi-project" {
		t.Errorf("expected project name multi-project, got %s", proj.Metadata.Name)
	}

	// Second resource: AgentPod
	pod, ok := resources[1].(*v1alpha1.AgentPod)
	if !ok {
		t.Fatalf("expected resource[1] to be *v1alpha1.AgentPod, got %T", resources[1])
	}
	if pod.Metadata.Name != "multi-pod" {
		t.Errorf("expected pod name multi-pod, got %s", pod.Metadata.Name)
	}

	// Third resource: DevTask
	task, ok := resources[2].(*v1alpha1.DevTask)
	if !ok {
		t.Fatalf("expected resource[2] to be *v1alpha1.DevTask, got %T", resources[2])
	}
	if task.Metadata.Name != "multi-task" {
		t.Errorf("expected task name multi-task, got %s", task.Metadata.Name)
	}
}

func TestParseDefaultAPIVersion(t *testing.T) {
	yaml := []byte(`
kind: Project
metadata:
  name: no-version-project
spec:
  description: "No apiVersion specified"
`)
	resources, err := ParseBytes(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	proj, ok := resources[0].(*v1alpha1.Project)
	if !ok {
		t.Fatalf("expected *v1alpha1.Project, got %T", resources[0])
	}
	if proj.APIVersion != v1alpha1.APIVersion {
		t.Errorf("expected default apiVersion %s, got %s", v1alpha1.APIVersion, proj.APIVersion)
	}
	if proj.Metadata.Name != "no-version-project" {
		t.Errorf("expected name no-version-project, got %s", proj.Metadata.Name)
	}
}

func TestParseEmptyName(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: ""
spec:
  description: "No name"
`)
	_, err := ParseBytes(yaml)
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestParseUnknownKind(t *testing.T) {
	yaml := []byte(`
apiVersion: orca.dev/v1alpha1
kind: UnknownThing
metadata:
  name: test
`)
	_, err := ParseBytes(yaml)
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}

func TestParseFile(t *testing.T) {
	content := []byte(`apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: file-project
spec:
  description: "Parsed from file"
---
apiVersion: orca.dev/v1alpha1
kind: AgentPod
metadata:
  name: file-pod
spec:
  model: claude-sonnet-4-20250514
`)

	tmpFile, err := os.CreateTemp("", "orca-manifest-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	resources, err := ParseFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	proj, ok := resources[0].(*v1alpha1.Project)
	if !ok {
		t.Fatalf("expected resource[0] to be *v1alpha1.Project, got %T", resources[0])
	}
	if proj.Metadata.Name != "file-project" {
		t.Errorf("expected name file-project, got %s", proj.Metadata.Name)
	}
	if proj.Spec.Description != "Parsed from file" {
		t.Errorf("expected description 'Parsed from file', got %s", proj.Spec.Description)
	}

	pod, ok := resources[1].(*v1alpha1.AgentPod)
	if !ok {
		t.Fatalf("expected resource[1] to be *v1alpha1.AgentPod, got %T", resources[1])
	}
	if pod.Metadata.Name != "file-pod" {
		t.Errorf("expected name file-pod, got %s", pod.Metadata.Name)
	}
	if pod.Spec.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", pod.Spec.Model)
	}
}
