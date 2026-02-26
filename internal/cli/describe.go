package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <resource-type> <name>",
		Short: "Show detailed info about a resource",
		Long:  "Print a detailed description of a specific resource in kubectl-describe style.",
		Example: `  orca describe pod my-agent
  orca describe pool my-pool -p myproject
  orca describe task build-feature
  orca describe project default`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			resourceType := normalizeResourceType(args[0])
			name := args[1]

			switch resourceType {
			case "agentpods":
				return describeAgentPod(name, project)
			case "agentpools":
				return describeAgentPool(name, project)
			case "devtasks":
				return describeDevTask(name, project)
			case "projects":
				return describeProject(name)
			default:
				return fmt.Errorf("unknown resource type %q", args[0])
			}
		},
	}

	cmd.Flags().StringP("project", "p", "default", "Project name")

	return cmd
}

func describeAgentPod(name, project string) error {
	pod, err := apiClient.GetAgentPod(name, project)
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)

	bold.Println("AgentPod:")
	printField("  Name", pod.Metadata.Name)
	printField("  Project", pod.Metadata.Project)
	printField("  UID", pod.Metadata.UID)
	printField("  Labels", formatLabels(pod.Metadata.Labels))
	printField("  Created", pod.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	printField("  Updated", pod.Metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()
	bold.Println("Spec:")
	printField("  Model", pod.Spec.Model)
	if pod.Spec.SystemPrompt != "" {
		printField("  System Prompt", truncate(pod.Spec.SystemPrompt, 80))
	}
	printField("  Capabilities", formatStringSlice(pod.Spec.Capabilities))
	printField("  Max Concurrency", fmt.Sprintf("%d", pod.Spec.MaxConcurrency))
	printField("  Max Tokens", fmt.Sprintf("%d", pod.Spec.MaxTokens))
	printField("  Tools", formatStringSlice(pod.Spec.Tools))
	printField("  Restart Policy", pod.Spec.RestartPolicy)
	if pod.Spec.OwnerPool != "" {
		printField("  Owner Pool", pod.Spec.OwnerPool)
	}

	fmt.Println()
	bold.Println("Status:")
	printField("  Phase", colorPhase(string(pod.Status.Phase)))
	printField("  Active Tasks", fmt.Sprintf("%d", pod.Status.ActiveTasks))
	printField("  Completed Tasks", fmt.Sprintf("%d", pod.Status.CompletedTasks))
	printField("  Failed Tasks", fmt.Sprintf("%d", pod.Status.FailedTasks))
	if !pod.Status.StartedAt.IsZero() {
		printField("  Started At", pod.Status.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if !pod.Status.LastHeartbeat.IsZero() {
		printField("  Last Heartbeat", pod.Status.LastHeartbeat.Format("2006-01-02 15:04:05"))
	}
	if pod.Status.Message != "" {
		printField("  Message", pod.Status.Message)
	}

	return nil
}

func describeAgentPool(name, project string) error {
	pool, err := apiClient.GetAgentPool(name, project)
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)

	bold.Println("AgentPool:")
	printField("  Name", pool.Metadata.Name)
	printField("  Project", pool.Metadata.Project)
	printField("  UID", pool.Metadata.UID)
	printField("  Labels", formatLabels(pool.Metadata.Labels))
	printField("  Created", pool.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	printField("  Updated", pool.Metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()
	bold.Println("Spec:")
	printField("  Replicas", fmt.Sprintf("%d", pool.Spec.Replicas))
	printField("  Selector", formatLabels(pool.Spec.Selector))

	fmt.Println()
	bold.Println("  Template:")
	printField("    Model", pool.Spec.Template.Spec.Model)
	if pool.Spec.Template.Spec.SystemPrompt != "" {
		printField("    System Prompt", truncate(pool.Spec.Template.Spec.SystemPrompt, 80))
	}
	printField("    Capabilities", formatStringSlice(pool.Spec.Template.Spec.Capabilities))
	printField("    Max Concurrency", fmt.Sprintf("%d", pool.Spec.Template.Spec.MaxConcurrency))
	printField("    Max Tokens", fmt.Sprintf("%d", pool.Spec.Template.Spec.MaxTokens))
	printField("    Tools", formatStringSlice(pool.Spec.Template.Spec.Tools))
	printField("    Restart Policy", pool.Spec.Template.Spec.RestartPolicy)

	fmt.Println()
	bold.Println("Status:")
	printField("  Replicas", fmt.Sprintf("%d", pool.Status.Replicas))
	printField("  Ready Replicas", fmt.Sprintf("%d", pool.Status.ReadyReplicas))
	printField("  Busy Replicas", fmt.Sprintf("%d", pool.Status.BusyReplicas))

	return nil
}

func describeDevTask(name, project string) error {
	task, err := apiClient.GetDevTask(name, project)
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)

	bold.Println("DevTask:")
	printField("  Name", task.Metadata.Name)
	printField("  Project", task.Metadata.Project)
	printField("  UID", task.Metadata.UID)
	printField("  Labels", formatLabels(task.Metadata.Labels))
	printField("  Created", task.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	printField("  Updated", task.Metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()
	bold.Println("Spec:")
	printField("  Prompt", task.Spec.Prompt)
	printField("  Required Capabilities", formatStringSlice(task.Spec.RequiredCapabilities))
	if task.Spec.PreferredModel != "" {
		printField("  Preferred Model", task.Spec.PreferredModel)
	}
	printField("  Max Retries", fmt.Sprintf("%d", task.Spec.MaxRetries))
	printField("  Timeout Seconds", fmt.Sprintf("%d", task.Spec.TimeoutSeconds))
	if len(task.Spec.DependsOn) > 0 {
		printField("  Depends On", formatStringSlice(task.Spec.DependsOn))
	}

	fmt.Println()
	bold.Println("Status:")
	printField("  Phase", colorPhase(string(task.Status.Phase)))
	assignedPod := task.Status.AssignedPod
	if assignedPod == "" {
		assignedPod = "<none>"
	}
	printField("  Assigned Pod", assignedPod)
	printField("  Retries", fmt.Sprintf("%d", task.Status.Retries))
	if !task.Status.StartedAt.IsZero() {
		printField("  Started At", task.Status.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if !task.Status.FinishedAt.IsZero() {
		printField("  Finished At", task.Status.FinishedAt.Format("2006-01-02 15:04:05"))
	}
	if task.Status.Output != "" {
		fmt.Println()
		bold.Println("Output:")
		fmt.Println(task.Status.Output)
	}
	if task.Status.Error != "" {
		fmt.Println()
		bold.Println("Error:")
		fmt.Println(color.RedString(task.Status.Error))
	}

	return nil
}

func describeProject(name string) error {
	proj, err := apiClient.GetProject(name)
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)

	bold.Println("Project:")
	printField("  Name", proj.Metadata.Name)
	printField("  UID", proj.Metadata.UID)
	printField("  Labels", formatLabels(proj.Metadata.Labels))
	printField("  Created", proj.Metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	printField("  Updated", proj.Metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	fmt.Println()
	bold.Println("Spec:")
	if proj.Spec.Description != "" {
		printField("  Description", proj.Spec.Description)
	}
	if proj.Spec.Path != "" {
		printField("  Path", proj.Spec.Path)
	}

	fmt.Println()
	bold.Println("Status:")
	status := proj.Status
	if status == "" {
		status = "Active"
	}
	printField("  Status", status)

	return nil
}

// --- Helpers ---

func printField(label, value string) {
	if value == "" {
		value = "<none>"
	}
	fmt.Printf("%-24s%s\n", label+":", value)
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

func formatStringSlice(items []string) string {
	if len(items) == 0 {
		return "<none>"
	}
	return strings.Join(items, ", ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
