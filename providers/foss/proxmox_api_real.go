package foss

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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
		if notFoundErr(err) {
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

// ── Raw-REST helpers ────────────────────────────────────────────────────────

// notFoundErr reports whether err means the referenced object does not exist.
// PVE surfaces missing objects as "500 <message>" HTTP statuses whose message
// go-proxmox keeps only in the error text, so proxmox.IsNotFound alone is not
// sufficient.
func notFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if proxmox.IsNotFound(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"does not exist",
		"doesn't exist",
		"not found",
		"no such",
		"no rule at position",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func pveBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

func splitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// postTask issues a POST whose response is a task UPID and waits for it.
func (r *realProxmoxAPI) postTask(ctx context.Context, path string, data interface{}) error {
	var upid proxmox.UPID
	if err := r.client.Post(ctx, path, data, &upid); err != nil {
		return err
	}
	return r.wait(ctx, proxmox.NewTask(upid, r.client))
}

// deleteTask issues a DELETE whose response is a task UPID and waits for it.
func (r *realProxmoxAPI) deleteTask(ctx context.Context, path string) error {
	var upid proxmox.UPID
	if err := r.client.Delete(ctx, path, &upid); err != nil {
		return err
	}
	return r.wait(ctx, proxmox.NewTask(upid, r.client))
}

// guestRef identifies a VM or container in the cluster.
type guestRef struct {
	Node string `json:"node"`
	Type string `json:"type"` // "qemu" or "lxc"
	VMID int    `json:"vmid"`
}

// findGuest locates vmid anywhere in the cluster; returns nil if absent.
func (r *realProxmoxAPI) findGuest(ctx context.Context, vmid int) (*guestRef, error) {
	var guests []guestRef
	if err := r.client.Get(ctx, "/cluster/resources?type=vm", &guests); err != nil {
		return nil, fmt.Errorf("proxmox: list cluster guests: %w", err)
	}
	for i := range guests {
		if guests[i].VMID == vmid {
			return &guests[i], nil
		}
	}
	return nil, nil
}

// guestPath returns the API base path for vmid (e.g. "/nodes/pve/qemu/100"),
// or a nil guestRef if the guest does not exist.
func (r *realProxmoxAPI) guestPath(ctx context.Context, vmid int) (string, *guestRef, error) {
	g, err := r.findGuest(ctx, vmid)
	if err != nil {
		return "", nil, err
	}
	if g == nil {
		return "", nil, nil
	}
	return fmt.Sprintf("/nodes/%s/%s/%d", g.Node, g.Type, vmid), g, nil
}

// ── Firewall rules ──────────────────────────────────────────────────────────

// firewallRulePayload always sends enable so rules can be toggled off.
type firewallRulePayload struct {
	Type    string `json:"type,omitempty"`
	Action  string `json:"action,omitempty"`
	Source  string `json:"source,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Proto   string `json:"proto,omitempty"`
	Dport   string `json:"dport,omitempty"`
	Sport   string `json:"sport,omitempty"`
	Enable  int    `json:"enable"`
	Comment string `json:"comment,omitempty"`
	Log     string `json:"log,omitempty"`
	Macro   string `json:"macro,omitempty"`
	Iface   string `json:"iface,omitempty"`
}

func firewallPayload(rule FirewallRuleInfo) firewallRulePayload {
	return firewallRulePayload{
		Type:    rule.Type,
		Action:  rule.Action,
		Source:  rule.Source,
		Dest:    rule.Dest,
		Proto:   rule.Proto,
		Dport:   rule.DPort,
		Sport:   rule.SPort,
		Enable:  pveBool(rule.Enable),
		Comment: rule.Comment,
		Log:     rule.Log,
		Macro:   rule.Macro,
		Iface:   rule.Iface,
	}
}

// firewallBase maps a provider scope ("cluster", "<node>", "<node>/<vmid>")
// to the API path holding its rules.
func (r *realProxmoxAPI) firewallBase(ctx context.Context, scope string) (string, error) {
	if scope == "cluster" {
		return "/cluster/firewall", nil
	}
	if !strings.Contains(scope, "/") {
		return "/nodes/" + url.PathEscape(scope) + "/firewall", nil
	}
	parts := strings.SplitN(scope, "/", 2)
	vmid, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("proxmox: invalid firewall scope %q: %w", scope, err)
	}
	path, g, err := r.guestPath(ctx, vmid)
	if err != nil {
		return "", err
	}
	if g == nil {
		// Worded so notFoundErr recognizes it on read/delete paths.
		return "", fmt.Errorf("proxmox: firewall scope %q: vmid %d does not exist", scope, vmid)
	}
	return path + "/firewall", nil
}

func (r *realProxmoxAPI) CreateFirewallRule(ctx context.Context, scope string, rule FirewallRuleInfo) (int, error) {
	base, err := r.firewallBase(ctx, scope)
	if err != nil {
		return 0, err
	}
	if err := r.client.Post(ctx, base+"/rules", firewallPayload(rule), nil); err != nil {
		return 0, fmt.Errorf("proxmox: create firewall rule: %w", err)
	}
	// PVE inserts new rules at the top of the list.
	return 0, nil
}

func (r *realProxmoxAPI) ReadFirewallRule(ctx context.Context, scope string, pos int) (*FirewallRuleInfo, error) {
	base, err := r.firewallBase(ctx, scope)
	if err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}
	var raw struct {
		firewallRulePayload
		Pos int `json:"pos"`
	}
	if err := r.client.Get(ctx, fmt.Sprintf("%s/rules/%d", base, pos), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read firewall rule %s/%d: %w", scope, pos, err)
	}
	return &FirewallRuleInfo{
		Pos:     raw.Pos,
		Type:    raw.Type,
		Action:  raw.Action,
		Source:  raw.Source,
		Dest:    raw.Dest,
		Proto:   raw.Proto,
		DPort:   raw.Dport,
		SPort:   raw.Sport,
		Enable:  raw.Enable == 1,
		Comment: raw.Comment,
		Log:     raw.Log,
		Macro:   raw.Macro,
		Iface:   raw.Iface,
	}, nil
}

func (r *realProxmoxAPI) UpdateFirewallRule(ctx context.Context, scope string, pos int, rule FirewallRuleInfo) error {
	base, err := r.firewallBase(ctx, scope)
	if err != nil {
		return err
	}
	if err := r.client.Put(ctx, fmt.Sprintf("%s/rules/%d", base, pos), firewallPayload(rule), nil); err != nil {
		return fmt.Errorf("proxmox: update firewall rule %s/%d: %w", scope, pos, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteFirewallRule(ctx context.Context, scope string, pos int) error {
	base, err := r.firewallBase(ctx, scope)
	if err != nil {
		if notFoundErr(err) {
			return nil
		}
		return err
	}
	if err := r.client.Delete(ctx, fmt.Sprintf("%s/rules/%d", base, pos), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete firewall rule %s/%d: %w", scope, pos, err)
	}
	return nil
}

// ── Snapshots ───────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateSnapshot(ctx context.Context, vmid int, name, description string, vmstate bool) error {
	path, g, err := r.guestPath(ctx, vmid)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("proxmox: snapshot %q: vmid %d does not exist", name, vmid)
	}
	body := map[string]interface{}{"snapname": name}
	if description != "" {
		body["description"] = description
	}
	// Only QEMU snapshots can include RAM state.
	if vmstate && g.Type == "qemu" {
		body["vmstate"] = 1
	}
	if err := r.postTask(ctx, path+"/snapshot", body); err != nil {
		return fmt.Errorf("proxmox: create snapshot %q on %d: %w", name, vmid, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadSnapshot(ctx context.Context, vmid int, name string) (*SnapshotInfo, error) {
	path, g, err := r.guestPath(ctx, vmid)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, nil
	}
	var snaps []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		VMState     int    `json:"vmstate"`
	}
	if err := r.client.Get(ctx, path+"/snapshot", &snaps); err != nil {
		return nil, fmt.Errorf("proxmox: list snapshots for %d: %w", vmid, err)
	}
	for _, s := range snaps {
		if s.Name == name {
			return &SnapshotInfo{
				Name: s.Name,
				// PVE appends a trailing newline to descriptions.
				Description: strings.TrimSpace(s.Description),
				VMState:     s.VMState == 1,
			}, nil
		}
	}
	return nil, nil
}

func (r *realProxmoxAPI) DeleteSnapshot(ctx context.Context, vmid int, name string) error {
	path, g, err := r.guestPath(ctx, vmid)
	if err != nil {
		return err
	}
	if g == nil {
		return nil
	}
	if err := r.deleteTask(ctx, path+"/snapshot/"+url.PathEscape(name)); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete snapshot %q on %d: %w", name, vmid, err)
	}
	return nil
}

// ── Pools ───────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreatePool(ctx context.Context, poolid, comment string) error {
	if err := r.client.NewPool(ctx, poolid, comment); err != nil {
		return fmt.Errorf("proxmox: create pool %q: %w", poolid, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadPool(ctx context.Context, poolid string) (*PoolInfo, error) {
	pool, err := r.client.Pool(ctx, poolid)
	if err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read pool %q: %w", poolid, err)
	}
	return &PoolInfo{PoolID: poolid, Comment: pool.Comment, Members: poolMemberIDs(pool.Members)}, nil
}

// poolMemberIDs extracts guest VMIDs from pool members, sorted ascending.
func poolMemberIDs(members []proxmox.ClusterResource) string {
	var ids []int
	for _, m := range members {
		if m.Type == "qemu" || m.Type == "lxc" {
			ids = append(ids, int(m.VMID))
		}
	}
	sort.Ints(ids)
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.Itoa(id)
	}
	return strings.Join(strs, ",")
}

func (r *realProxmoxAPI) UpdatePool(ctx context.Context, poolid, comment, members string) error {
	// Raw PUT so an empty comment actually clears the field.
	body := map[string]string{"comment": comment}
	if err := r.client.Put(ctx, "/pools/"+url.PathEscape(poolid), body, nil); err != nil {
		return fmt.Errorf("proxmox: update pool %q: %w", poolid, err)
	}
	if members == "" {
		// Empty means unmanaged: leave existing membership alone.
		return nil
	}
	// PVE membership is add/remove, not declarative; reconcile against the
	// current member list.
	pool, err := r.client.Pool(ctx, poolid)
	if err != nil {
		return fmt.Errorf("proxmox: read pool %q for member reconcile: %w", poolid, err)
	}
	currentSet := map[string]bool{}
	for _, id := range splitCommaList(poolMemberIDs(pool.Members)) {
		currentSet[id] = true
	}
	desiredSet := map[string]bool{}
	for _, id := range splitCommaList(members) {
		desiredSet[id] = true
	}
	var toAdd, toRemove []string
	for id := range desiredSet {
		if !currentSet[id] {
			toAdd = append(toAdd, id)
		}
	}
	for id := range currentSet {
		if !desiredSet[id] {
			toRemove = append(toRemove, id)
		}
	}
	sort.Strings(toAdd)
	sort.Strings(toRemove)
	if len(toAdd) > 0 {
		body := map[string]interface{}{"vms": strings.Join(toAdd, ",")}
		if err := r.client.Put(ctx, "/pools/"+url.PathEscape(poolid), body, nil); err != nil {
			return fmt.Errorf("proxmox: add members to pool %q: %w", poolid, err)
		}
	}
	if len(toRemove) > 0 {
		body := map[string]interface{}{"vms": strings.Join(toRemove, ","), "delete": 1}
		if err := r.client.Put(ctx, "/pools/"+url.PathEscape(poolid), body, nil); err != nil {
			return fmt.Errorf("proxmox: remove members from pool %q: %w", poolid, err)
		}
	}
	return nil
}

func (r *realProxmoxAPI) DeletePool(ctx context.Context, poolid string) error {
	if err := r.client.Delete(ctx, "/pools/"+url.PathEscape(poolid), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete pool %q: %w", poolid, err)
	}
	return nil
}

// ── Backup jobs ─────────────────────────────────────────────────────────────

// backupJobPayload always sends enabled so jobs can be toggled off.
type backupJobPayload struct {
	VMID         string `json:"vmid,omitempty"`
	Storage      string `json:"storage,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Compress     string `json:"compress,omitempty"`
	Mailto       string `json:"mailto,omitempty"`
	Schedule     string `json:"schedule,omitempty"`
	Enabled      int    `json:"enabled"`
	PruneBackups string `json:"prune-backups,omitempty"`
}

func backupPayload(job BackupJobInfo) backupJobPayload {
	return backupJobPayload{
		VMID:         job.VMID,
		Storage:      job.Storage,
		Mode:         job.Mode,
		Compress:     job.Compress,
		Mailto:       job.Mailto,
		Schedule:     job.Schedule,
		Enabled:      pveBool(job.Enabled),
		PruneBackups: job.PruneBackups,
	}
}

type backupJobRecord struct {
	ID           string            `json:"id"`
	VMID         string            `json:"vmid"`
	Storage      string            `json:"storage"`
	Mode         string            `json:"mode"`
	Compress     string            `json:"compress"`
	Mailto       string            `json:"mailto"`
	Schedule     string            `json:"schedule"`
	Enabled      proxmox.IntOrBool `json:"enabled"`
	PruneBackups string            `json:"prune-backups"`
}

func (rec *backupJobRecord) toInfo() *BackupJobInfo {
	return &BackupJobInfo{
		ID:           rec.ID,
		VMID:         rec.VMID,
		Storage:      rec.Storage,
		Mode:         rec.Mode,
		Compress:     rec.Compress,
		Mailto:       rec.Mailto,
		Schedule:     rec.Schedule,
		Enabled:      bool(rec.Enabled),
		PruneBackups: rec.PruneBackups,
	}
}

func (r *realProxmoxAPI) listBackupJobs(ctx context.Context) ([]backupJobRecord, error) {
	var jobs []backupJobRecord
	if err := r.client.Get(ctx, "/cluster/backup", &jobs); err != nil {
		return nil, fmt.Errorf("proxmox: list backup jobs: %w", err)
	}
	return jobs, nil
}

func (r *realProxmoxAPI) CreateBackupJob(ctx context.Context, job BackupJobInfo) (string, error) {
	// PVE autogenerates the job ID and the create call does not return it,
	// so diff the job list around the create.
	before, err := r.listBackupJobs(ctx)
	if err != nil {
		return "", err
	}
	seen := make(map[string]bool, len(before))
	for _, j := range before {
		seen[j.ID] = true
	}
	if err := r.client.Post(ctx, "/cluster/backup", backupPayload(job), nil); err != nil {
		return "", fmt.Errorf("proxmox: create backup job: %w", err)
	}
	after, err := r.listBackupJobs(ctx)
	if err != nil {
		return "", err
	}
	for _, j := range after {
		if !seen[j.ID] {
			return j.ID, nil
		}
	}
	return "", fmt.Errorf("proxmox: backup job created but its ID could not be determined")
}

func (r *realProxmoxAPI) ReadBackupJob(ctx context.Context, id string) (*BackupJobInfo, error) {
	var rec backupJobRecord
	if err := r.client.Get(ctx, "/cluster/backup/"+url.PathEscape(id), &rec); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read backup job %q: %w", id, err)
	}
	if rec.ID == "" {
		rec.ID = id
	}
	return rec.toInfo(), nil
}

func (r *realProxmoxAPI) UpdateBackupJob(ctx context.Context, id string, job BackupJobInfo) error {
	if err := r.client.Put(ctx, "/cluster/backup/"+url.PathEscape(id), backupPayload(job), nil); err != nil {
		return fmt.Errorf("proxmox: update backup job %q: %w", id, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteBackupJob(ctx context.Context, id string) error {
	if err := r.client.Delete(ctx, "/cluster/backup/"+url.PathEscape(id), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete backup job %q: %w", id, err)
	}
	return nil
}

// ── SDN ─────────────────────────────────────────────────────────────────────

// applySDN commits pending SDN configuration changes cluster-wide.
func (r *realProxmoxAPI) applySDN(ctx context.Context) error {
	var upid proxmox.UPID
	if err := r.client.Put(ctx, "/cluster/sdn", nil, &upid); err != nil {
		return fmt.Errorf("proxmox: apply sdn config: %w", err)
	}
	return r.wait(ctx, proxmox.NewTask(upid, r.client))
}

type sdnZonePayload struct {
	Zone       string `json:"zone,omitempty"`
	Type       string `json:"type,omitempty"`
	Bridge     string `json:"bridge,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
	Nodes      string `json:"nodes,omitempty"`
	DNS        string `json:"dns,omitempty"`
	ReverseDNS string `json:"reversedns,omitempty"`
	DNSZone    string `json:"dnszone,omitempty"`
	IPAM       string `json:"ipam,omitempty"`
}

func sdnZonePayloadFrom(zone SDNZoneInfo) sdnZonePayload {
	return sdnZonePayload{
		Zone:       zone.Zone,
		Type:       zone.Type,
		Bridge:     zone.Bridge,
		MTU:        zone.MTU,
		Nodes:      zone.Nodes,
		DNS:        zone.DNS,
		ReverseDNS: zone.ReverseDNS,
		DNSZone:    zone.DNSZone,
		IPAM:       zone.IPAM,
	}
}

func (r *realProxmoxAPI) CreateSDNZone(ctx context.Context, zone SDNZoneInfo) error {
	if err := r.client.Post(ctx, "/cluster/sdn/zones", sdnZonePayloadFrom(zone), nil); err != nil {
		return fmt.Errorf("proxmox: create sdn zone %q: %w", zone.Zone, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) ReadSDNZone(ctx context.Context, zone string) (*SDNZoneInfo, error) {
	var raw sdnZonePayload
	if err := r.client.Get(ctx, "/cluster/sdn/zones/"+url.PathEscape(zone), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read sdn zone %q: %w", zone, err)
	}
	if raw.Zone == "" {
		raw.Zone = zone
	}
	return &SDNZoneInfo{
		Zone:       raw.Zone,
		Type:       raw.Type,
		Bridge:     raw.Bridge,
		MTU:        raw.MTU,
		Nodes:      raw.Nodes,
		DNS:        raw.DNS,
		ReverseDNS: raw.ReverseDNS,
		DNSZone:    raw.DNSZone,
		IPAM:       raw.IPAM,
	}, nil
}

func (r *realProxmoxAPI) UpdateSDNZone(ctx context.Context, zone string, info SDNZoneInfo) error {
	payload := sdnZonePayloadFrom(info)
	// The zone id lives in the path and the type is immutable; PVE rejects
	// both as body parameters on update.
	payload.Zone = ""
	payload.Type = ""
	if err := r.client.Put(ctx, "/cluster/sdn/zones/"+url.PathEscape(zone), payload, nil); err != nil {
		return fmt.Errorf("proxmox: update sdn zone %q: %w", zone, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) DeleteSDNZone(ctx context.Context, zone string) error {
	if err := r.client.Delete(ctx, "/cluster/sdn/zones/"+url.PathEscape(zone), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete sdn zone %q: %w", zone, err)
	}
	return r.applySDN(ctx)
}

type sdnVnetPayload struct {
	Vnet      string `json:"vnet,omitempty"`
	Zone      string `json:"zone,omitempty"`
	Tag       int    `json:"tag,omitempty"`
	Alias     string `json:"alias,omitempty"`
	VlanAware int    `json:"vlanaware,omitempty"`
}

func (r *realProxmoxAPI) CreateSDNVnet(ctx context.Context, vnet SDNVnetInfo) error {
	payload := sdnVnetPayload{
		Vnet:      vnet.Vnet,
		Zone:      vnet.Zone,
		Tag:       vnet.Tag,
		Alias:     vnet.Alias,
		VlanAware: pveBool(vnet.VlanAware),
	}
	if err := r.client.Post(ctx, "/cluster/sdn/vnets", payload, nil); err != nil {
		return fmt.Errorf("proxmox: create sdn vnet %q: %w", vnet.Vnet, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) ReadSDNVnet(ctx context.Context, vnet string) (*SDNVnetInfo, error) {
	var raw struct {
		Vnet      string            `json:"vnet"`
		Zone      string            `json:"zone"`
		Tag       int               `json:"tag"`
		Alias     string            `json:"alias"`
		VlanAware proxmox.IntOrBool `json:"vlanaware"`
	}
	if err := r.client.Get(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read sdn vnet %q: %w", vnet, err)
	}
	if raw.Vnet == "" {
		raw.Vnet = vnet
	}
	return &SDNVnetInfo{
		Vnet:      raw.Vnet,
		Zone:      raw.Zone,
		Tag:       raw.Tag,
		Alias:     raw.Alias,
		VlanAware: bool(raw.VlanAware),
	}, nil
}

func (r *realProxmoxAPI) UpdateSDNVnet(ctx context.Context, vnet string, info SDNVnetInfo) error {
	payload := sdnVnetPayload{
		Zone:      info.Zone,
		Tag:       info.Tag,
		Alias:     info.Alias,
		VlanAware: pveBool(info.VlanAware),
	}
	if err := r.client.Put(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet), payload, nil); err != nil {
		return fmt.Errorf("proxmox: update sdn vnet %q: %w", vnet, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) DeleteSDNVnet(ctx context.Context, vnet string) error {
	if err := r.client.Delete(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete sdn vnet %q: %w", vnet, err)
	}
	return r.applySDN(ctx)
}

// sdnSubnetRecord is a subnet as listed by PVE; Subnet holds the canonical
// id (e.g. "zone-10.0.0.0-24") while CIDR holds the human form.
type sdnSubnetRecord struct {
	Subnet        string            `json:"subnet"`
	CIDR          string            `json:"cidr"`
	Vnet          string            `json:"vnet"`
	Gateway       string            `json:"gateway"`
	SNAT          proxmox.IntOrBool `json:"snat"`
	DNSZonePrefix string            `json:"dnszoneprefix"`
}

// findSDNSubnet resolves a subnet by CIDR or canonical id; nil if absent.
func (r *realProxmoxAPI) findSDNSubnet(ctx context.Context, vnet, subnet string) (*sdnSubnetRecord, error) {
	var subs []sdnSubnetRecord
	if err := r.client.Get(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets", &subs); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: list subnets of vnet %q: %w", vnet, err)
	}
	for i := range subs {
		if subs[i].CIDR == subnet || subs[i].Subnet == subnet {
			return &subs[i], nil
		}
	}
	return nil, nil
}

func (r *realProxmoxAPI) CreateSDNSubnet(ctx context.Context, vnet string, subnet SDNSubnetInfo) error {
	body := map[string]interface{}{
		"subnet": subnet.Subnet,
		"type":   "subnet",
		"snat":   pveBool(subnet.SNAT),
	}
	if subnet.Gateway != "" {
		body["gateway"] = subnet.Gateway
	}
	if subnet.DNSZonePrefix != "" {
		body["dnszoneprefix"] = subnet.DNSZonePrefix
	}
	if err := r.client.Post(ctx, "/cluster/sdn/vnets/"+url.PathEscape(vnet)+"/subnets", body, nil); err != nil {
		return fmt.Errorf("proxmox: create sdn subnet %q in vnet %q: %w", subnet.Subnet, vnet, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) ReadSDNSubnet(ctx context.Context, vnet, subnet string) (*SDNSubnetInfo, error) {
	rec, err := r.findSDNSubnet(ctx, vnet, subnet)
	if err != nil || rec == nil {
		return nil, err
	}
	cidr := rec.CIDR
	if cidr == "" {
		cidr = rec.Subnet
	}
	info := &SDNSubnetInfo{
		Subnet:        cidr,
		Vnet:          vnet,
		Gateway:       rec.Gateway,
		SNAT:          bool(rec.SNAT),
		DNSZonePrefix: rec.DNSZonePrefix,
	}
	if rec.Vnet != "" {
		info.Vnet = rec.Vnet
	}
	return info, nil
}

func (r *realProxmoxAPI) UpdateSDNSubnet(ctx context.Context, vnet, subnet string, info SDNSubnetInfo) error {
	rec, err := r.findSDNSubnet(ctx, vnet, subnet)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("proxmox: sdn subnet %q in vnet %q does not exist", subnet, vnet)
	}
	body := map[string]interface{}{"snat": pveBool(info.SNAT)}
	if info.Gateway != "" {
		body["gateway"] = info.Gateway
	}
	if info.DNSZonePrefix != "" {
		body["dnszoneprefix"] = info.DNSZonePrefix
	}
	path := "/cluster/sdn/vnets/" + url.PathEscape(vnet) + "/subnets/" + url.PathEscape(rec.Subnet)
	if err := r.client.Put(ctx, path, body, nil); err != nil {
		return fmt.Errorf("proxmox: update sdn subnet %q in vnet %q: %w", subnet, vnet, err)
	}
	return r.applySDN(ctx)
}

func (r *realProxmoxAPI) DeleteSDNSubnet(ctx context.Context, vnet, subnet string) error {
	rec, err := r.findSDNSubnet(ctx, vnet, subnet)
	if err != nil {
		return err
	}
	if rec == nil {
		return nil
	}
	path := "/cluster/sdn/vnets/" + url.PathEscape(vnet) + "/subnets/" + url.PathEscape(rec.Subnet)
	if err := r.client.Delete(ctx, path, nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete sdn subnet %q in vnet %q: %w", subnet, vnet, err)
	}
	return r.applySDN(ctx)
}

// ── HA ──────────────────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateHAGroup(ctx context.Context, group HAGroupInfo) error {
	body := map[string]interface{}{
		"group":      group.Group,
		"type":       "group",
		"nodes":      group.Nodes,
		"restricted": pveBool(group.Restricted),
		"nofailback": pveBool(group.NoFailback),
	}
	if group.Comment != "" {
		body["comment"] = group.Comment
	}
	if err := r.client.Post(ctx, "/cluster/ha/groups", body, nil); err != nil {
		return fmt.Errorf("proxmox: create ha group %q: %w", group.Group, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadHAGroup(ctx context.Context, group string) (*HAGroupInfo, error) {
	var raw struct {
		Group      string            `json:"group"`
		Nodes      string            `json:"nodes"`
		Restricted proxmox.IntOrBool `json:"restricted"`
		NoFailback proxmox.IntOrBool `json:"nofailback"`
		Comment    string            `json:"comment"`
	}
	if err := r.client.Get(ctx, "/cluster/ha/groups/"+url.PathEscape(group), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read ha group %q: %w", group, err)
	}
	if raw.Group == "" {
		raw.Group = group
	}
	return &HAGroupInfo{
		Group:      raw.Group,
		Nodes:      raw.Nodes,
		Restricted: bool(raw.Restricted),
		NoFailback: bool(raw.NoFailback),
		Comment:    raw.Comment,
	}, nil
}

func (r *realProxmoxAPI) UpdateHAGroup(ctx context.Context, group string, info HAGroupInfo) error {
	body := map[string]interface{}{
		"nodes":      info.Nodes,
		"restricted": pveBool(info.Restricted),
		"nofailback": pveBool(info.NoFailback),
	}
	if info.Comment != "" {
		body["comment"] = info.Comment
	}
	if err := r.client.Put(ctx, "/cluster/ha/groups/"+url.PathEscape(group), body, nil); err != nil {
		return fmt.Errorf("proxmox: update ha group %q: %w", group, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteHAGroup(ctx context.Context, group string) error {
	if err := r.client.Delete(ctx, "/cluster/ha/groups/"+url.PathEscape(group), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete ha group %q: %w", group, err)
	}
	return nil
}

func haResourceBody(info HAResourceInfo) map[string]interface{} {
	body := map[string]interface{}{}
	if info.Group != "" {
		body["group"] = info.Group
	}
	if info.MaxRelocate != 0 {
		body["max_relocate"] = info.MaxRelocate
	}
	if info.MaxRestart != 0 {
		body["max_restart"] = info.MaxRestart
	}
	if info.State != "" {
		body["state"] = info.State
	}
	if info.Comment != "" {
		body["comment"] = info.Comment
	}
	return body
}

func (r *realProxmoxAPI) CreateHAResource(ctx context.Context, res HAResourceInfo) error {
	body := haResourceBody(res)
	body["sid"] = res.SID
	if err := r.client.Post(ctx, "/cluster/ha/resources", body, nil); err != nil {
		return fmt.Errorf("proxmox: create ha resource %q: %w", res.SID, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadHAResource(ctx context.Context, sid string) (*HAResourceInfo, error) {
	var raw struct {
		SID         string `json:"sid"`
		Group       string `json:"group"`
		MaxRelocate int    `json:"max_relocate"`
		MaxRestart  int    `json:"max_restart"`
		State       string `json:"state"`
		Comment     string `json:"comment"`
	}
	if err := r.client.Get(ctx, "/cluster/ha/resources/"+url.PathEscape(sid), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read ha resource %q: %w", sid, err)
	}
	if raw.SID == "" {
		raw.SID = sid
	}
	return &HAResourceInfo{
		SID:         raw.SID,
		Group:       raw.Group,
		MaxRelocate: raw.MaxRelocate,
		MaxRestart:  raw.MaxRestart,
		State:       raw.State,
		Comment:     raw.Comment,
	}, nil
}

func (r *realProxmoxAPI) UpdateHAResource(ctx context.Context, sid string, info HAResourceInfo) error {
	if err := r.client.Put(ctx, "/cluster/ha/resources/"+url.PathEscape(sid), haResourceBody(info), nil); err != nil {
		return fmt.Errorf("proxmox: update ha resource %q: %w", sid, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteHAResource(ctx context.Context, sid string) error {
	if err := r.client.Delete(ctx, "/cluster/ha/resources/"+url.PathEscape(sid), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete ha resource %q: %w", sid, err)
	}
	return nil
}

// ── ACME ────────────────────────────────────────────────────────────────────

const defaultACMEDirectory = "https://acme-v02.api.letsencrypt.org/directory"

func (r *realProxmoxAPI) CreateACMEAccount(ctx context.Context, acct ACMEAccountInfo) error {
	directory := acct.Directory
	if directory == "" {
		directory = defaultACMEDirectory
	}
	// Registration requires agreeing to the directory's current TOS.
	var tos string
	if err := r.client.Get(ctx, "/cluster/acme/tos?directory="+url.QueryEscape(directory), &tos); err != nil {
		return fmt.Errorf("proxmox: fetch acme tos for %q: %w", directory, err)
	}
	body := map[string]interface{}{
		"name":      acct.Name,
		"contact":   acct.Contact,
		"directory": directory,
	}
	if tos != "" {
		body["tos_url"] = tos
	}
	if err := r.postTask(ctx, "/cluster/acme/account", body); err != nil {
		return fmt.Errorf("proxmox: create acme account %q: %w", acct.Name, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadACMEAccount(ctx context.Context, name string) (*ACMEAccountInfo, error) {
	var raw struct {
		Account struct {
			Contact []string `json:"contact"`
		} `json:"account"`
		Directory string `json:"directory"`
	}
	if err := r.client.Get(ctx, "/cluster/acme/account/"+url.PathEscape(name), &raw); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read acme account %q: %w", name, err)
	}
	contacts := make([]string, 0, len(raw.Account.Contact))
	for _, c := range raw.Account.Contact {
		contacts = append(contacts, strings.TrimPrefix(c, "mailto:"))
	}
	return &ACMEAccountInfo{
		Name:      name,
		Contact:   strings.Join(contacts, ","),
		Directory: raw.Directory,
	}, nil
}

func (r *realProxmoxAPI) DeleteACMEAccount(ctx context.Context, name string) error {
	if err := r.deleteTask(ctx, "/cluster/acme/account/"+url.PathEscape(name)); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete acme account %q: %w", name, err)
	}
	return nil
}

// ── Users/Roles/ACLs ────────────────────────────────────────────────────────

func (r *realProxmoxAPI) CreateUser(ctx context.Context, user PVEUserInfo) error {
	nu := &proxmox.NewUser{
		UserID:    user.UserID,
		Password:  user.Password,
		Comment:   user.Comment,
		Email:     user.Email,
		Enable:    user.Enable,
		Expire:    user.Expire,
		Firstname: user.FirstName,
		Lastname:  user.LastName,
		Groups:    splitCommaList(user.Groups),
	}
	if err := r.client.NewUser(ctx, nu); err != nil {
		return fmt.Errorf("proxmox: create user %q: %w", user.UserID, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadUser(ctx context.Context, userid string) (*PVEUserInfo, error) {
	u, err := r.client.User(ctx, userid)
	if err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read user %q: %w", userid, err)
	}
	info := &PVEUserInfo{
		UserID:    userid,
		Email:     u.Email,
		FirstName: u.Firstname,
		LastName:  u.Lastname,
		Groups:    strings.Join(u.Groups, ","),
		Enable:    bool(u.Enable),
		Expire:    u.Expire,
		Comment:   u.Comment,
	}
	if u.UserID != "" {
		info.UserID = u.UserID
	}
	return info, nil
}

func (r *realProxmoxAPI) UpdateUser(ctx context.Context, userid string, user PVEUserInfo) error {
	// Raw PUT with enable always present so users can be disabled.
	body := map[string]interface{}{"enable": pveBool(user.Enable)}
	set := func(k, v string) {
		if v != "" {
			body[k] = v
		}
	}
	set("email", user.Email)
	set("firstname", user.FirstName)
	set("lastname", user.LastName)
	set("groups", user.Groups)
	set("comment", user.Comment)
	if user.Expire != 0 {
		body["expire"] = user.Expire
	}
	if err := r.client.Put(ctx, "/access/users/"+url.PathEscape(userid), body, nil); err != nil {
		return fmt.Errorf("proxmox: update user %q: %w", userid, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteUser(ctx context.Context, userid string) error {
	if err := r.client.Delete(ctx, "/access/users/"+url.PathEscape(userid), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete user %q: %w", userid, err)
	}
	return nil
}

func (r *realProxmoxAPI) CreateRole(ctx context.Context, role PVERoleInfo) error {
	if err := r.client.NewRole(ctx, role.RoleID, role.Privs); err != nil {
		return fmt.Errorf("proxmox: create role %q: %w", role.RoleID, err)
	}
	return nil
}

func (r *realProxmoxAPI) ReadRole(ctx context.Context, roleid string) (*PVERoleInfo, error) {
	// PVE returns a role as a map of privilege names to 1.
	var privMap map[string]interface{}
	if err := r.client.Get(ctx, "/access/roles/"+url.PathEscape(roleid), &privMap); err != nil {
		if notFoundErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("proxmox: read role %q: %w", roleid, err)
	}
	privs := make([]string, 0, len(privMap))
	for p := range privMap {
		privs = append(privs, p)
	}
	sort.Strings(privs)
	return &PVERoleInfo{RoleID: roleid, Privs: strings.Join(privs, ",")}, nil
}

func (r *realProxmoxAPI) UpdateRole(ctx context.Context, roleid string, role PVERoleInfo) error {
	body := map[string]interface{}{"privs": role.Privs}
	if err := r.client.Put(ctx, "/access/roles/"+url.PathEscape(roleid), body, nil); err != nil {
		return fmt.Errorf("proxmox: update role %q: %w", roleid, err)
	}
	return nil
}

func (r *realProxmoxAPI) DeleteRole(ctx context.Context, roleid string) error {
	if err := r.client.Delete(ctx, "/access/roles/"+url.PathEscape(roleid), nil); err != nil {
		if notFoundErr(err) {
			return nil
		}
		return fmt.Errorf("proxmox: delete role %q: %w", roleid, err)
	}
	return nil
}

func (r *realProxmoxAPI) SetACL(ctx context.Context, acl PVEACLInfo) error {
	body := map[string]interface{}{
		"path":      acl.Path,
		"roles":     acl.Roles,
		"propagate": pveBool(acl.Propagate),
	}
	if acl.Users != "" {
		body["users"] = acl.Users
	}
	if acl.Groups != "" {
		body["groups"] = acl.Groups
	}
	if err := r.client.Put(ctx, "/access/acl", body, nil); err != nil {
		return fmt.Errorf("proxmox: set acl on %q: %w", acl.Path, err)
	}
	return nil
}

// aclEntriesAt returns all ACL entries on exactly the given path.
func (r *realProxmoxAPI) aclEntriesAt(ctx context.Context, path string) (proxmox.ACLs, error) {
	acls, err := r.client.ACL(ctx)
	if err != nil {
		return nil, fmt.Errorf("proxmox: list acls: %w", err)
	}
	var out proxmox.ACLs
	for _, a := range acls {
		if a.Path == path {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *realProxmoxAPI) ReadACL(ctx context.Context, path string) (*PVEACLInfo, error) {
	entries, err := r.aclEntriesAt(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	userSet := map[string]bool{}
	groupSet := map[string]bool{}
	roleSet := map[string]bool{}
	propagate := false
	for _, a := range entries {
		roleSet[a.RoleID] = true
		switch a.Type {
		case "user", "token":
			userSet[a.UGID] = true
		case "group":
			groupSet[a.UGID] = true
		}
		if bool(a.Propagate) {
			propagate = true
		}
	}
	joinSorted := func(set map[string]bool) string {
		keys := make([]string, 0, len(set))
		for k := range set {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return strings.Join(keys, ",")
	}
	return &PVEACLInfo{
		Path:      path,
		Roles:     joinSorted(roleSet),
		Users:     joinSorted(userSet),
		Groups:    joinSorted(groupSet),
		Propagate: propagate,
	}, nil
}

func (r *realProxmoxAPI) DeleteACL(ctx context.Context, path string) error {
	entries, err := r.aclEntriesAt(ctx, path)
	if err != nil {
		return err
	}
	// Remove entries role by role; PVE deletes matching (path, role, subject)
	// tuples in one call per role.
	byRole := map[string]*struct{ users, groups []string }{}
	for _, a := range entries {
		agg := byRole[a.RoleID]
		if agg == nil {
			agg = &struct{ users, groups []string }{}
			byRole[a.RoleID] = agg
		}
		switch a.Type {
		case "user", "token":
			agg.users = append(agg.users, a.UGID)
		case "group":
			agg.groups = append(agg.groups, a.UGID)
		}
	}
	for role, agg := range byRole {
		body := map[string]interface{}{
			"path":   path,
			"roles":  role,
			"delete": 1,
		}
		if len(agg.users) > 0 {
			body["users"] = strings.Join(agg.users, ",")
		}
		if len(agg.groups) > 0 {
			body["groups"] = strings.Join(agg.groups, ",")
		}
		if err := r.client.Put(ctx, "/access/acl", body, nil); err != nil {
			return fmt.Errorf("proxmox: delete acl on %q (role %s): %w", path, role, err)
		}
	}
	return nil
}

// ── Cluster options ─────────────────────────────────────────────────────────

func (r *realProxmoxAPI) ReadClusterOptions(ctx context.Context) (*ClusterOptionsInfo, error) {
	var raw struct {
		Keyboard  string `json:"keyboard"`
		Language  string `json:"language"`
		EmailFrom string `json:"email_from"`
		Migration string `json:"migration"`
	}
	if err := r.client.Get(ctx, "/cluster/options", &raw); err != nil {
		return nil, fmt.Errorf("proxmox: read cluster options: %w", err)
	}
	info := &ClusterOptionsInfo{
		Keyboard:  raw.Keyboard,
		Language:  raw.Language,
		EmailFrom: raw.EmailFrom,
	}
	// migration is a property string, e.g. "type=secure,network=10.0.0.0/24".
	for _, part := range strings.Split(raw.Migration, ",") {
		switch {
		case strings.HasPrefix(part, "type="):
			info.MigrationType = strings.TrimPrefix(part, "type=")
		case strings.HasPrefix(part, "network="):
			info.MigrationNetwork = strings.TrimPrefix(part, "network=")
		case part == "secure" || part == "insecure":
			info.MigrationType = part
		}
	}
	return info, nil
}

func (r *realProxmoxAPI) UpdateClusterOptions(ctx context.Context, opts ClusterOptionsInfo) error {
	body := map[string]interface{}{}
	set := func(k, v string) {
		if v != "" {
			body[k] = v
		}
	}
	set("keyboard", opts.Keyboard)
	set("language", opts.Language)
	set("email_from", opts.EmailFrom)
	if opts.MigrationType != "" || opts.MigrationNetwork != "" {
		typ := opts.MigrationType
		if typ == "" {
			typ = "secure"
		}
		migration := "type=" + typ
		if opts.MigrationNetwork != "" {
			migration += ",network=" + opts.MigrationNetwork
		}
		body["migration"] = migration
	}
	if len(body) == 0 {
		return nil
	}
	if err := r.client.Put(ctx, "/cluster/options", body, nil); err != nil {
		return fmt.Errorf("proxmox: update cluster options: %w", err)
	}
	return nil
}
