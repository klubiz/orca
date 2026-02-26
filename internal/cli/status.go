package cli

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

func newStatusCmd() *cobra.Command {
	var watch bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show cluster dashboard",
		Long:  "Display an overview of the Orca control plane status.",
		Example: `  orca status
  orca status -p myproject
  orca status --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")

			if watch {
				return statusWatch(project)
			}

			return statusPrint(project)
		},
	}

	cmd.Flags().StringP("project", "p", "", "Filter by project (empty = all)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Continuously refresh (every 5 seconds)")

	return cmd
}

func statusPrint(project string) error {
	// Check server health first.
	if err := apiClient.Healthz(); err != nil {
		color.Red("Orca Control Plane: UNREACHABLE")
		return fmt.Errorf("cannot reach server: %w", err)
	}

	bold := color.New(color.FgCyan, color.Bold)
	bold.Println("Orca Control Plane Status")
	fmt.Println("========================")
	fmt.Println()

	// Projects
	projects, err := apiClient.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}
	fmt.Printf("Projects: %d\n", len(projects))

	// Determine which projects to query for pods/pools/tasks.
	projectNames := []string{}
	if project != "" {
		projectNames = []string{project}
	} else {
		for _, p := range projects {
			projectNames = append(projectNames, p.Metadata.Name)
		}
		// Always include "default" if not already present.
		found := false
		for _, n := range projectNames {
			if n == "default" {
				found = true
				break
			}
		}
		if !found {
			projectNames = append(projectNames, "default")
		}
	}

	// Aggregate pod stats.
	var totalPods, readyPods, busyPods, failedPods, pendingPods int
	for _, pName := range projectNames {
		pods, err := apiClient.ListAgentPods(pName)
		if err != nil {
			continue
		}
		for _, pod := range pods {
			totalPods++
			switch pod.Status.Phase {
			case v1alpha1.PodReady:
				readyPods++
			case v1alpha1.PodBusy:
				busyPods++
			case v1alpha1.PodFailed:
				failedPods++
			case v1alpha1.PodPending, v1alpha1.PodStarting:
				pendingPods++
			}
		}
	}

	fmt.Printf("Agent Pods: %d total", totalPods)
	if totalPods > 0 {
		fmt.Printf(" (")
		parts := []string{}
		if readyPods > 0 {
			parts = append(parts, color.GreenString("%d ready", readyPods))
		}
		if busyPods > 0 {
			parts = append(parts, color.YellowString("%d busy", busyPods))
		}
		if pendingPods > 0 {
			parts = append(parts, fmt.Sprintf("%d pending", pendingPods))
		}
		if failedPods > 0 {
			parts = append(parts, color.RedString("%d failed", failedPods))
		}
		for i, p := range parts {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(p)
		}
		fmt.Print(")")
	}
	fmt.Println()

	// Aggregate pool stats.
	var totalPools int
	for _, pName := range projectNames {
		pools, err := apiClient.ListAgentPools(pName)
		if err != nil {
			continue
		}
		totalPools += len(pools)
	}
	fmt.Printf("Agent Pools: %d\n", totalPools)

	// Aggregate task stats.
	var totalTasks, pendingTasks, runningTasks, succeededTasks, failedTasks int
	for _, pName := range projectNames {
		tasks, err := apiClient.ListDevTasks(pName)
		if err != nil {
			continue
		}
		for _, task := range tasks {
			totalTasks++
			switch task.Status.Phase {
			case v1alpha1.TaskPending, v1alpha1.TaskScheduled:
				pendingTasks++
			case v1alpha1.TaskRunning:
				runningTasks++
			case v1alpha1.TaskSucceeded:
				succeededTasks++
			case v1alpha1.TaskFailed:
				failedTasks++
			}
		}
	}

	fmt.Printf("Dev Tasks: %d total", totalTasks)
	if totalTasks > 0 {
		fmt.Printf(" (")
		parts := []string{}
		if pendingTasks > 0 {
			parts = append(parts, fmt.Sprintf("%d pending", pendingTasks))
		}
		if runningTasks > 0 {
			parts = append(parts, color.YellowString("%d running", runningTasks))
		}
		if succeededTasks > 0 {
			parts = append(parts, color.GreenString("%d succeeded", succeededTasks))
		}
		if failedTasks > 0 {
			parts = append(parts, color.RedString("%d failed", failedTasks))
		}
		for i, p := range parts {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(p)
		}
		fmt.Print(")")
	}
	fmt.Println()

	return nil
}

func statusWatch(project string) error {
	fmt.Println("Watching status (Ctrl+C to stop)...")
	fmt.Println()

	for {
		// Clear screen with ANSI escape.
		fmt.Print("\033[2J\033[H")

		if err := statusPrint(project); err != nil {
			fmt.Printf("\nError: %v\n", err)
		}

		fmt.Printf("\nLast updated: %s\n", time.Now().Format("15:04:05"))
		time.Sleep(5 * time.Second)
	}
}
