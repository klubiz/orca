package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
)

func newRunCmd() *cobra.Command {
	var (
		model   string
		project string
		timeout int
	)

	cmd := &cobra.Command{
		Use:   "run -- <prompt>",
		Short: "Run a one-shot task",
		Long: `Create a temporary DevTask from a prompt and wait for completion.

Everything after "--" is treated as the prompt text.`,
		Example: `  orca run -- "Write a hello world program in Go"
  orca run --model claude-haiku -- "Summarize this code"
  orca run -p myproject -- "Fix the bug in auth.go"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("prompt required: orca run -- \"your prompt here\"")
			}
			prompt := strings.Join(args, " ")

			// Generate a unique task name based on the current time.
			taskName := fmt.Sprintf("run-%d", time.Now().UnixMilli())

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
					PreferredModel: model,
					MaxRetries:     0,
					TimeoutSeconds: timeout,
				},
			}

			// Create the task via the API.
			created, err := apiClient.CreateDevTask(task)
			if err != nil {
				return fmt.Errorf("creating task: %w", err)
			}

			fmt.Printf("Task %s created. Waiting for completion...\n", created.Metadata.Name)

			// Poll for task completion.
			pollInterval := 2 * time.Second
			timeoutDuration := time.Duration(timeout) * time.Second
			if timeout == 0 {
				timeoutDuration = 5 * time.Minute
			}
			deadline := time.Now().Add(timeoutDuration)

			for {
				if time.Now().After(deadline) {
					return fmt.Errorf("task %s did not complete within timeout (%v)", taskName, timeoutDuration)
				}

				current, err := apiClient.GetDevTask(taskName, project)
				if err != nil {
					return fmt.Errorf("polling task status: %w", err)
				}

				switch current.Status.Phase {
				case v1alpha1.TaskSucceeded:
					fmt.Println()
					color.New(color.FgGreen, color.Bold).Println("Task Succeeded")
					fmt.Println(strings.Repeat("-", 60))
					fmt.Println(current.Status.Output)
					return nil

				case v1alpha1.TaskFailed:
					fmt.Println()
					color.New(color.FgRed, color.Bold).Println("Task Failed")
					fmt.Println(strings.Repeat("-", 60))
					if current.Status.Error != "" {
						fmt.Println(current.Status.Error)
					}
					return fmt.Errorf("task %s failed", taskName)

				case v1alpha1.TaskRunning:
					fmt.Print(".")

				case v1alpha1.TaskScheduled:
					fmt.Print(".")

				case v1alpha1.TaskPending:
					// Still waiting for scheduling.
				}

				time.Sleep(pollInterval)
			}
		},
	}

	cmd.Flags().StringVar(&model, "model", "claude-sonnet", "Model to use")
	cmd.Flags().StringVarP(&project, "project", "p", "default", "Project name")
	cmd.Flags().IntVar(&timeout, "timeout", 300, "Timeout in seconds (0 for default 5 minutes)")

	return cmd
}
