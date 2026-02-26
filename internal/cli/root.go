package cli

import (
	"github.com/klubi/orca/pkg/client"
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	apiClient  *client.Client
)

// NewRootCmd creates the top-level orca CLI command with all subcommands.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orca",
		Short: "Kubernetes-inspired AI Agent Orchestration",
		Long: `Orca orchestrates AI agents using Kubernetes patterns.
Manage agent pods, pools, and development tasks.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Skip client init for commands that don't need the API server.
			name := cmd.Name()
			if name == "serve" || name == "init" {
				return
			}
			apiClient = client.New(serverAddr)
		},
	}

	cmd.PersistentFlags().StringVar(&serverAddr, "server", "http://127.0.0.1:7117", "Orca server address")
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table|json|yaml")

	cmd.AddCommand(
		newServeCmd(),
		newApplyCmd(),
		newGetCmd(),
		newDescribeCmd(),
		newDeleteCmd(),
		newLogsCmd(),
		newRunCmd(),
		newScaleCmd(),
		newStatusCmd(),
		newExecCmd(),
		newInitCmd(),
	)

	return cmd
}
