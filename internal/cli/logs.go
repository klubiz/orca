package cli

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs <podname>",
		Short: "Show logs for an agent pod",
		Long:  "Retrieve and display log entries from a specific agent pod.",
		Example: `  orca logs my-agent
  orca logs my-agent -p myproject
  orca logs my-agent --follow`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			podName := args[0]

			if follow {
				return logsFollow(podName, project)
			}

			return logsPrint(podName, project)
		},
	}

	cmd.Flags().StringP("project", "p", "default", "Project name")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (polls every 2 seconds)")

	return cmd
}

func logsPrint(podName, project string) error {
	entries, err := apiClient.GetLogs(podName, project)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Printf("No logs found for pod %s.\n", podName)
		return nil
	}

	for _, entry := range entries {
		printLogEntry(entry.Timestamp, entry.Level, entry.Message)
	}

	return nil
}

func logsFollow(podName, project string) error {
	// Track the number of entries we've already printed to avoid duplicates.
	seen := 0

	fmt.Printf("Following logs for pod %s (Ctrl+C to stop)...\n", podName)

	for {
		entries, err := apiClient.GetLogs(podName, project)
		if err != nil {
			return err
		}

		// Print only new entries.
		if len(entries) > seen {
			for _, entry := range entries[seen:] {
				printLogEntry(entry.Timestamp, entry.Level, entry.Message)
			}
			seen = len(entries)
		}

		time.Sleep(2 * time.Second)
	}
}

// printLogEntry prints a single formatted log line.
func printLogEntry(ts time.Time, level, message string) {
	timestamp := ts.Format("2006-01-02 15:04:05")

	var levelStr string
	switch level {
	case "ERROR", "error":
		levelStr = color.RedString("%-5s", level)
	case "WARN", "warn":
		levelStr = color.YellowString("%-5s", level)
	case "INFO", "info":
		levelStr = color.GreenString("%-5s", level)
	case "DEBUG", "debug":
		levelStr = color.HiBlackString("%-5s", level)
	default:
		levelStr = fmt.Sprintf("%-5s", level)
	}

	fmt.Printf("[%s] [%s] %s\n", timestamp, levelStr, message)
}
