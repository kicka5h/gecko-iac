// Package k8s provides the Gecko Kubernetes provider.
// Manages Kubernetes resources across any conformant cluster (k3s, k0s, Talos, etc.)
package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// Provider implements core.Provider for Kubernetes clusters
type Provider struct {
	endpoint   string
	kubeconfig string
	namespace  string
	context_   string
}

// NewProvider creates a new Kubernetes provider with optional config overrides
func NewProvider(config map[string]interface{}) *Provider {
	p := &Provider{
		kubeconfig: "~/.kube/config",
		namespace:  "default",
	}
	if config != nil {
		if v, ok := config["kubeconfig"].(string); ok {
			p.kubeconfig = v
		}
		if v, ok := config["namespace"].(string); ok {
			p.namespace = v
		}
		if v, ok := config["context"].(string); ok {
			p.context_ = v
		}
		if v, ok := config["endpoint"].(string); ok {
			p.endpoint = v
		}
	}
	return p
}

func (p *Provider) Name() string    { return "k8s" }
func (p *Provider) Version() string { return "0.1.0" }

func (p *Provider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"k8s:namespace",
		"k8s:deployment",
		"k8s:statefulset",
		"k8s:daemonset",
		"k8s:service",
		"k8s:ingress",
		"k8s:configmap",
		"k8s:secret",
		"k8s:persistentvolumeclaim",
		"k8s:persistentvolume",
		"k8s:storageclass",
		"k8s:serviceaccount",
		"k8s:clusterrole",
		"k8s:clusterrolebinding",
		"k8s:role",
		"k8s:rolebinding",
		"k8s:horizontalpodautoscaler",
		"k8s:poddisruptionbudget",
		"k8s:networkpolicy",
		"k8s:job",
		"k8s:cronjob",
		"k8s:customresource",
	}
}

// Configure validates provider configuration and establishes connectivity
func (p *Provider) Configure(ctx context.Context, config map[string]interface{}) error {
	if v, ok := config["kubeconfig"].(string); ok {
		p.kubeconfig = v
	}
	// In a real implementation: parse kubeconfig, test connection
	return nil
}

// Validate checks that resource args are valid for this provider
func (p *Provider) Validate(ctx context.Context, args core.ResourceArgs) error {
	if args.Name == "" {
		return fmt.Errorf("k8s resource name cannot be empty")
	}
	for _, t := range p.SupportedTypes() {
		if t == args.Type {
			return nil
		}
	}
	return fmt.Errorf("unsupported resource type: %s", args.Type)
}

// Create provisions a new Kubernetes resource
func (p *Provider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	// Real implementation: build a Kubernetes manifest from args.Inputs and apply via client-go
	namespace := getStringInput(args.Inputs, "namespace", p.namespace)
	externalID := fmt.Sprintf("%s/%s", namespace, args.Name)

	state := &core.ResourceState{
		ID:         core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name)),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    p.deriveOutputs(args),
		Tags:       args.Tags,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		ProviderID: p.Name(),
		ExternalID: externalID,
	}
	return state, nil
}

// Read retrieves the current state of a Kubernetes resource
func (p *Provider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// Real implementation: GET resource via Kubernetes API and map to ResourceState
	return nil, fmt.Errorf("not yet implemented: Read")
}

// Update applies changes to an existing Kubernetes resource
func (p *Provider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// Real implementation: apply a strategic merge patch or server-side apply
	updated := *current
	updated.Inputs = desired.Inputs
	updated.Status = core.StatusRunning
	updated.UpdatedAt = time.Now()
	return &updated, nil
}

// Delete removes a Kubernetes resource
func (p *Provider) Delete(ctx context.Context, state *core.ResourceState) error {
	// Real implementation: DELETE via Kubernetes API with graceful period
	return nil
}

// Import reads an existing resource into Gecko state
func (p *Provider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	// Real implementation: GET the resource and map its live state to ResourceState
	return &core.ResourceState{
		Type:       resourceType,
		ExternalID: externalID,
		Status:     core.StatusRunning,
		ProviderID: p.Name(),
		Inputs:     core.Inputs{},
		Outputs:    core.Outputs{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

// Diff computes what would change between current state and desired args
func (p *Provider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	if current == nil {
		// Resource doesn't exist yet → Create
		return &core.Diff{
			ResourceID: core.ResourceID(fmt.Sprintf("%s::%s", desired.Type, desired.Name)),
			Kind:       core.ChangeAdd,
		}, nil
	}

	// Compare inputs
	var changes []core.FieldChange
	for k, desiredVal := range desired.Inputs {
		currentVal, exists := current.Inputs[k]
		if !exists {
			changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeAdd, NewValue: desiredVal})
		} else if fmt.Sprintf("%v", currentVal) != fmt.Sprintf("%v", desiredVal) {
			changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeUpdate, OldValue: currentVal, NewValue: desiredVal})
		}
	}
	// Check for removed inputs
	for k, currentVal := range current.Inputs {
		if _, exists := desired.Inputs[k]; !exists {
			changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeDelete, OldValue: currentVal})
		}
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
		// Some fields require replace (immutable fields like namespace, name, etc.)
		for _, c := range changes {
			if c.Field == "namespace" || c.Field == "type" {
				return &core.Diff{
					ResourceID:      current.ID,
					Kind:            core.ChangeReplace,
					Changes:         changes,
					RequiresReplace: true,
				}, nil
			}
		}
	}

	return &core.Diff{
		ResourceID: current.ID,
		Kind:       kind,
		Changes:    changes,
	}, nil
}

// deriveOutputs computes known outputs for a resource after creation
func (p *Provider) deriveOutputs(args core.ResourceArgs) core.Outputs {
	outputs := core.Outputs{}
	switch args.Type {
	case "k8s:service":
		outputs["cluster_ip"] = "10.96.0.1" // would be real IP from API response
		outputs["port"] = getInput(args.Inputs, "port", 80)
	case "k8s:deployment":
		outputs["ready_replicas"] = getInput(args.Inputs, "replicas", 1)
		outputs["available_replicas"] = getInput(args.Inputs, "replicas", 1)
	case "k8s:persistentvolumeclaim":
		outputs["status"] = "Bound"
		outputs["volume_name"] = fmt.Sprintf("pvc-%s", args.Name)
	}
	return outputs
}

func getStringInput(inputs core.Inputs, key, def string) string {
	if v, ok := inputs[key].(string); ok {
		return v
	}
	return def
}

func getInput(inputs core.Inputs, key string, def interface{}) interface{} {
	if v, ok := inputs[key]; ok {
		return v
	}
	return def
}
