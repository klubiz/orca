// Package client provides a Go client library for the Orca API server.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

// Client communicates with the Orca API server.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Orca API client pointing at the given base URL
// (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// doRequest builds and executes an HTTP request.
// If body is non-nil it is JSON-encoded and sent as the request body.
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	return resp, nil
}

// doJSON executes a request, checks for a 2xx status, and JSON-decodes
// the response body into target (when target is non-nil).
func (c *Client) doJSON(method, path string, body interface{}, target interface{}) error {
	resp, err := c.doRequest(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if target != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("decode response body: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// Healthz checks whether the API server is healthy.
func (c *Client) Healthz() error {
	resp, err := c.doRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("healthz failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

// CreateProject creates a new project.
func (c *Client) CreateProject(p *v1alpha1.Project) (*v1alpha1.Project, error) {
	var out v1alpha1.Project
	if err := c.doJSON(http.MethodPost, "/api/v1alpha1/projects", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProject retrieves a project by name.
func (c *Client) GetProject(name string) (*v1alpha1.Project, error) {
	var out v1alpha1.Project
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/api/v1alpha1/projects/%s", name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListProjects returns all projects.
func (c *Client) ListProjects() ([]v1alpha1.Project, error) {
	var out []v1alpha1.Project
	if err := c.doJSON(http.MethodGet, "/api/v1alpha1/projects", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateProject updates an existing project.
func (c *Client) UpdateProject(p *v1alpha1.Project) (*v1alpha1.Project, error) {
	var out v1alpha1.Project
	path := fmt.Sprintf("/api/v1alpha1/projects/%s", p.Metadata.Name)
	if err := c.doJSON(http.MethodPut, path, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteProject removes a project by name.
func (c *Client) DeleteProject(name string) error {
	return c.doJSON(http.MethodDelete, fmt.Sprintf("/api/v1alpha1/projects/%s", name), nil, nil)
}

// ---------------------------------------------------------------------------
// AgentPods
// ---------------------------------------------------------------------------

// CreateAgentPod creates a new agent pod in the given project.
func (c *Client) CreateAgentPod(pod *v1alpha1.AgentPod) (*v1alpha1.AgentPod, error) {
	var out v1alpha1.AgentPod
	path := fmt.Sprintf("/api/v1alpha1/agentpods?project=%s", pod.Metadata.Project)
	if err := c.doJSON(http.MethodPost, path, pod, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgentPod retrieves an agent pod by name within a project.
func (c *Client) GetAgentPod(name, project string) (*v1alpha1.AgentPod, error) {
	var out v1alpha1.AgentPod
	path := fmt.Sprintf("/api/v1alpha1/agentpods/%s?project=%s", name, project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgentPods returns all agent pods in a project.
func (c *Client) ListAgentPods(project string) ([]v1alpha1.AgentPod, error) {
	var out []v1alpha1.AgentPod
	path := fmt.Sprintf("/api/v1alpha1/agentpods?project=%s", project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateAgentPod updates an existing agent pod.
func (c *Client) UpdateAgentPod(pod *v1alpha1.AgentPod) (*v1alpha1.AgentPod, error) {
	var out v1alpha1.AgentPod
	path := fmt.Sprintf("/api/v1alpha1/agentpods/%s?project=%s", pod.Metadata.Name, pod.Metadata.Project)
	if err := c.doJSON(http.MethodPut, path, pod, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAgentPod removes an agent pod by name within a project.
func (c *Client) DeleteAgentPod(name, project string) error {
	path := fmt.Sprintf("/api/v1alpha1/agentpods/%s?project=%s", name, project)
	return c.doJSON(http.MethodDelete, path, nil, nil)
}

// ---------------------------------------------------------------------------
// AgentPools
// ---------------------------------------------------------------------------

// CreateAgentPool creates a new agent pool in the given project.
func (c *Client) CreateAgentPool(pool *v1alpha1.AgentPool) (*v1alpha1.AgentPool, error) {
	var out v1alpha1.AgentPool
	path := fmt.Sprintf("/api/v1alpha1/agentpools?project=%s", pool.Metadata.Project)
	if err := c.doJSON(http.MethodPost, path, pool, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgentPool retrieves an agent pool by name within a project.
func (c *Client) GetAgentPool(name, project string) (*v1alpha1.AgentPool, error) {
	var out v1alpha1.AgentPool
	path := fmt.Sprintf("/api/v1alpha1/agentpools/%s?project=%s", name, project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgentPools returns all agent pools in a project.
func (c *Client) ListAgentPools(project string) ([]v1alpha1.AgentPool, error) {
	var out []v1alpha1.AgentPool
	path := fmt.Sprintf("/api/v1alpha1/agentpools?project=%s", project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateAgentPool updates an existing agent pool.
func (c *Client) UpdateAgentPool(pool *v1alpha1.AgentPool) (*v1alpha1.AgentPool, error) {
	var out v1alpha1.AgentPool
	path := fmt.Sprintf("/api/v1alpha1/agentpools/%s?project=%s", pool.Metadata.Name, pool.Metadata.Project)
	if err := c.doJSON(http.MethodPut, path, pool, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAgentPool removes an agent pool by name within a project.
func (c *Client) DeleteAgentPool(name, project string) error {
	path := fmt.Sprintf("/api/v1alpha1/agentpools/%s?project=%s", name, project)
	return c.doJSON(http.MethodDelete, path, nil, nil)
}

// ScaleAgentPool adjusts the replica count of an agent pool.
func (c *Client) ScaleAgentPool(name, project string, replicas int) (*v1alpha1.AgentPool, error) {
	var out v1alpha1.AgentPool
	path := fmt.Sprintf("/api/v1alpha1/agentpools/%s/scale?project=%s", name, project)
	body := map[string]int{"replicas": replicas}
	if err := c.doJSON(http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// DevTasks
// ---------------------------------------------------------------------------

// CreateDevTask creates a new development task in the given project.
func (c *Client) CreateDevTask(task *v1alpha1.DevTask) (*v1alpha1.DevTask, error) {
	var out v1alpha1.DevTask
	path := fmt.Sprintf("/api/v1alpha1/devtasks?project=%s", task.Metadata.Project)
	if err := c.doJSON(http.MethodPost, path, task, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDevTask retrieves a development task by name within a project.
func (c *Client) GetDevTask(name, project string) (*v1alpha1.DevTask, error) {
	var out v1alpha1.DevTask
	path := fmt.Sprintf("/api/v1alpha1/devtasks/%s?project=%s", name, project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDevTasks returns all development tasks in a project.
func (c *Client) ListDevTasks(project string) ([]v1alpha1.DevTask, error) {
	var out []v1alpha1.DevTask
	path := fmt.Sprintf("/api/v1alpha1/devtasks?project=%s", project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateDevTask updates an existing development task.
func (c *Client) UpdateDevTask(task *v1alpha1.DevTask) (*v1alpha1.DevTask, error) {
	var out v1alpha1.DevTask
	path := fmt.Sprintf("/api/v1alpha1/devtasks/%s?project=%s", task.Metadata.Name, task.Metadata.Project)
	if err := c.doJSON(http.MethodPut, path, task, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDevTask removes a development task by name within a project.
func (c *Client) DeleteDevTask(name, project string) error {
	path := fmt.Sprintf("/api/v1alpha1/devtasks/%s?project=%s", name, project)
	return c.doJSON(http.MethodDelete, path, nil, nil)
}

// ---------------------------------------------------------------------------
// Apply (generic create-or-update)
// ---------------------------------------------------------------------------

// Apply sends a resource to the server's apply endpoint, which performs a
// create-or-update operation. The returned interface{} contains the
// server's response decoded as a raw JSON map.
func (c *Client) Apply(resource interface{}) (interface{}, error) {
	var out interface{}
	if err := c.doJSON(http.MethodPost, "/api/v1alpha1/apply", resource, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Logs
// ---------------------------------------------------------------------------

// GetLogs retrieves log entries for an agent pod.
func (c *Client) GetLogs(podName, project string) ([]v1alpha1.LogEntry, error) {
	var out []v1alpha1.LogEntry
	path := fmt.Sprintf("/api/v1alpha1/agentpods/%s/logs?project=%s", podName, project)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
