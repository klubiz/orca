package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

func newExecCmd() *cobra.Command {
	var timeout int

	cmd := &cobra.Command{
		Use:   "exec <podname> -- <prompt>",
		Short: "Send a prompt to a specific pod",
		Long: `Execute a prompt on a specific agent pod by creating a targeted DevTask.

Everything after "--" is treated as the prompt text.`,
		Example: `  orca exec my-agent -- "Explain this codebase"
  orca exec my-agent -p myproject -- "Write tests for auth.go"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			podName := args[0]

			// Find the prompt: everything after the first arg (pod name).
			if len(args) < 2 {
				return fmt.Errorf("prompt required: orca exec <podname> -- \"your prompt\"")
			}
			prompt := strings.Join(args[1:], " ")

			// Verify the pod exists and get its model.
			pod, err := apiClient.GetAgentPod(podName, project)
			if err != nil {
				return fmt.Errorf("getting pod %s: %w", podName, err)
			}

			// Create a task targeting this pod's project with the pod's model.
			taskName := fmt.Sprintf("exec-%s-%d", podName, time.Now().UnixMilli())

			task := &v1alpha1.DevTask{
				TypeMeta: v1alpha1.TypeMeta{
					APIVersion: v1alpha1.APIVersion,
					Kind:       v1alpha1.KindDevTask,
				},
				Metadata: v1alpha1.ObjectMeta{
					Name:    taskName,
					Project: project,
				},
				Spec: v1alpha1.DevTaskSpec{
					Prompt:         prompt,
					PreferredModel: pod.Spec.Model,
					MaxRetries:     0,
					TimeoutSeconds: timeout,
				},
			}

			created, err := apiClient.CreateDevTask(task)
			if err != nil {
				return fmt.Errorf("creating exec task: %w", err)
			}

			fmt.Printf("Exec task %s created targeting pod %s. Waiting for completion...\n", created.Metadata.Name, podName)

			// Poll for task completion.
			pollInterval := 2 * time.Second
			timeoutDuration := time.Duration(timeout) * time.Second
			if timeout == 0 {
				timeoutDuration = 5 * time.Minute
			}
			deadline := time.Now().Add(timeoutDuration)

			for {
				if time.Now().After(deadline) {
					return fmt.Errorf("exec task %s did not complete within timeout (%v)", taskName, timeoutDuration)
				}

				current, err := apiClient.GetDevTask(taskName, project)
				if err != nil {
					return fmt.Errorf("polling task status: %w", err)
				}

				switch current.Status.Phase {
				case v1alpha1.TaskSucceeded:
					fmt.Println()
					color.New(color.FgGreen, color.Bold).Printf("Exec on %s Succeeded\n", podName)
					fmt.Println(strings.Repeat("-", 60))
					fmt.Println(current.Status.Output)
					return nil

				case v1alpha1.TaskFailed:
					fmt.Println()
					color.New(color.FgRed, color.Bold).Printf("Exec on %s Failed\n", podName)
					fmt.Println(strings.Repeat("-", 60))
					if current.Status.Error != "" {
						fmt.Println(current.Status.Error)
					}
					return fmt.Errorf("exec task %s failed", taskName)

				case v1alpha1.TaskRunning, v1alpha1.TaskScheduled:
					fmt.Print(".")

				case v1alpha1.TaskPending:
					// Still waiting.
				}

				time.Sleep(pollInterval)
			}
		},
	}

	cmd.Flags().StringP("project", "p", "default", "Project name")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Timeout in seconds")

	return cmd
}
