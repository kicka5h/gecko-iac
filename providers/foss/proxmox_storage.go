package k8s

import (
	"context"
	"fmt"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

func (p *ProxmoxProvider) createStorage(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	name := getStringInputFallback(args.Inputs, "storage", args.Name)
	opts := storageOptionsFromInputs(args.Inputs, name)

	if err := p.api.CreateStorage(ctx, opts); err != nil {
		return nil, fmt.Errorf("proxmox: create storage %q: %w", name, err)
	}

	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"storage": name},
		ExternalID: name,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readStorage(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadStorage(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:storage",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"storage": info.Name,
			"type":    info.Type,
			"content": info.Content,
		},
		Outputs:   core.Outputs{"storage": info.Name},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateStorage(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	name := current.ExternalID
	opts := storageOptionsFromInputs(desired.Inputs, name)

	if err := p.api.UpdateStorage(ctx, name, opts); err != nil {
		return nil, err
	}

	return p.readStorage(ctx, proxmoxResourceID(desired), name)
}

func (p *ProxmoxProvider) deleteStorage(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteStorage(ctx, state.ExternalID)
}

// storageOptionsFromInputs maps Scute inputs to ClusterStorageOptions.
func storageOptionsFromInputs(inputs core.Inputs, name string) []proxmox.ClusterStorageOptions {
	var opts []proxmox.ClusterStorageOptions

	opts = append(opts, proxmox.ClusterStorageOptions{Name: "storage", Value: name})

	if v, ok := inputs["type"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "type", Value: v})
	}
	if v, ok := inputs["content"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "content", Value: v})
	}
	if v, ok := inputs["path"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "path", Value: v})
	}
	if v, ok := inputs["server"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "server", Value: v})
	}
	if v, ok := inputs["export"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "export", Value: v})
	}
	if v, ok := inputs["nodes"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "nodes", Value: v})
	}

	return opts
}
