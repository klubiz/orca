package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	v1alpha1 "github.com/klubi/orca/pkg/apis/v1alpha1"
	"github.com/klubi/orca/pkg/manifest"
)

func newApplyCmd() *cobra.Command {
	var filename string

	cmd := &cobra.Command{
		Use:   "apply -f <file>",
		Short: "Apply a manifest file",
		Long:  "Create or update resources from a YAML manifest file.",
		Example: `  orca apply -f project.yaml
  orca apply -f agents.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resources, err := manifest.ParseFile(filename)
			if err != nil {
				return fmt.Errorf("parsing manifest %s: %w", filename, err)
			}

			if len(resources) == 0 {
				fmt.Println("No resources found in manifest.")
				return nil
			}

			for _, resource := range resources {
				kind, name := resourceIdentity(resource)

				_, err := apiClient.Apply(resource)
				if err != nil {
					return fmt.Errorf("applying %s/%s: %w", kind, name, err)
				}

				fmt.Printf("%s/%s configured\n", kind, name)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filename, "filename", "f", "", "Path to manifest file (required)")
	cmd.MarkFlagRequired("filename")

	return cmd
}

// resourceIdentity extracts the kind and name from a typed resource.
func resourceIdentity(resource interface{}) (kind, name string) {
	switch r := resource.(type) {
	case *v1alpha1.Project:
		return r.Kind, r.Metadata.Name
	case *v1alpha1.AgentPod:
		return r.Kind, r.Metadata.Name
	case *v1alpha1.AgentPool:
		return r.Kind, r.Metadata.Name
	case *v1alpha1.DevTask:
		return r.Kind, r.Metadata.Name
	default:
		return "Unknown", "unknown"
	}
}
