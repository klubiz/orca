package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <resource-type> <name>",
		Short: "Delete a resource",
		Long:  "Delete a resource by type and name.",
		Example: `  orca delete pod my-agent -p myproject
  orca delete pool my-pool
  orca delete task build-feature
  orca delete project staging`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			resourceType := normalizeResourceType(args[0])
			name := args[1]

			switch resourceType {
			case "agentpods":
				if err := apiClient.DeleteAgentPod(name, project); err != nil {
					return err
				}
				fmt.Printf("agentpod/%s deleted\n", name)

			case "agentpools":
				if err := apiClient.DeleteAgentPool(name, project); err != nil {
					return err
				}
				fmt.Printf("agentpool/%s deleted\n", name)

			case "devtasks":
				if err := apiClient.DeleteDevTask(name, project); err != nil {
					return err
				}
				fmt.Printf("devtask/%s deleted\n", name)

			case "projects":
				if err := apiClient.DeleteProject(name); err != nil {
					return err
				}
				fmt.Printf("project/%s deleted\n", name)

			default:
				return fmt.Errorf("unknown resource type %q. Valid types: agentpods, agentpools, devtasks, projects", args[0])
			}

			return nil
		},
	}

	cmd.Flags().StringP("project", "p", "default", "Project name")

	return cmd
}
