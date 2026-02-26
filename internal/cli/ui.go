package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/klubi/orca/internal/tui"
)

func newUICmd() *cobra.Command {
	var server string

	cmd := &cobra.Command{
		Use:     "ui",
		Aliases: []string{"top", "dashboard"},
		Short:   "Launch the interactive terminal UI",
		Long:    "Launch a k9s-style terminal UI for real-time monitoring and management of Orca resources.",
		Example: `  orca ui
  orca ui --server http://127.0.0.1:7117`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := tui.NewApp(server)
			if err := app.Run(); err != nil {
				return fmt.Errorf("UI error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&server, "server", "http://127.0.0.1:7117", "Orca API server address")

	return cmd
}
