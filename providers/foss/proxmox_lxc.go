package k8s

import (
	"context"
	"fmt"
	"strconv"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

func (p *ProxmoxProvider) createLXC(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	vmid, hasID := toInt(args.Inputs["vmid"])
	if !hasID || vmid == 0 {
		var err error
		vmid, err = p.api.NextVMID(ctx)
		if err != nil {
			return nil, err
		}
	}

	opts := lxcOptionsFromInputs(args.Inputs, args.Name)
	if err := p.api.CreateLXC(ctx, vmid, opts); err != nil {
		return nil, fmt.Errorf("proxmox: create lxc %q: %w", args.Name, err)
	}

	if start, _ := args.Inputs["start"].(bool); start {
		if err := p.api.StartLXC(ctx, vmid); err != nil {
			return nil, err
		}
	}

	// ExternalID uses "lxc/<vmid>" to avoid collisions with VMs sharing the same numeric ID.
	externalID := "lxc/" + strconv.Itoa(vmid)
	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"vmid": vmid},
		ExternalID: externalID,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readLXC(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	vmid, err := lxcVMID(externalID)
	if err != nil {
		return nil, err
	}

	info, err := p.api.ReadLXC(ctx, vmid)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	status := core.StatusRunning
	if info.Status == "stopped" {
		status = core.StatusPending
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:lxc",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     status,
		Inputs: core.Inputs{
			"vmid":     info.VMID,
			"hostname": info.Hostname,
			"memory":   info.Memory,
			"cores":    info.Cores,
		},
		Outputs:   core.Outputs{"vmid": vmid},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateLXC(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	vmid, err := lxcVMID(current.ExternalID)
	if err != nil {
		return nil, err
	}

	opts := lxcOptionsFromInputs(desired.Inputs, desired.Name)
	if err := p.api.UpdateLXC(ctx, vmid, opts); err != nil {
		return nil, err
	}

	return p.readLXC(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deleteLXC(ctx context.Context, state *core.ResourceState) error {
	vmid, err := lxcVMID(state.ExternalID)
	if err != nil {
		return err
	}

	info, err := p.api.ReadLXC(ctx, vmid)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	if info.Status == "running" {
		if err := p.api.StopLXC(ctx, vmid); err != nil {
			return err
		}
	}

	return p.api.DeleteLXC(ctx, vmid)
}

// lxcOptionsFromInputs maps Scute inputs to ContainerOptions.
func lxcOptionsFromInputs(inputs core.Inputs, name string) []proxmox.ContainerOption {
	var opts []proxmox.ContainerOption

	hostname := getStringInputFallback(inputs, "hostname", name)
	opts = append(opts, proxmox.ContainerOption{Name: "hostname", Value: hostname})

	if v, ok := inputs["ostemplate"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "ostemplate", Value: v})
	}
	if v, ok := toInt(inputs["memory"]); ok {
		opts = append(opts, proxmox.ContainerOption{Name: "memory", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["cores"]); ok {
		opts = append(opts, proxmox.ContainerOption{Name: "cores", Value: strconv.Itoa(v)})
	}
	if v, ok := inputs["rootfs"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "rootfs", Value: v})
	}
	if v, ok := inputs["net0"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "net0", Value: v})
	}
	if v, ok := inputs["password"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "password", Value: v})
	}

	return opts
}

// lxcVMID parses the VMID from an LXC externalID ("lxc/<vmid>").
func lxcVMID(externalID string) (int, error) {
	raw := externalID
	if len(externalID) > 4 && externalID[:4] == "lxc/" {
		raw = externalID[4:]
	}
	vmid, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("proxmox: invalid lxc externalID %q: %w", externalID, err)
	}
	return vmid, nil
}
