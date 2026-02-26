package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newScaleCmd() *cobra.Command {
	var replicas int

	cmd := &cobra.Command{
		Use:   "scale <resource-type> <name>",
		Short: "Scale an agent pool",
		Long:  "Adjust the replica count of an agent pool.",
		Example: `  orca scale agentpool my-pool --replicas=5
  orca scale pool my-pool --replicas=3 -p myproject`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			resourceType := normalizeResourceType(args[0])
			name := args[1]

			if resourceType != "agentpools" {
				return fmt.Errorf("scaling is only supported for agentpools, got %q", args[0])
			}

			if replicas < 0 {
				return fmt.Errorf("replicas must be >= 0, got %d", replicas)
			}

			_, err := apiClient.ScaleAgentPool(name, project, replicas)
			if err != nil {
				return err
			}

			fmt.Printf("agentpool/%s scaled to %d replicas\n", name, replicas)
			return nil
		},
	}

	cmd.Flags().IntVar(&replicas, "replicas", 1, "Number of replicas")
	cmd.Flags().StringP("project", "p", "default", "Project name")

	return cmd
}
