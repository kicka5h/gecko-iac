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
	if cfg := vm.VirtualMachineConfig; cfg != nil {
		info.Cores = cfg.Cores
		info.Memory = int(cfg.Memory)
		info.Sockets = cfg.Sockets
		info.CPU = cfg.CPU
		info.Balloon = cfg.Balloon
		info.Numa = cfg.Numa != 0
		info.Vcpus = cfg.Vcpus
		info.Boot = cfg.Boot
		info.OnBoot = cfg.OnBoot != 0
		info.OSType = cfg.OSType
		info.Machine = cfg.Machine
		info.Bios = cfg.Bios
		info.VGA = cfg.VGA
		info.Agent = cfg.Agent
		info.Serial0 = cfg.Serial0
		info.SCSIHW = cfg.SCSIHW
		info.Tags = cfg.Tags
		info.Protection = cfg.Protection != 0
		info.Description = cfg.Description
		info.SCSI0 = cfg.SCSI0
		info.SCSI1 = cfg.SCSI1
		info.SCSI2 = cfg.SCSI2
		info.SCSI3 = cfg.SCSI3
		info.IDE2 = cfg.IDE2
		info.CIUser = cfg.CIUser
		info.CIPassword = cfg.CIPassword
		info.IPConfig0 = cfg.IPConfig0
		info.SSHKeys = cfg.SSHKeys
		info.Nameserver = cfg.Nameserver
		info.Searchdomain = cfg.Searchdomain
		info.CICustom = cfg.CICustom

		if nets := cfg.MergeNets(); nets != nil {
			info.Net0 = nets["net0"]
			info.Net1 = nets["net1"]
			info.Net2 = nets["net2"]
			info.Net3 = nets["net3"]
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

// ── Firewall rules (stub) ───────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateFirewallRule(_ context.Context, _ string, _ FirewallRuleInfo) (int, error) {
	return 0, fmt.Errorf("proxmox: CreateFirewallRule not yet implemented")
}

func (r *realProxmoxAPI) ReadFirewallRule(_ context.Context, _ string, _ int) (*FirewallRuleInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadFirewallRule not yet implemented")
}

func (r *realProxmoxAPI) UpdateFirewallRule(_ context.Context, _ string, _ int, _ FirewallRuleInfo) error {
	return fmt.Errorf("proxmox: UpdateFirewallRule not yet implemented")
}

func (r *realProxmoxAPI) DeleteFirewallRule(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("proxmox: DeleteFirewallRule not yet implemented")
}

// ── Snapshots (stub) ────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateSnapshot(_ context.Context, _ int, _, _ string, _ bool) error {
	return fmt.Errorf("proxmox: CreateSnapshot not yet implemented")
}

func (r *realProxmoxAPI) ReadSnapshot(_ context.Context, _ int, _ string) (*SnapshotInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadSnapshot not yet implemented")
}

func (r *realProxmoxAPI) DeleteSnapshot(_ context.Context, _ int, _ string) error {
	return fmt.Errorf("proxmox: DeleteSnapshot not yet implemented")
}

// ── Pools (stub) ────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreatePool(_ context.Context, _, _ string) error {
	return fmt.Errorf("proxmox: CreatePool not yet implemented")
}

func (r *realProxmoxAPI) ReadPool(_ context.Context, _ string) (*PoolInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadPool not yet implemented")
}

func (r *realProxmoxAPI) UpdatePool(_ context.Context, _, _ string) error {
	return fmt.Errorf("proxmox: UpdatePool not yet implemented")
}

func (r *realProxmoxAPI) DeletePool(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeletePool not yet implemented")
}

// ── Backup jobs (stub) ──────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateBackupJob(_ context.Context, _ BackupJobInfo) (string, error) {
	return "", fmt.Errorf("proxmox: CreateBackupJob not yet implemented")
}

func (r *realProxmoxAPI) ReadBackupJob(_ context.Context, _ string) (*BackupJobInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadBackupJob not yet implemented")
}

func (r *realProxmoxAPI) UpdateBackupJob(_ context.Context, _ string, _ BackupJobInfo) error {
	return fmt.Errorf("proxmox: UpdateBackupJob not yet implemented")
}

func (r *realProxmoxAPI) DeleteBackupJob(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteBackupJob not yet implemented")
}

// ── SDN (stub) ──────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateSDNZone(_ context.Context, _ SDNZoneInfo) error {
	return fmt.Errorf("proxmox: CreateSDNZone not yet implemented")
}
func (r *realProxmoxAPI) ReadSDNZone(_ context.Context, _ string) (*SDNZoneInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadSDNZone not yet implemented")
}
func (r *realProxmoxAPI) UpdateSDNZone(_ context.Context, _ string, _ SDNZoneInfo) error {
	return fmt.Errorf("proxmox: UpdateSDNZone not yet implemented")
}
func (r *realProxmoxAPI) DeleteSDNZone(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteSDNZone not yet implemented")
}
func (r *realProxmoxAPI) CreateSDNVnet(_ context.Context, _ SDNVnetInfo) error {
	return fmt.Errorf("proxmox: CreateSDNVnet not yet implemented")
}
func (r *realProxmoxAPI) ReadSDNVnet(_ context.Context, _ string) (*SDNVnetInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadSDNVnet not yet implemented")
}
func (r *realProxmoxAPI) UpdateSDNVnet(_ context.Context, _ string, _ SDNVnetInfo) error {
	return fmt.Errorf("proxmox: UpdateSDNVnet not yet implemented")
}
func (r *realProxmoxAPI) DeleteSDNVnet(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteSDNVnet not yet implemented")
}
func (r *realProxmoxAPI) CreateSDNSubnet(_ context.Context, _ string, _ SDNSubnetInfo) error {
	return fmt.Errorf("proxmox: CreateSDNSubnet not yet implemented")
}
func (r *realProxmoxAPI) ReadSDNSubnet(_ context.Context, _, _ string) (*SDNSubnetInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadSDNSubnet not yet implemented")
}
func (r *realProxmoxAPI) UpdateSDNSubnet(_ context.Context, _, _ string, _ SDNSubnetInfo) error {
	return fmt.Errorf("proxmox: UpdateSDNSubnet not yet implemented")
}
func (r *realProxmoxAPI) DeleteSDNSubnet(_ context.Context, _, _ string) error {
	return fmt.Errorf("proxmox: DeleteSDNSubnet not yet implemented")
}

// ── HA (stub) ───────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateHAGroup(_ context.Context, _ HAGroupInfo) error {
	return fmt.Errorf("proxmox: CreateHAGroup not yet implemented")
}
func (r *realProxmoxAPI) ReadHAGroup(_ context.Context, _ string) (*HAGroupInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadHAGroup not yet implemented")
}
func (r *realProxmoxAPI) UpdateHAGroup(_ context.Context, _ string, _ HAGroupInfo) error {
	return fmt.Errorf("proxmox: UpdateHAGroup not yet implemented")
}
func (r *realProxmoxAPI) DeleteHAGroup(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteHAGroup not yet implemented")
}
func (r *realProxmoxAPI) CreateHAResource(_ context.Context, _ HAResourceInfo) error {
	return fmt.Errorf("proxmox: CreateHAResource not yet implemented")
}
func (r *realProxmoxAPI) ReadHAResource(_ context.Context, _ string) (*HAResourceInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadHAResource not yet implemented")
}
func (r *realProxmoxAPI) UpdateHAResource(_ context.Context, _ string, _ HAResourceInfo) error {
	return fmt.Errorf("proxmox: UpdateHAResource not yet implemented")
}
func (r *realProxmoxAPI) DeleteHAResource(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteHAResource not yet implemented")
}

// ── ACME (stub) ─────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateACMEAccount(_ context.Context, _ ACMEAccountInfo) error {
	return fmt.Errorf("proxmox: CreateACMEAccount not yet implemented")
}
func (r *realProxmoxAPI) ReadACMEAccount(_ context.Context, _ string) (*ACMEAccountInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadACMEAccount not yet implemented")
}
func (r *realProxmoxAPI) DeleteACMEAccount(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteACMEAccount not yet implemented")
}

// ── Users/Roles/ACLs (stub) ─────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateUser(_ context.Context, _ PVEUserInfo) error {
	return fmt.Errorf("proxmox: CreateUser not yet implemented")
}
func (r *realProxmoxAPI) ReadUser(_ context.Context, _ string) (*PVEUserInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadUser not yet implemented")
}
func (r *realProxmoxAPI) UpdateUser(_ context.Context, _ string, _ PVEUserInfo) error {
	return fmt.Errorf("proxmox: UpdateUser not yet implemented")
}
func (r *realProxmoxAPI) DeleteUser(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteUser not yet implemented")
}
func (r *realProxmoxAPI) CreateRole(_ context.Context, _ PVERoleInfo) error {
	return fmt.Errorf("proxmox: CreateRole not yet implemented")
}
func (r *realProxmoxAPI) ReadRole(_ context.Context, _ string) (*PVERoleInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadRole not yet implemented")
}
func (r *realProxmoxAPI) UpdateRole(_ context.Context, _ string, _ PVERoleInfo) error {
	return fmt.Errorf("proxmox: UpdateRole not yet implemented")
}
func (r *realProxmoxAPI) DeleteRole(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteRole not yet implemented")
}
func (r *realProxmoxAPI) SetACL(_ context.Context, _ PVEACLInfo) error {
	return fmt.Errorf("proxmox: SetACL not yet implemented")
}
func (r *realProxmoxAPI) ReadACL(_ context.Context, _ string) (*PVEACLInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadACL not yet implemented")
}
func (r *realProxmoxAPI) DeleteACL(_ context.Context, _ string) error {
	return fmt.Errorf("proxmox: DeleteACL not yet implemented")
}

// ── Cluster options (stub) ──────────────────────────────────────────────────

func (r *realProxmoxAPI) ReadClusterOptions(_ context.Context) (*ClusterOptionsInfo, error) {
	return nil, fmt.Errorf("proxmox: ReadClusterOptions not yet implemented")
}
func (r *realProxmoxAPI) UpdateClusterOptions(_ context.Context, _ ClusterOptionsInfo) error {
	return fmt.Errorf("proxmox: UpdateClusterOptions not yet implemented")
}
