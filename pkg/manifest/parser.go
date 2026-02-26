// Package manifest provides YAML manifest parsing for Orca resources.
package manifest

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/klubi/orca/pkg/apis/v1alpha1"
	"gopkg.in/yaml.v3"
)

// ParseFile reads a YAML file at the given path and parses it into typed
// Orca resources. Multi-document YAML (separated by ---) is supported.
func ParseFile(path string) ([]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file %s: %w", path, err)
	}
	return ParseBytes(data)
}

// ParseBytes parses raw YAML bytes into typed Orca resources.
// Multi-document YAML (separated by ---) is supported.
func ParseBytes(data []byte) ([]interface{}, error) {
	return parseDocuments(data)
}

// parseDocuments splits multi-document YAML and decodes each document into
// its concrete Orca resource type.
func parseDocuments(data []byte) ([]interface{}, error) {
	var resources []interface{}

	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		// Decode into a generic yaml.Node so we can re-decode it.
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decoding yaml document: %w", err)
		}

		// Skip empty documents.
		if node.Kind == 0 {
			continue
		}

		// First pass: extract TypeMeta to determine the Kind.
		var meta v1alpha1.TypeMeta
		if err := node.Decode(&meta); err != nil {
			return nil, fmt.Errorf("decoding type meta: %w", err)
		}

		// Skip completely empty documents.
		if meta.Kind == "" && meta.APIVersion == "" {
			continue
		}

		// Second pass: decode into the concrete type based on Kind.
		resource, err := decodeResource(&node, meta.Kind)
		if err != nil {
			return nil, err
		}

		// Set default APIVersion if empty.
		setDefaultAPIVersion(resource)

		// Validate required fields.
		if err := validateResource(resource); err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// decodeResource unmarshals a yaml.Node into the correct concrete type
// based on the resource Kind.
func decodeResource(node *yaml.Node, kind string) (interface{}, error) {
	switch kind {
	case v1alpha1.KindProject:
		var r v1alpha1.Project
		if err := node.Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding Project: %w", err)
		}
		return &r, nil

	case v1alpha1.KindAgentPod:
		var r v1alpha1.AgentPod
		if err := node.Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding AgentPod: %w", err)
		}
		return &r, nil

	case v1alpha1.KindAgentPool:
		var r v1alpha1.AgentPool
		if err := node.Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding AgentPool: %w", err)
		}
		return &r, nil

	case v1alpha1.KindDevTask:
		var r v1alpha1.DevTask
		if err := node.Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding DevTask: %w", err)
		}
		return &r, nil

	default:
		return nil, fmt.Errorf("unknown resource kind: %q", kind)
	}
}

// setDefaultAPIVersion sets the APIVersion to the default value if it is empty.
func setDefaultAPIVersion(resource interface{}) {
	switch r := resource.(type) {
	case *v1alpha1.Project:
		if r.APIVersion == "" {
			r.APIVersion = v1alpha1.APIVersion
		}
	case *v1alpha1.AgentPod:
		if r.APIVersion == "" {
			r.APIVersion = v1alpha1.APIVersion
		}
	case *v1alpha1.AgentPool:
		if r.APIVersion == "" {
			r.APIVersion = v1alpha1.APIVersion
		}
	case *v1alpha1.DevTask:
		if r.APIVersion == "" {
			r.APIVersion = v1alpha1.APIVersion
		}
	}
}

// validateResource checks that required fields are set on the resource.
func validateResource(resource interface{}) error {
	switch r := resource.(type) {
	case *v1alpha1.Project:
		if r.Metadata.Name == "" {
			return fmt.Errorf("validation failed: Project name must not be empty")
		}
	case *v1alpha1.AgentPod:
		if r.Metadata.Name == "" {
			return fmt.Errorf("validation failed: AgentPod name must not be empty")
		}
	case *v1alpha1.AgentPool:
		if r.Metadata.Name == "" {
			return fmt.Errorf("validation failed: AgentPool name must not be empty")
		}
	case *v1alpha1.DevTask:
		if r.Metadata.Name == "" {
			return fmt.Errorf("validation failed: DevTask name must not be empty")
		}
	}
	return nil
}
