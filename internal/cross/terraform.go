package cross

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// tfState represents the top-level Terraform state file structure (version 4).
type tfState struct {
	Version          int          `json:"version"`
	TerraformVersion string       `json:"terraform_version"`
	Resources        []tfResource `json:"resources"`
}

type tfResource struct {
	Module    string       `json:"module"`
	Mode      string       `json:"mode"` // "managed" or "data"
	Type      string       `json:"type"`
	Name      string       `json:"name"`
	Instances []tfInstance `json:"instances"`
}

type tfInstance struct {
	Attributes map[string]interface{} `json:"attributes"`
}

// ParseTerraformState reads a Terraform state file (v4) and returns a CrossResult.
func ParseTerraformState(path string) (*CrossResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read state file: %w", err)
	}

	var state tfState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid Terraform state JSON: %w", err)
	}
	if state.Version < 4 {
		return nil, fmt.Errorf("unsupported Terraform state version %d (need >= 4)", state.Version)
	}

	result := &CrossResult{
		SourceFile:  path,
		Kind:        SourceTerraform,
		ToolVersion: state.TerraformVersion,
		ImportedAt:  time.Now(),
	}

	for _, r := range state.Resources {
		if r.Mode != "managed" {
			continue // skip data sources
		}
		for i, inst := range r.Instances {
			name := r.Name
			if len(r.Instances) > 1 {
				name = fmt.Sprintf("%s-%d", r.Name, i)
			}

			attrs := flattenTFAttrs(inst.Attributes)
			externalID, _ := attrs["id"].(string)
			delete(attrs, "id")
			delete(attrs, "timeouts")

			res := &ImportedResource{
				SourceKind: SourceTerraform,
				SourceType: r.Type,
				GeckoType:  MapTerraformType(r.Type),
				Name:       sanitizeName(name),
				ExternalID: externalID,
				Inputs:     attrs,
				Module:     r.Module,
			}

			normalizeTFInputs(r.Type, res)
			result.Resources = append(result.Resources, res)
		}
	}

	return result, nil
}

// flattenTFAttrs flattens Terraform's block-as-array convention.
// Single-element arrays of objects are unwrapped to maps; empty values are pruned.
func flattenTFAttrs(attrs map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		if flat := flattenTFValue(v); flat != nil {
			out[k] = flat
		}
	}
	return out
}

func flattenTFValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		if len(val) == 0 {
			return nil
		}
		// Single-element block arrays are unwrapped (the Terraform block-as-array pattern)
		if len(val) == 1 {
			if m, ok := val[0].(map[string]interface{}); ok {
				flat := flattenTFAttrs(m)
				if len(flat) == 0 {
					return nil
				}
				return flat
			}
		}
		// Multi-element: flatten each element individually
		result := make([]interface{}, 0, len(val))
		for _, el := range val {
			if flat := flattenTFValue(el); flat != nil {
				result = append(result, flat)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result

	case map[string]interface{}:
		flat := flattenTFAttrs(val)
		if len(flat) == 0 {
			return nil
		}
		return flat

	case string:
		if val == "" {
			return nil
		}
		return val

	case float64, bool:
		return val

	case nil:
		return nil
	}
	return v
}

// normalizeTFInputs applies type-specific extraction to pull the most meaningful
// fields out of a flattened Terraform attributes map.
func normalizeTFInputs(tfType string, res *ImportedResource) {
	attrs := res.Inputs

	switch {
	case tfType == "gitea_repository":
		clean := make(map[string]interface{})
		for _, k := range []string{"name", "owner", "description", "private", "auto_init", "default_branch"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "minio_s3_bucket":
		clean := make(map[string]interface{})
		for _, k := range []string{"bucket", "acl", "force_destroy"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if b, ok := attrs["bucket"].(string); ok {
			res.Name = sanitizeName(b)
		}
		res.Inputs = clean

	case tfType == "postgresql_database" || tfType == "postgresql_role":
		clean := make(map[string]interface{})
		for _, k := range []string{"name", "owner", "encoding", "lc_collate", "lc_ctype", "login", "superuser"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean
	}
}

// sanitizeName converts an arbitrary string into a valid Scute resource name.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-_")
}
