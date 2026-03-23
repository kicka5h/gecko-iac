package foss

import (
	"context"
	"fmt"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
)

// realProxmoxAPI implements proxmoxAPI using the live go-proxmox client.
type realProxmoxAPI struct {
	client *proxmox.Client
	node   string
}

func newRealProxmoxAPI(client *proxmox.Client, node string) *realProxmoxAPI {
	return &realProxmoxAPI{client: client, node: node}
}

func (r *realProxmoxAPI) pveNode(ctx context.Context) (*proxmox.Node, error) {
	n, err := r.client.Node(ctx, r.node)
	if err != nil {
		return nil, fmt.Errorf("proxmox: get node %q: %w", r.node, err)
	}
	return n, nil
}

func (r *realProxmoxAPI) wait(ctx context.Context, task *proxmox.Task) error {
	if task == nil {
		return nil
	}
	if err := task.Wait(ctx, 2*time.Second, 10*time.Minute); err != nil {
		return fmt.Errorf("proxmox: task failed: %w", err)
	}
	return nil
}

// ── Cluster ───────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) NextVMID(ctx context.Context) (int, error) {
	cluster, err := r.client.Cluster(ctx)
	if err != nil {
		return 0, fmt.Errorf("proxmox: get cluster: %w", err)
	}
	id, err := cluster.NextID(ctx)
	if err != nil {
		return 0, fmt.Errorf("proxmox: next vmid: %w", err)
	}
	return id, nil
}

// ── VM ────────────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	task, err := node.NewVirtualMachine(ctx, vmid, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: create vm %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) ReadVM(ctx context.Context, vmid int) (*VMInfo, error) {
	node, err := r.pveNode(ctx)
	if err != nil {
		return nil, err
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read vm %d: %w", vmid, err)
	}

	info := &VMInfo{
		VMID:   vmid,
		Name:   vm.Name,
		Status: vm.Status,
	}
	if vm.VirtualMachineConfig != nil {
		cfg := vm.VirtualMachineConfig
		info.Cores = cfg.Cores
		if nets := cfg.MergeNets(); nets != nil {
			info.Net0 = nets["net0"]
		}
	}
	return info, nil
}

func (r *realProxmoxAPI) UpdateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch vm %d for update: %w", vmid, err)
	}
	task, err := vm.Config(ctx, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: update vm %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) StartVM(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch vm %d for start: %w", vmid, err)
	}
	task, err := vm.Start(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: start vm %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) StopVM(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch vm %d for stop: %w", vmid, err)
	}
	task, err := vm.Stop(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: stop vm %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) DeleteVM(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	vm, err := node.VirtualMachine(ctx, vmid)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("proxmox: fetch vm %d for delete: %w", vmid, err)
	}
	task, err := vm.Delete(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: delete vm %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

// ── LXC ───────────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	task, err := node.NewContainer(ctx, vmid, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: create lxc %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) ReadLXC(ctx context.Context, vmid int) (*LXCInfo, error) {
	node, err := r.pveNode(ctx)
	if err != nil {
		return nil, err
	}
	ct, err := node.Container(ctx, vmid)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read lxc %d: %w", vmid, err)
	}

	info := &LXCInfo{
		VMID:   vmid,
		Status: ct.Status,
	}
	if ct.ContainerConfig != nil {
		info.Hostname = ct.ContainerConfig.Hostname
		info.Memory = ct.ContainerConfig.Memory
		info.Cores = ct.ContainerConfig.Cores
	}
	return info, nil
}

func (r *realProxmoxAPI) UpdateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	ct, err := node.Container(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch lxc %d for update: %w", vmid, err)
	}
	task, err := ct.Config(ctx, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: update lxc %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) StartLXC(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	ct, err := node.Container(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch lxc %d for start: %w", vmid, err)
	}
	task, err := ct.Start(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: start lxc %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) StopLXC(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	ct, err := node.Container(ctx, vmid)
	if err != nil {
		return fmt.Errorf("proxmox: fetch lxc %d for stop: %w", vmid, err)
	}
	task, err := ct.Stop(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: stop lxc %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) DeleteLXC(ctx context.Context, vmid int) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	ct, err := node.Container(ctx, vmid)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("proxmox: fetch lxc %d for delete: %w", vmid, err)
	}
	task, err := ct.Delete(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: delete lxc %d: %w", vmid, err)
	}
	return r.wait(ctx, task)
}

// ── Storage ───────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateStorage(ctx context.Context, opts []proxmox.ClusterStorageOptions) error {
	task, err := r.client.NewClusterStorage(ctx, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: create storage: %w", err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) ReadStorage(ctx context.Context, name string) (*StorageInfo, error) {
	s, err := r.client.ClusterStorage(ctx, name)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read storage %q: %w", name, err)
	}
	return &StorageInfo{Name: s.Storage, Type: s.Type, Content: s.Content}, nil
}

func (r *realProxmoxAPI) UpdateStorage(ctx context.Context, name string, opts []proxmox.ClusterStorageOptions) error {
	task, err := r.client.UpdateClusterStorage(ctx, name, opts...)
	if err != nil {
		return fmt.Errorf("proxmox: update storage %q: %w", name, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) DeleteStorage(ctx context.Context, name string) error {
	task, err := r.client.DeleteClusterStorage(ctx, name)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete storage %q: %w", name, err)
	}
	return r.wait(ctx, task)
}

// ── Network ───────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateNetwork(ctx context.Context, cfg NetworkConfig) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	nw := networkConfigToNodeNetwork(cfg, r.node, node)
	task, err := node.NewNetwork(ctx, nw)
	if err != nil {
		return fmt.Errorf("proxmox: create network %q: %w", cfg.Iface, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) ReadNetwork(ctx context.Context, iface string) (*NetworkConfig, error) {
	node, err := r.pveNode(ctx)
	if err != nil {
		return nil, err
	}
	nw, err := node.Network(ctx, iface)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read network %q: %w", iface, err)
	}
	return nodeNetworkToConfig(nw), nil
}

func (r *realProxmoxAPI) UpdateNetwork(ctx context.Context, iface string, cfg NetworkConfig) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	nw, err := node.Network(ctx, iface)
	if err != nil {
		return fmt.Errorf("proxmox: fetch network %q for update: %w", iface, err)
	}
	applyNetworkConfig(nw, cfg)
	if err := nw.Update(ctx); err != nil {
		return fmt.Errorf("proxmox: update network %q: %w", iface, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteNetwork(ctx context.Context, iface string) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	nw, err := node.Network(ctx, iface)
	if err != nil {
		if proxmox.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("proxmox: fetch network %q for delete: %w", iface, err)
	}
	task, err := nw.Delete(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: delete network %q: %w", iface, err)
	}
	return r.wait(ctx, task)
}

func (r *realProxmoxAPI) ReloadNetwork(ctx context.Context) error {
	node, err := r.pveNode(ctx)
	if err != nil {
		return err
	}
	task, err := node.NetworkReload(ctx)
	if err != nil {
		return fmt.Errorf("proxmox: reload network: %w", err)
	}
	return r.wait(ctx, task)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func networkConfigToNodeNetwork(cfg NetworkConfig, nodeName string, node *proxmox.Node) *proxmox.NodeNetwork {
	nw := &proxmox.NodeNetwork{
		Node:        nodeName,
		NodeAPI:     node,
		Iface:       cfg.Iface,
		Type:        cfg.Type,
		CIDR:        cfg.CIDR,
		Gateway:     cfg.Gateway,
		BridgePorts: cfg.BridgePorts,
		Comments:    cfg.Comments,
	}
	if cfg.Autostart {
		nw.Autostart = 1
	}
	return nw
}

func nodeNetworkToConfig(nw *proxmox.NodeNetwork) *NetworkConfig {
	return &NetworkConfig{
		Iface:       nw.Iface,
		Type:        nw.Type,
		CIDR:        nw.CIDR,
		Gateway:     nw.Gateway,
		BridgePorts: nw.BridgePorts,
		Comments:    nw.Comments,
		Autostart:   nw.Autostart == 1,
	}
}

func applyNetworkConfig(nw *proxmox.NodeNetwork, cfg NetworkConfig) {
	nw.CIDR = cfg.CIDR
	nw.Gateway = cfg.Gateway
	nw.BridgePorts = cfg.BridgePorts
	nw.Comments = cfg.Comments
	if cfg.Autostart {
		nw.Autostart = 1
	} else {
		nw.Autostart = 0
	}
}
