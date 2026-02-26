package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const projectTemplate = `apiVersion: orca.dev/v1alpha1
kind: Project
metadata:
  name: %s
spec:
  description: "%s"
  path: "%s"
---
apiVersion: orca.dev/v1alpha1
kind: AgentPool
metadata:
  name: %s-pool
  project: %s
spec:
  replicas: 1
  selector:
    app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      model: claude-sonnet
      capabilities:
        - code-generation
        - code-review
      maxConcurrency: 1
      maxTokens: 8192
      restartPolicy: Always
`

func newInitCmd() *cobra.Command {
	var (
		description string
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Initialize a new Orca project",
		Long: `Create a project manifest template in the current directory.

This generates a YAML file with a Project and a default AgentPool
that you can customize and apply with 'orca apply -f'.`,
		Example: `  orca init myproject
  orca init myproject --description "My AI project"
  orca init myproject --output-file custom-manifest.yaml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := "default"
			if len(args) > 0 {
				projectName = args[0]
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			if description == "" {
				description = fmt.Sprintf("Orca project: %s", projectName)
			}

			if outputFile == "" {
				outputFile = "project.yaml"
			}

			content := fmt.Sprintf(projectTemplate,
				projectName,
				description,
				cwd,
				projectName,
				projectName,
				projectName,
				projectName,
			)

			outputPath := filepath.Join(cwd, outputFile)

			// Check if file already exists.
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("file %s already exists. Use a different name with -o", outputFile)
			}

			if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("writing manifest file: %w", err)
			}

			bold := color.New(color.FgCyan, color.Bold)
			bold.Println("Orca project initialized!")
			fmt.Println()
			fmt.Printf("  Manifest: %s\n", outputPath)
			fmt.Printf("  Project:  %s\n", projectName)
			fmt.Println()

			color.New(color.Bold).Println("Next steps:")
			fmt.Println("  1. Review and customize the manifest:")
			fmt.Printf("     vi %s\n", outputFile)
			fmt.Println()
			fmt.Println("  2. Start the Orca control plane (if not running):")
			fmt.Println("     orca serve")
			fmt.Println()
			fmt.Println("  3. Apply the manifest:")
			fmt.Printf("     orca apply -f %s\n", outputFile)
			fmt.Println()
			fmt.Println("  4. Check status:")
			fmt.Println("     orca status")
			fmt.Println("     orca get pods")
			fmt.Println()
			fmt.Println("  5. Run a task:")
			fmt.Println("     orca run -- \"Write a hello world program\"")

			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&outputFile, "output-file", "project.yaml", "Output manifest filename")

	return cmd
}
