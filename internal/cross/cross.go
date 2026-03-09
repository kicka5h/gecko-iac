// Package cross implements gecko cross — import Terraform or Pulumi state into Scute.
// It parses foreign state files, maps known resource types to Gecko equivalents,
// and generates ready-to-review .scute declarations.
package cross

import "time"

// SourceKind identifies the origin IaC tool.
type SourceKind string

const (
	SourceTerraform SourceKind = "terraform"
	SourcePulumi    SourceKind = "pulumi"
)

// ImportedResource is a normalized resource extracted from a foreign state file.
type ImportedResource struct {
	SourceKind SourceKind
	SourceType string                 // original type, e.g. "kubernetes_deployment"
	GeckoType  string                 // mapped gecko type, e.g. "k8s:deployment"; empty if unmapped
	Name       string                 // sanitized resource name
	ExternalID string                 // provider-side ID
	Inputs     map[string]interface{} // normalized input fields
	Module     string                 // TF module path, empty for root / Pulumi parent URN
}

// Unmapped returns true when no Gecko provider handles this resource type.
func (r *ImportedResource) Unmapped() bool { return r.GeckoType == "" }

// CrossResult is the parsed, normalized output of a foreign state file.
type CrossResult struct {
	SourceFile  string
	Kind        SourceKind
	ToolVersion string
	ImportedAt  time.Time
	Resources   []*ImportedResource
}

// Mapped returns only resources that have a known Gecko type.
func (r *CrossResult) Mapped() []*ImportedResource {
	var out []*ImportedResource
	for _, res := range r.Resources {
		if !res.Unmapped() {
			out = append(out, res)
		}
	}
	return out
}

// UnmappedResources returns resources with no Gecko type mapping.
func (r *CrossResult) UnmappedResources() []*ImportedResource {
	var out []*ImportedResource
	for _, res := range r.Resources {
		if res.Unmapped() {
			out = append(out, res)
		}
	}
	return out
}
