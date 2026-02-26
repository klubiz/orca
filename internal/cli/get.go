package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <resource-type> [name]",
		Short: "List or get resources",
		Long: `Display one or many resources.

Resource types: agentpods (pod), agentpools (pool), devtasks (task), projects`,
		Example: `  orca get pods
  orca get pods my-agent -p myproject
  orca get pools
  orca get tasks
  orca get projects`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			resourceType := normalizeResourceType(args[0])

			var name string
			if len(args) > 1 {
				name = args[1]
			}

			switch resourceType {
			case "agentpods":
				return getAgentPods(project, name)
			case "agentpools":
				return getAgentPools(project, name)
			case "devtasks":
				return getDevTasks(project, name)
			case "projects":
				return getProjects(name)
			default:
				return fmt.Errorf("unknown resource type %q. Valid types: agentpods, agentpools, devtasks, projects", args[0])
			}
		},
	}

	cmd.Flags().StringP("project", "p", "default", "Project name")

	return cmd
}

// normalizeResourceType maps various aliases to canonical resource type names.
func normalizeResourceType(t string) string {
	t = strings.ToLower(t)
	switch t {
	case "agentpod", "agentpods", "pod", "pods":
		return "agentpods"
	case "agentpool", "agentpools", "pool", "pools":
		return "agentpools"
	case "devtask", "devtasks", "task", "tasks":
		return "devtasks"
	case "project", "projects", "proj":
		return "projects"
	default:
		return t
	}
}

func getAgentPods(project, name string) error {
	if name != "" {
		pod, err := apiClient.GetAgentPod(name, project)
		if err != nil {
			return err
		}
		printOutput(pod, agentPodHeaders(), agentPodToRow)
		return nil
	}

	pods, err := apiClient.ListAgentPods(project)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		fmt.Println("No agent pods found.")
		return nil
	}

	items := make([]interface{}, len(pods))
	for i := range pods {
		items[i] = &pods[i]
	}
	printOutput(items, agentPodHeaders(), agentPodToRow)
	return nil
}

func getAgentPools(project, name string) error {
	if name != "" {
		pool, err := apiClient.GetAgentPool(name, project)
		if err != nil {
			return err
		}
		printOutput(pool, agentPoolHeaders(), agentPoolToRow)
		return nil
	}

	pools, err := apiClient.ListAgentPools(project)
	if err != nil {
		return err
	}

	if len(pools) == 0 {
		fmt.Println("No agent pools found.")
		return nil
	}

	items := make([]interface{}, len(pools))
	for i := range pools {
		items[i] = &pools[i]
	}
	printOutput(items, agentPoolHeaders(), agentPoolToRow)
	return nil
}

func getDevTasks(project, name string) error {
	if name != "" {
		task, err := apiClient.GetDevTask(name, project)
		if err != nil {
			return err
		}
		printOutput(task, devTaskHeaders(), devTaskToRow)
		return nil
	}

	tasks, err := apiClient.ListDevTasks(project)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No dev tasks found.")
		return nil
	}

	items := make([]interface{}, len(tasks))
	for i := range tasks {
		items[i] = &tasks[i]
	}
	printOutput(items, devTaskHeaders(), devTaskToRow)
	return nil
}

func getProjects(name string) error {
	if name != "" {
		proj, err := apiClient.GetProject(name)
		if err != nil {
			return err
		}
		printOutput(proj, projectHeaders(), projectToRow)
		return nil
	}

	projects, err := apiClient.ListProjects()
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	items := make([]interface{}, len(projects))
	for i := range projects {
		items[i] = &projects[i]
	}
	printOutput(items, projectHeaders(), projectToRow)
	return nil
}

// --- Table headers and row converters ---

func agentPodHeaders() []string {
	return []string{"NAME", "PROJECT", "MODEL", "PHASE", "ACTIVE-TASKS", "AGE"}
}

func agentPodToRow(v interface{}) []string {
	pod, ok := v.(*v1alpha1.AgentPod)
	if !ok {
		return []string{"?", "?", "?", "?", "?", "?"}
	}
	return []string{
		pod.Metadata.Name,
		pod.Metadata.Project,
		pod.Spec.Model,
		colorPhase(string(pod.Status.Phase)),
		strconv.Itoa(pod.Status.ActiveTasks),
		formatAge(pod.Metadata.CreatedAt),
	}
}

func agentPoolHeaders() []string {
	return []string{"NAME", "PROJECT", "REPLICAS", "READY", "BUSY", "AGE"}
}

func agentPoolToRow(v interface{}) []string {
	pool, ok := v.(*v1alpha1.AgentPool)
	if !ok {
		return []string{"?", "?", "?", "?", "?", "?"}
	}
	return []string{
		pool.Metadata.Name,
		pool.Metadata.Project,
		strconv.Itoa(pool.Spec.Replicas),
		strconv.Itoa(pool.Status.ReadyReplicas),
		strconv.Itoa(pool.Status.BusyReplicas),
		formatAge(pool.Metadata.CreatedAt),
	}
}

func devTaskHeaders() []string {
	return []string{"NAME", "PROJECT", "PHASE", "ASSIGNED-POD", "RETRIES", "AGE"}
}

func devTaskToRow(v interface{}) []string {
	task, ok := v.(*v1alpha1.DevTask)
	if !ok {
		return []string{"?", "?", "?", "?", "?", "?"}
	}
	assignedPod := task.Status.AssignedPod
	if assignedPod == "" {
		assignedPod = "<none>"
	}
	return []string{
		task.Metadata.Name,
		task.Metadata.Project,
		colorPhase(string(task.Status.Phase)),
		assignedPod,
		strconv.Itoa(task.Status.Retries),
		formatAge(task.Metadata.CreatedAt),
	}
}

func projectHeaders() []string {
	return []string{"NAME", "STATUS", "AGE"}
}

func projectToRow(v interface{}) []string {
	proj, ok := v.(*v1alpha1.Project)
	if !ok {
		return []string{"?", "?", "?"}
	}
	status := proj.Status
	if status == "" {
		status = "Active"
	}
	return []string{
		proj.Metadata.Name,
		status,
		formatAge(proj.Metadata.CreatedAt),
	}
}

// colorPhase returns a colored string for known phases.
func colorPhase(phase string) string {
	switch phase {
	case "Ready", "Succeeded":
		return color.GreenString(phase)
	case "Failed":
		return color.RedString(phase)
	case "Busy", "Running":
		return color.YellowString(phase)
	case "Pending", "Scheduled":
		return color.WhiteString(phase)
	case "Terminating":
		return color.MagentaString(phase)
	case "Terminated":
		return color.HiBlackString(phase)
	default:
		return phase
	}
}
