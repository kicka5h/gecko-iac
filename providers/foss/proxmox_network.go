package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

func (p *ProxmoxProvider) createNetwork(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	cfg := networkCfgFromInputs(args.Inputs, args.Name)

	if err := p.api.CreateNetwork(ctx, cfg); err != nil {
		return nil, err
	}
	if err := p.api.ReloadNetwork(ctx); err != nil {
		return nil, err
	}

	// ExternalID = "<node>/<iface>" since iface names are per-node.
	externalID := p.node + "/" + cfg.Iface
	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"iface": cfg.Iface, "node": p.node},
		ExternalID: externalID,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readNetwork(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	_, iface, err := splitNetworkID(externalID)
	if err != nil {
		return nil, err
	}

	cfg, err := p.api.ReadNetwork(ctx, iface)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}

	nodeName, _, _ := splitNetworkID(externalID)
	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:network",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"iface":        cfg.Iface,
			"type":         cfg.Type,
			"cidr":         cfg.CIDR,
			"gateway":      cfg.Gateway,
			"bridge_ports": cfg.BridgePorts,
			"comments":     cfg.Comments,
			"autostart":    cfg.Autostart,
		},
		Outputs:   core.Outputs{"iface": cfg.Iface, "node": nodeName},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateNetwork(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	_, iface, err := splitNetworkID(current.ExternalID)
	if err != nil {
		return nil, err
	}

	cfg := networkCfgFromInputs(desired.Inputs, iface)
	if err := p.api.UpdateNetwork(ctx, iface, cfg); err != nil {
		return nil, err
	}
	if err := p.api.ReloadNetwork(ctx); err != nil {
		return nil, err
	}

	return p.readNetwork(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deleteNetwork(ctx context.Context, state *core.ResourceState) error {
	_, iface, err := splitNetworkID(state.ExternalID)
	if err != nil {
		return err
	}

	if err := p.api.DeleteNetwork(ctx, iface); err != nil {
		return err
	}
	return p.api.ReloadNetwork(ctx)
}

// networkCfgFromInputs builds a NetworkConfig from Scute inputs.
func networkCfgFromInputs(inputs core.Inputs, name string) NetworkConfig {
	return NetworkConfig{
		Iface:       getStringInputFallback(inputs, "iface", name),
		Type:        getStringInputFallback(inputs, "type", "bridge"),
		CIDR:        func() string { v, _ := inputs["cidr"].(string); return v }(),
		Gateway:     func() string { v, _ := inputs["gateway"].(string); return v }(),
		BridgePorts: func() string { v, _ := inputs["bridge_ports"].(string); return v }(),
		Comments:    func() string { v, _ := inputs["comments"].(string); return v }(),
		Autostart:   func() bool { v, _ := inputs["autostart"].(bool); return v }(),
	}
}

// splitNetworkID splits "<node>/<iface>" externalID into its parts.
func splitNetworkID(externalID string) (nodeName, iface string, err error) {
	idx := strings.LastIndex(externalID, "/")
	if idx <= 0 {
		return "", "", fmt.Errorf("proxmox: invalid network externalID %q (expected <node>/<iface>)", externalID)
	}
	return externalID[:idx], externalID[idx+1:], nil
}
