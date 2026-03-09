package cross

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// pulumiState represents a Pulumi stack state file (version 3).
type pulumiState struct {
	Version    int              `json:"version"`
	Deployment pulumiDeployment `json:"deployment"`
}

type pulumiDeployment struct {
	Manifest  pulumiManifest   `json:"manifest"`
	Resources []pulumiResource `json:"resources"`
}

type pulumiManifest struct {
	Version string `json:"version"`
}

type pulumiResource struct {
	URN    string                 `json:"urn"`
	Type   string                 `json:"type"`
	ID     string                 `json:"id"`
	Custom bool                   `json:"custom"`
	Inputs map[string]interface{} `json:"inputs"`
	Parent string                 `json:"parent"`
}

// ParsePulumiState reads a Pulumi stack state file (v3) and returns a CrossResult.
func ParsePulumiState(path string) (*CrossResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read state file: %w", err)
	}

	var state pulumiState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid Pulumi state JSON: %w", err)
	}
	if state.Version < 3 {
		return nil, fmt.Errorf("unsupported Pulumi state version %d (need >= 3)", state.Version)
	}

	result := &CrossResult{
		SourceFile:  path,
		Kind:        SourcePulumi,
		ToolVersion: state.Deployment.Manifest.Version,
		ImportedAt:  time.Now(),
	}

	for _, r := range state.Deployment.Resources {
		// Skip component resources, the stack root, and Pulumi internal types
		if !r.Custom || strings.HasPrefix(r.Type, "pulumi:") {
			continue
		}

		name := urnName(r.URN)
		inputs := cleanPulumiInputs(r.Inputs)

		res := &ImportedResource{
			SourceKind: SourcePulumi,
			SourceType: r.Type,
			GeckoType:  MapPulumiType(r.Type),
			Name:       sanitizeName(name),
			ExternalID: r.ID,
			Inputs:     inputs,
			Module:     r.Parent,
		}

		normalizePulumiInputs(r.Type, res)
		result.Resources = append(result.Resources, res)
	}

	return result, nil
}

// urnName extracts the resource name from a Pulumi URN.
// Format: urn:pulumi:<stack>::<project>::<type>::<name>
func urnName(urn string) string {
	parts := strings.Split(urn, "::")
	if len(parts) >= 4 {
		return parts[len(parts)-1]
	}
	return urn
}

// cleanPulumiInputs strips Pulumi internal bookkeeping fields and nil values.
func cleanPulumiInputs(inputs map[string]interface{}) map[string]interface{} {
	internal := map[string]bool{"__defaults": true}
	out := make(map[string]interface{}, len(inputs))
	for k, v := range inputs {
		if internal[k] || v == nil {
			continue
		}
		if m, ok := v.(map[string]interface{}); ok {
			if cleaned := cleanPulumiInputs(m); len(cleaned) > 0 {
				out[k] = cleaned
			}
		} else {
			out[k] = v
		}
	}
	return out
}

// normalizePulumiInputs applies type-specific field extraction for Pulumi resources.
func normalizePulumiInputs(pulumiType string, res *ImportedResource) {
	inputs := res.Inputs

	// All kubernetes resources share the same metadata/spec structure
	if strings.HasPrefix(pulumiType, "kubernetes:") {
		clean := make(map[string]interface{})
		if meta, ok := inputs["metadata"].(map[string]interface{}); ok {
			if n, ok := meta["name"].(string); ok && n != "" {
				clean["name"] = n
				res.Name = sanitizeName(n)
			}
			if ns, ok := meta["namespace"].(string); ok && ns != "" {
				clean["namespace"] = ns
			}
			if labels, ok := meta["labels"]; ok && labels != nil {
				clean["labels"] = labels
			}
			if annotations, ok := meta["annotations"]; ok && annotations != nil {
				clean["annotations"] = annotations
			}
		}
		if spec, ok := inputs["spec"].(map[string]interface{}); ok {
			if replicas, ok := spec["replicas"]; ok {
				clean["replicas"] = replicas
			}
			extractPulumiContainerImage(spec, clean)
		}
		if len(clean) > 0 {
			res.Inputs = clean
		}
		return
	}

	switch pulumiType {
	case "vault:kv/secretV2:SecretV2", "vault:kv/secret:Secret":
		clean := make(map[string]interface{})
		for _, k := range []string{"mount", "name", "path", "namespace"} {
			if v, ok := inputs[k]; ok {
				clean[k] = v
			}
		}
		res.Inputs = clean

	case "vault:index/policy:Policy":
		clean := make(map[string]interface{})
		if name, ok := inputs["name"].(string); ok {
			clean["name"] = name
			res.Name = sanitizeName(name)
		}
		if policy, ok := inputs["policy"].(string); ok {
			clean["policy"] = policy
		}
		res.Inputs = clean

	case "nomad:index/job:Job":
		clean := make(map[string]interface{})
		for _, k := range []string{"name", "jobspec", "detach"} {
			if v, ok := inputs[k]; ok {
				clean[k] = v
			}
		}
		res.Inputs = clean
	}
}

// extractPulumiContainerImage walks a k8s deployment spec to find the first container image.
func extractPulumiContainerImage(spec map[string]interface{}, out map[string]interface{}) {
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	podSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return
	}
	containers, ok := podSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return
	}
	if c, ok := containers[0].(map[string]interface{}); ok {
		if img, ok := c["image"].(string); ok && img != "" {
			out["image"] = img
		}
	}
}
