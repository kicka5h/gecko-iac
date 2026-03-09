package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// KindProvider manages local kind (Kubernetes in Docker) clusters.
// It shells out to the kind CLI, which must be installed and in PATH.
type KindProvider struct {
	config map[string]interface{}
}

// NewKindProvider creates a new kind provider instance.
func NewKindProvider(config map[string]interface{}) *KindProvider {
	if config == nil {
		config = map[string]interface{}{}
	}
	return &KindProvider{config: config}
}

func (p *KindProvider) Name() string    { return "kind" }
func (p *KindProvider) Version() string { return "0.1.0" }
func (p *KindProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{"kind:cluster"}
}

// Configure verifies that the kind CLI is available.
func (p *KindProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		p.config = config
	}
	if _, err := exec.LookPath("kind"); err != nil {
		return fmt.Errorf("kind CLI not found in PATH — install it from https://kind.sigs.k8s.io/")
	}
	return nil
}

// Validate checks that required inputs are present.
func (p *KindProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil // name defaults to the resource name
}

// ── helpers ───────────────────────────────────────────────────────────────────

func kindClusterNameFrom(inputs core.Inputs, fallback string) string {
	if v, ok := inputs["name"].(string); ok && v != "" {
		return v
	}
	return fallback
}

func kindListClusters(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "kind", "get", "clusters").Output()
	if err != nil {
		// "No kind clusters found." exits 0, so an error here is unexpected
		return nil, fmt.Errorf("kind get clusters: %w", err)
	}
	var clusters []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			clusters = append(clusters, line)
		}
	}
	return clusters, nil
}

func kindClusterExists(ctx context.Context, name string) (bool, error) {
	clusters, err := kindListClusters(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range clusters {
		if c == name {
			return true, nil
		}
	}
	return false, nil
}

func kindResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

func kindState(args core.ResourceArgs, clusterName string, status core.ResourceStatus) *core.ResourceState {
	return &core.ResourceState{
		ID:         kindResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     status,
		Inputs:     args.Inputs,
		Outputs: core.Outputs{
			"cluster_name": clusterName,
			"context":      "kind-" + clusterName,
			"kubeconfig":   "~/.kube/config",
		},
		ExternalID: clusterName,
		ProviderID: "kind",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// ── CRUD ─────────────────────────────────────────────────────────────────────

// Create runs `kind create cluster --name <name>`.
func (p *KindProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	clusterName := kindClusterNameFrom(args.Inputs, args.Name)

	cmdArgs := []string{"create", "cluster", "--name", clusterName, "--wait", "60s"}

	if img, ok := args.Inputs["node_image"].(string); ok && img != "" {
		cmdArgs = append(cmdArgs, "--image", img)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "kind", cmdArgs...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kind create cluster %q: %s\n%s", clusterName, err, stderr.String())
	}

	return kindState(args, clusterName, core.StatusRunning), nil
}

// Read checks whether the kind cluster exists and returns its state.
func (p *KindProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	clusterName := externalID
	if clusterName == "" {
		// externalID is the cluster name; fall back to the resource name part of the ID
		parts := strings.SplitN(string(id), "::", 2)
		if len(parts) == 2 {
			clusterName = parts[1]
		} else {
			clusterName = string(id)
		}
	}

	exists, err := kindClusterExists(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil // not found
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "kind:cluster",
		Name:       clusterName,
		Status:     core.StatusRunning,
		ExternalID: clusterName,
		ProviderID: "kind",
		Outputs: core.Outputs{
			"cluster_name": clusterName,
			"context":      "kind-" + clusterName,
			"kubeconfig":   "~/.kube/config",
		},
		UpdatedAt: time.Now(),
	}, nil
}

// Update is a no-op — kind clusters cannot be mutated in-place.
func (p *KindProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	clusterName := kindClusterNameFrom(desired.Inputs, desired.Name)
	return kindState(desired, clusterName, core.StatusRunning), nil
}

// Delete runs `kind delete cluster --name <name>`.
func (p *KindProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	clusterName := state.ExternalID
	if clusterName == "" {
		clusterName = state.Name
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", clusterName)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind delete cluster %q: %s\n%s", clusterName, err, stderr.String())
	}
	return nil
}

// Import looks up an existing kind cluster by name.
func (p *KindProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	exists, err := kindClusterExists(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("kind cluster %q does not exist", externalID)
	}
	return &core.ResourceState{
		Type:       resourceType,
		Name:       externalID,
		ExternalID: externalID,
		ProviderID: "kind",
		Status:     core.StatusRunning,
		Outputs: core.Outputs{
			"cluster_name": externalID,
			"context":      "kind-" + externalID,
			"kubeconfig":   "~/.kube/config",
		},
		UpdatedAt: time.Now(),
	}, nil
}

// Diff computes whether the cluster needs to be created or already exists.
func (p *KindProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := kindResourceID(desired)

	if current == nil {
		// Check if the cluster already exists outside of gecko state
		clusterName := kindClusterNameFrom(desired.Inputs, desired.Name)
		exists, err := kindClusterExists(ctx, clusterName)
		if err != nil {
			return nil, err
		}
		if exists {
			// Already running — treat as no-op (adopt it)
			return &core.Diff{ResourceID: id, Kind: core.ChangeNoOp}, nil
		}
		return &core.Diff{
			ResourceID: id,
			Kind:       core.ChangeAdd,
			Changes: []core.FieldChange{
				{Field: "cluster_name", Kind: core.ChangeAdd, NewValue: clusterName, Computed: false},
			},
		}, nil
	}

	// Cluster is in state — check it still exists
	clusterName := current.ExternalID
	if clusterName == "" {
		clusterName = kindClusterNameFrom(desired.Inputs, desired.Name)
	}

	exists, err := kindClusterExists(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	if !exists {
		// Drifted — cluster was deleted externally
		return &core.Diff{
			ResourceID: id,
			Kind:       core.ChangeAdd,
			Changes: []core.FieldChange{
				{Field: "cluster_name", Kind: core.ChangeAdd, NewValue: clusterName},
			},
		}, nil
	}

	return &core.Diff{ResourceID: id, Kind: core.ChangeNoOp}, nil
}
