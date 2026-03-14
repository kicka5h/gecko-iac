package k8s

import (
	"context"
	"fmt"
	"strconv"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

func (p *ProxmoxProvider) createVM(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	vmid, hasID := toInt(args.Inputs["vmid"])
	if !hasID || vmid == 0 {
		var err error
		vmid, err = p.api.NextVMID(ctx)
		if err != nil {
			return nil, err
		}
	}

	opts := vmOptionsFromInputs(args.Inputs, args.Name)
	if err := p.api.CreateVM(ctx, vmid, opts); err != nil {
		return nil, fmt.Errorf("proxmox: create vm %q: %w", args.Name, err)
	}

	if start, _ := args.Inputs["start"].(bool); start {
		if err := p.api.StartVM(ctx, vmid); err != nil {
			return nil, err
		}
	}

	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"vmid": vmid},
		ExternalID: strconv.Itoa(vmid),
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readVM(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	vmid, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("proxmox: invalid vm externalID %q: %w", externalID, err)
	}

	info, err := p.api.ReadVM(ctx, vmid)
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
		Type:       "proxmox:vm",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     status,
		Inputs: core.Inputs{
			"vmid":   info.VMID,
			"name":   info.Name,
			"memory": info.Memory,
			"cores":  info.Cores,
			"net0":   info.Net0,
		},
		Outputs:   core.Outputs{"vmid": vmid},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateVM(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	vmid, err := strconv.Atoi(current.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("proxmox: invalid vm externalID %q: %w", current.ExternalID, err)
	}

	opts := vmOptionsFromInputs(desired.Inputs, desired.Name)
	if err := p.api.UpdateVM(ctx, vmid, opts); err != nil {
		return nil, err
	}

	return p.readVM(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deleteVM(ctx context.Context, state *core.ResourceState) error {
	vmid, err := strconv.Atoi(state.ExternalID)
	if err != nil {
		return fmt.Errorf("proxmox: invalid vm externalID %q: %w", state.ExternalID, err)
	}

	// Check if running so we can stop first.
	info, err := p.api.ReadVM(ctx, vmid)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	if info.Status == "running" {
		if err := p.api.StopVM(ctx, vmid); err != nil {
			return err
		}
	}

	return p.api.DeleteVM(ctx, vmid)
}

// vmOptionsFromInputs maps Scute inputs to VirtualMachineOptions.
func vmOptionsFromInputs(inputs core.Inputs, name string) []proxmox.VirtualMachineOption {
	var opts []proxmox.VirtualMachineOption

	vmName := getStringInputFallback(inputs, "name", name)
	opts = append(opts, proxmox.VirtualMachineOption{Name: "name", Value: vmName})

	if v, ok := toInt(inputs["memory"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "memory", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["cores"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cores", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["sockets"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "sockets", Value: strconv.Itoa(v)})
	}
	if v, ok := inputs["iso"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cdrom", Value: v})
	}
	if v, ok := inputs["net0"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "net0", Value: v})
	}
	if v, ok := inputs["disk_size"].(string); ok && v != "" {
		storage := getStringInputFallback(inputs, "storage", "local-lvm")
		opts = append(opts, proxmox.VirtualMachineOption{Name: "scsi0", Value: storage + ":" + v})
		opts = append(opts, proxmox.VirtualMachineOption{Name: "scsihw", Value: "virtio-scsi-pci"})
	}

	return opts
}
