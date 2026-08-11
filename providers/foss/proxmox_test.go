package foss

import (
	"context"
	"errors"
	"testing"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

// ── mock ──────────────────────────────────────────────────────────────────────

// mockProxmoxAPI is a function-field mock; each field is nil by default (not called).
// Set only the fields your test needs — an unset field panics if called, which
// surfaces unexpected calls clearly.
type mockProxmoxAPI struct {
	nextVMID      func(ctx context.Context) (int, error)
	createVM      func(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error
	readVM        func(ctx context.Context, vmid int) (*VMInfo, error)
	updateVM      func(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error
	startVM       func(ctx context.Context, vmid int) error
	stopVM        func(ctx context.Context, vmid int) error
	deleteVM      func(ctx context.Context, vmid int) error
	createLXC     func(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error
	readLXC       func(ctx context.Context, vmid int) (*LXCInfo, error)
	updateLXC     func(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error
	startLXC      func(ctx context.Context, vmid int) error
	stopLXC       func(ctx context.Context, vmid int) error
	deleteLXC     func(ctx context.Context, vmid int) error
	createStorage func(ctx context.Context, opts []proxmox.ClusterStorageOptions) error
	readStorage   func(ctx context.Context, name string) (*StorageInfo, error)
	updateStorage func(ctx context.Context, name string, opts []proxmox.ClusterStorageOptions) error
	deleteStorage func(ctx context.Context, name string) error
	createNetwork func(ctx context.Context, cfg NetworkConfig) error
	readNetwork   func(ctx context.Context, iface string) (*NetworkConfig, error)
	updateNetwork func(ctx context.Context, iface string, cfg NetworkConfig) error
	deleteNetwork func(ctx context.Context, iface string) error
	reloadNetwork func(ctx context.Context) error

	createFirewallRule func(ctx context.Context, scope string, rule FirewallRuleInfo) (int, error)
	readFirewallRule   func(ctx context.Context, scope string, pos int) (*FirewallRuleInfo, error)
	updateFirewallRule func(ctx context.Context, scope string, pos int, rule FirewallRuleInfo) error
	deleteFirewallRule func(ctx context.Context, scope string, pos int) error

	createSnapshot func(ctx context.Context, vmid int, name, description string, vmstate bool) error
	readSnapshot   func(ctx context.Context, vmid int, name string) (*SnapshotInfo, error)
	deleteSnapshot func(ctx context.Context, vmid int, name string) error

	createPool func(ctx context.Context, poolid, comment string) error
	readPool   func(ctx context.Context, poolid string) (*PoolInfo, error)
	updatePool func(ctx context.Context, poolid, comment, members string) error
	deletePool func(ctx context.Context, poolid string) error

	createBackupJob func(ctx context.Context, job BackupJobInfo) (string, error)
	readBackupJob   func(ctx context.Context, id string) (*BackupJobInfo, error)
	updateBackupJob func(ctx context.Context, id string, job BackupJobInfo) error
	deleteBackupJob func(ctx context.Context, id string) error

	createSDNZone func(ctx context.Context, zone SDNZoneInfo) error
	readSDNZone   func(ctx context.Context, zone string) (*SDNZoneInfo, error)
	updateSDNZone func(ctx context.Context, zone string, info SDNZoneInfo) error
	deleteSDNZone func(ctx context.Context, zone string) error
	createSDNVnet func(ctx context.Context, vnet SDNVnetInfo) error
	readSDNVnet   func(ctx context.Context, vnet string) (*SDNVnetInfo, error)
	updateSDNVnet func(ctx context.Context, vnet string, info SDNVnetInfo) error
	deleteSDNVnet func(ctx context.Context, vnet string) error
	createSDNSubnet func(ctx context.Context, vnet string, subnet SDNSubnetInfo) error
	readSDNSubnet   func(ctx context.Context, vnet, subnet string) (*SDNSubnetInfo, error)
	updateSDNSubnet func(ctx context.Context, vnet, subnet string, info SDNSubnetInfo) error
	deleteSDNSubnet func(ctx context.Context, vnet, subnet string) error
	createHAGroup func(ctx context.Context, group HAGroupInfo) error
	readHAGroup   func(ctx context.Context, group string) (*HAGroupInfo, error)
	updateHAGroup func(ctx context.Context, group string, info HAGroupInfo) error
	deleteHAGroup func(ctx context.Context, group string) error
	createHAResource func(ctx context.Context, res HAResourceInfo) error
	readHAResource   func(ctx context.Context, sid string) (*HAResourceInfo, error)
	updateHAResource func(ctx context.Context, sid string, info HAResourceInfo) error
	deleteHAResource func(ctx context.Context, sid string) error
	createACMEAccount func(ctx context.Context, acct ACMEAccountInfo) error
	readACMEAccount   func(ctx context.Context, name string) (*ACMEAccountInfo, error)
	deleteACMEAccount func(ctx context.Context, name string) error
	createUser func(ctx context.Context, user PVEUserInfo) error
	readUser   func(ctx context.Context, userid string) (*PVEUserInfo, error)
	updateUser func(ctx context.Context, userid string, user PVEUserInfo) error
	deleteUser func(ctx context.Context, userid string) error
	createRole func(ctx context.Context, role PVERoleInfo) error
	readRole   func(ctx context.Context, roleid string) (*PVERoleInfo, error)
	updateRole func(ctx context.Context, roleid string, role PVERoleInfo) error
	deleteRole func(ctx context.Context, roleid string) error
	setACL     func(ctx context.Context, acl PVEACLInfo) error
	readACL    func(ctx context.Context, path string) (*PVEACLInfo, error)
	deleteACL  func(ctx context.Context, path string) error
	readClusterOptions   func(ctx context.Context) (*ClusterOptionsInfo, error)
	updateClusterOptions func(ctx context.Context, opts ClusterOptionsInfo) error
}

func (m *mockProxmoxAPI) NextVMID(ctx context.Context) (int, error) {
	return m.nextVMID(ctx)
}
func (m *mockProxmoxAPI) CreateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error {
	return m.createVM(ctx, vmid, opts)
}
func (m *mockProxmoxAPI) ReadVM(ctx context.Context, vmid int) (*VMInfo, error) {
	return m.readVM(ctx, vmid)
}
func (m *mockProxmoxAPI) UpdateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error {
	return m.updateVM(ctx, vmid, opts)
}
func (m *mockProxmoxAPI) StartVM(ctx context.Context, vmid int) error { return m.startVM(ctx, vmid) }
func (m *mockProxmoxAPI) StopVM(ctx context.Context, vmid int) error  { return m.stopVM(ctx, vmid) }
func (m *mockProxmoxAPI) DeleteVM(ctx context.Context, vmid int) error {
	return m.deleteVM(ctx, vmid)
}
func (m *mockProxmoxAPI) CreateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error {
	return m.createLXC(ctx, vmid, opts)
}
func (m *mockProxmoxAPI) ReadLXC(ctx context.Context, vmid int) (*LXCInfo, error) {
	return m.readLXC(ctx, vmid)
}
func (m *mockProxmoxAPI) UpdateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error {
	return m.updateLXC(ctx, vmid, opts)
}
func (m *mockProxmoxAPI) StartLXC(ctx context.Context, vmid int) error { return m.startLXC(ctx, vmid) }
func (m *mockProxmoxAPI) StopLXC(ctx context.Context, vmid int) error  { return m.stopLXC(ctx, vmid) }
func (m *mockProxmoxAPI) DeleteLXC(ctx context.Context, vmid int) error {
	return m.deleteLXC(ctx, vmid)
}
func (m *mockProxmoxAPI) CreateStorage(ctx context.Context, opts []proxmox.ClusterStorageOptions) error {
	return m.createStorage(ctx, opts)
}
func (m *mockProxmoxAPI) ReadStorage(ctx context.Context, name string) (*StorageInfo, error) {
	return m.readStorage(ctx, name)
}
func (m *mockProxmoxAPI) UpdateStorage(ctx context.Context, name string, opts []proxmox.ClusterStorageOptions) error {
	return m.updateStorage(ctx, name, opts)
}
func (m *mockProxmoxAPI) DeleteStorage(ctx context.Context, name string) error {
	return m.deleteStorage(ctx, name)
}
func (m *mockProxmoxAPI) CreateNetwork(ctx context.Context, cfg NetworkConfig) error {
	return m.createNetwork(ctx, cfg)
}
func (m *mockProxmoxAPI) ReadNetwork(ctx context.Context, iface string) (*NetworkConfig, error) {
	return m.readNetwork(ctx, iface)
}
func (m *mockProxmoxAPI) UpdateNetwork(ctx context.Context, iface string, cfg NetworkConfig) error {
	return m.updateNetwork(ctx, iface, cfg)
}
func (m *mockProxmoxAPI) DeleteNetwork(ctx context.Context, iface string) error {
	return m.deleteNetwork(ctx, iface)
}
func (m *mockProxmoxAPI) ReloadNetwork(ctx context.Context) error { return m.reloadNetwork(ctx) }

func (m *mockProxmoxAPI) CreateFirewallRule(ctx context.Context, scope string, rule FirewallRuleInfo) (int, error) {
	return m.createFirewallRule(ctx, scope, rule)
}
func (m *mockProxmoxAPI) ReadFirewallRule(ctx context.Context, scope string, pos int) (*FirewallRuleInfo, error) {
	return m.readFirewallRule(ctx, scope, pos)
}
func (m *mockProxmoxAPI) UpdateFirewallRule(ctx context.Context, scope string, pos int, rule FirewallRuleInfo) error {
	return m.updateFirewallRule(ctx, scope, pos, rule)
}
func (m *mockProxmoxAPI) DeleteFirewallRule(ctx context.Context, scope string, pos int) error {
	return m.deleteFirewallRule(ctx, scope, pos)
}
func (m *mockProxmoxAPI) CreateSnapshot(ctx context.Context, vmid int, name, description string, vmstate bool) error {
	return m.createSnapshot(ctx, vmid, name, description, vmstate)
}
func (m *mockProxmoxAPI) ReadSnapshot(ctx context.Context, vmid int, name string) (*SnapshotInfo, error) {
	return m.readSnapshot(ctx, vmid, name)
}
func (m *mockProxmoxAPI) DeleteSnapshot(ctx context.Context, vmid int, name string) error {
	return m.deleteSnapshot(ctx, vmid, name)
}
func (m *mockProxmoxAPI) CreatePool(ctx context.Context, poolid, comment string) error {
	return m.createPool(ctx, poolid, comment)
}
func (m *mockProxmoxAPI) ReadPool(ctx context.Context, poolid string) (*PoolInfo, error) {
	return m.readPool(ctx, poolid)
}
func (m *mockProxmoxAPI) UpdatePool(ctx context.Context, poolid, comment, members string) error {
	return m.updatePool(ctx, poolid, comment, members)
}
func (m *mockProxmoxAPI) DeletePool(ctx context.Context, poolid string) error {
	return m.deletePool(ctx, poolid)
}
func (m *mockProxmoxAPI) CreateBackupJob(ctx context.Context, job BackupJobInfo) (string, error) {
	return m.createBackupJob(ctx, job)
}
func (m *mockProxmoxAPI) ReadBackupJob(ctx context.Context, id string) (*BackupJobInfo, error) {
	return m.readBackupJob(ctx, id)
}
func (m *mockProxmoxAPI) UpdateBackupJob(ctx context.Context, id string, job BackupJobInfo) error {
	return m.updateBackupJob(ctx, id, job)
}
func (m *mockProxmoxAPI) DeleteBackupJob(ctx context.Context, id string) error {
	return m.deleteBackupJob(ctx, id)
}

// SDN
func (m *mockProxmoxAPI) CreateSDNZone(ctx context.Context, z SDNZoneInfo) error { return m.createSDNZone(ctx, z) }
func (m *mockProxmoxAPI) ReadSDNZone(ctx context.Context, z string) (*SDNZoneInfo, error) { return m.readSDNZone(ctx, z) }
func (m *mockProxmoxAPI) UpdateSDNZone(ctx context.Context, z string, i SDNZoneInfo) error { return m.updateSDNZone(ctx, z, i) }
func (m *mockProxmoxAPI) DeleteSDNZone(ctx context.Context, z string) error { return m.deleteSDNZone(ctx, z) }
func (m *mockProxmoxAPI) CreateSDNVnet(ctx context.Context, v SDNVnetInfo) error { return m.createSDNVnet(ctx, v) }
func (m *mockProxmoxAPI) ReadSDNVnet(ctx context.Context, v string) (*SDNVnetInfo, error) { return m.readSDNVnet(ctx, v) }
func (m *mockProxmoxAPI) UpdateSDNVnet(ctx context.Context, v string, i SDNVnetInfo) error { return m.updateSDNVnet(ctx, v, i) }
func (m *mockProxmoxAPI) DeleteSDNVnet(ctx context.Context, v string) error { return m.deleteSDNVnet(ctx, v) }
func (m *mockProxmoxAPI) CreateSDNSubnet(ctx context.Context, v string, s SDNSubnetInfo) error { return m.createSDNSubnet(ctx, v, s) }
func (m *mockProxmoxAPI) ReadSDNSubnet(ctx context.Context, v, s string) (*SDNSubnetInfo, error) { return m.readSDNSubnet(ctx, v, s) }
func (m *mockProxmoxAPI) UpdateSDNSubnet(ctx context.Context, v, s string, i SDNSubnetInfo) error { return m.updateSDNSubnet(ctx, v, s, i) }
func (m *mockProxmoxAPI) DeleteSDNSubnet(ctx context.Context, v, s string) error { return m.deleteSDNSubnet(ctx, v, s) }

// HA
func (m *mockProxmoxAPI) CreateHAGroup(ctx context.Context, g HAGroupInfo) error { return m.createHAGroup(ctx, g) }
func (m *mockProxmoxAPI) ReadHAGroup(ctx context.Context, g string) (*HAGroupInfo, error) { return m.readHAGroup(ctx, g) }
func (m *mockProxmoxAPI) UpdateHAGroup(ctx context.Context, g string, i HAGroupInfo) error { return m.updateHAGroup(ctx, g, i) }
func (m *mockProxmoxAPI) DeleteHAGroup(ctx context.Context, g string) error { return m.deleteHAGroup(ctx, g) }
func (m *mockProxmoxAPI) CreateHAResource(ctx context.Context, r HAResourceInfo) error { return m.createHAResource(ctx, r) }
func (m *mockProxmoxAPI) ReadHAResource(ctx context.Context, s string) (*HAResourceInfo, error) { return m.readHAResource(ctx, s) }
func (m *mockProxmoxAPI) UpdateHAResource(ctx context.Context, s string, i HAResourceInfo) error { return m.updateHAResource(ctx, s, i) }
func (m *mockProxmoxAPI) DeleteHAResource(ctx context.Context, s string) error { return m.deleteHAResource(ctx, s) }

// ACME
func (m *mockProxmoxAPI) CreateACMEAccount(ctx context.Context, a ACMEAccountInfo) error { return m.createACMEAccount(ctx, a) }
func (m *mockProxmoxAPI) ReadACMEAccount(ctx context.Context, n string) (*ACMEAccountInfo, error) { return m.readACMEAccount(ctx, n) }
func (m *mockProxmoxAPI) DeleteACMEAccount(ctx context.Context, n string) error { return m.deleteACMEAccount(ctx, n) }

// Users/Roles/ACLs
func (m *mockProxmoxAPI) CreateUser(ctx context.Context, u PVEUserInfo) error { return m.createUser(ctx, u) }
func (m *mockProxmoxAPI) ReadUser(ctx context.Context, u string) (*PVEUserInfo, error) { return m.readUser(ctx, u) }
func (m *mockProxmoxAPI) UpdateUser(ctx context.Context, u string, i PVEUserInfo) error { return m.updateUser(ctx, u, i) }
func (m *mockProxmoxAPI) DeleteUser(ctx context.Context, u string) error { return m.deleteUser(ctx, u) }
func (m *mockProxmoxAPI) CreateRole(ctx context.Context, r PVERoleInfo) error { return m.createRole(ctx, r) }
func (m *mockProxmoxAPI) ReadRole(ctx context.Context, r string) (*PVERoleInfo, error) { return m.readRole(ctx, r) }
func (m *mockProxmoxAPI) UpdateRole(ctx context.Context, r string, i PVERoleInfo) error { return m.updateRole(ctx, r, i) }
func (m *mockProxmoxAPI) DeleteRole(ctx context.Context, r string) error { return m.deleteRole(ctx, r) }
func (m *mockProxmoxAPI) SetACL(ctx context.Context, a PVEACLInfo) error { return m.setACL(ctx, a) }
func (m *mockProxmoxAPI) ReadACL(ctx context.Context, p string) (*PVEACLInfo, error) { return m.readACL(ctx, p) }
func (m *mockProxmoxAPI) DeleteACL(ctx context.Context, p string) error { return m.deleteACL(ctx, p) }

// Cluster
func (m *mockProxmoxAPI) ReadClusterOptions(ctx context.Context) (*ClusterOptionsInfo, error) { return m.readClusterOptions(ctx) }
func (m *mockProxmoxAPI) UpdateClusterOptions(ctx context.Context, o ClusterOptionsInfo) error { return m.updateClusterOptions(ctx, o) }

// newTestProvider returns a ProxmoxProvider with the given mock injected.
func newTestProvider(mock *mockProxmoxAPI) *ProxmoxProvider {
	return &ProxmoxProvider{
		endpoint: "https://pve.test:8006",
		node:     "pve",
		api:      mock,
	}
}

// ── Diff tests ────────────────────────────────────────────────────────────────

func TestProxmoxDiff_NewResource_ReturnsChangeAdd(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()

	types := []core.ResourceType{"proxmox:vm", "proxmox:lxc", "proxmox:storage", "proxmox:network"}
	for _, rt := range types {
		args := core.ResourceArgs{Type: rt, Name: "test", Inputs: core.Inputs{}}
		diff, err := p.Diff(ctx, nil, args)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", rt, err)
		}
		if diff.Kind != core.ChangeAdd {
			t.Errorf("%s: want ChangeAdd, got %v", rt, diff.Kind)
		}
	}
}

func TestProxmoxDiff_NoChange_ReturnsChangeNoOp(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()

	inputs := core.Inputs{"memory": 2048, "cores": 2, "net0": "virtio,bridge=vmbr0"}
	current := &core.ResourceState{Inputs: inputs}
	args := core.ResourceArgs{Type: "proxmox:vm", Name: "web-01", Inputs: inputs}

	diff, err := p.Diff(ctx, current, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.Kind != core.ChangeNoOp {
		t.Errorf("want ChangeNoOp, got %v", diff.Kind)
	}
}

func TestProxmoxDiff_VMMemoryChanged_ReturnsChangeUpdate(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()

	current := &core.ResourceState{Inputs: core.Inputs{"memory": 1024, "cores": 2}}
	desired := core.ResourceArgs{
		Type:   "proxmox:vm",
		Name:   "web-01",
		Inputs: core.Inputs{"memory": 4096, "cores": 2},
	}

	diff, err := p.Diff(ctx, current, desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.Kind != core.ChangeUpdate {
		t.Errorf("want ChangeUpdate, got %v", diff.Kind)
	}
	if len(diff.Changes) == 0 {
		t.Error("expected at least one field change")
	}
	if diff.Changes[0].Field != "memory" {
		t.Errorf("want field %q, got %q", "memory", diff.Changes[0].Field)
	}
}

func TestProxmoxDiff_NetworkAutostart_ReturnsChangeUpdate(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()

	current := &core.ResourceState{Inputs: core.Inputs{"autostart": false, "bridge_ports": "eth0"}}
	desired := core.ResourceArgs{
		Type:   "proxmox:network",
		Name:   "vmbr1",
		Inputs: core.Inputs{"autostart": true, "bridge_ports": "eth0"},
	}

	diff, err := p.Diff(ctx, current, desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.Kind != core.ChangeUpdate {
		t.Errorf("want ChangeUpdate, got %v", diff.Kind)
	}
}

// ── Read tests ────────────────────────────────────────────────────────────────

func TestProxmoxReadVM_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readVM: func(_ context.Context, vmid int) (*VMInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:vm::web-01", "101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing VM, got %+v", state)
	}
}

func TestProxmoxReadVM_Found_MapsFields(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readVM: func(_ context.Context, vmid int) (*VMInfo, error) {
			return &VMInfo{VMID: vmid, Name: "web-01", Status: "running", Memory: 2048, Cores: 2}, nil
		},
	})

	state, err := p.Read(context.Background(), "proxmox:vm::web-01", "101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("want non-nil state")
	}
	if state.Status != core.StatusRunning {
		t.Errorf("want StatusRunning, got %v", state.Status)
	}
	if state.Inputs["name"] != "web-01" {
		t.Errorf("want name %q, got %v", "web-01", state.Inputs["name"])
	}
	if state.Inputs["cores"] != 2 {
		t.Errorf("want cores 2, got %v", state.Inputs["cores"])
	}
}

func TestProxmoxReadVM_Stopped_ReturnsPending(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readVM: func(_ context.Context, _ int) (*VMInfo, error) {
			return &VMInfo{VMID: 101, Name: "web-01", Status: "stopped"}, nil
		},
	})
	state, err := p.Read(context.Background(), "proxmox:vm::web-01", "101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Status != core.StatusPending {
		t.Errorf("want StatusPending for stopped VM, got %v", state.Status)
	}
}

func TestProxmoxReadLXC_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readLXC: func(_ context.Context, _ int) (*LXCInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:lxc::cache-01", "lxc/201")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing LXC, got %+v", state)
	}
}

func TestProxmoxReadStorage_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readStorage: func(_ context.Context, _ string) (*StorageInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:storage::my-nfs", "my-nfs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing storage, got %+v", state)
	}
}

func TestProxmoxReadNetwork_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readNetwork: func(_ context.Context, _ string) (*NetworkConfig, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:network::vmbr1", "pve/vmbr1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing network, got %+v", state)
	}
}

// ── Create tests ──────────────────────────────────────────────────────────────

func TestProxmoxCreateVM_AutoVMID(t *testing.T) {
	nextVMIDCalled := false
	p := newTestProvider(&mockProxmoxAPI{
		nextVMID: func(_ context.Context) (int, error) {
			nextVMIDCalled = true
			return 100, nil
		},
		createVM: func(_ context.Context, vmid int, _ []proxmox.VirtualMachineOption) error {
			if vmid != 100 {
				t.Errorf("want vmid 100, got %d", vmid)
			}
			return nil
		},
	})

	args := core.ResourceArgs{
		Type:   "proxmox:vm",
		Name:   "web-01",
		Inputs: core.Inputs{"memory": 2048, "cores": 2}, // no vmid → auto-assign
	}
	state, err := p.Create(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextVMIDCalled {
		t.Error("NextVMID was not called for auto-assign")
	}
	if state.Outputs["vmid"] != 100 {
		t.Errorf("want vmid 100 in outputs, got %v", state.Outputs["vmid"])
	}
}

func TestProxmoxCreateVM_ExplicitVMID_SkipsNextVMID(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		createVM: func(_ context.Context, vmid int, _ []proxmox.VirtualMachineOption) error {
			if vmid != 150 {
				t.Errorf("want vmid 150, got %d", vmid)
			}
			return nil
		},
		// nextVMID intentionally nil — should not be called
	})

	args := core.ResourceArgs{
		Type:   "proxmox:vm",
		Name:   "web-01",
		Inputs: core.Inputs{"vmid": 150, "memory": 2048},
	}
	if _, err := p.Create(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProxmoxCreateVM_StartAfterCreate(t *testing.T) {
	startCalled := false
	p := newTestProvider(&mockProxmoxAPI{
		createVM: func(_ context.Context, _ int, _ []proxmox.VirtualMachineOption) error { return nil },
		startVM: func(_ context.Context, _ int) error {
			startCalled = true
			return nil
		},
		nextVMID: func(_ context.Context) (int, error) { return 100, nil },
	})

	args := core.ResourceArgs{
		Type:   "proxmox:vm",
		Name:   "web-01",
		Inputs: core.Inputs{"start": true},
	}
	if _, err := p.Create(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !startCalled {
		t.Error("StartVM was not called when start=true")
	}
}

// ── Delete tests ──────────────────────────────────────────────────────────────

func TestProxmoxDeleteVM_AlreadyGone_IsNoOp(t *testing.T) {
	deleteCalled := false
	p := newTestProvider(&mockProxmoxAPI{
		readVM: func(_ context.Context, _ int) (*VMInfo, error) { return nil, nil },
		deleteVM: func(_ context.Context, _ int) error {
			deleteCalled = true
			return nil
		},
	})

	state := &core.ResourceState{Type: "proxmox:vm", ExternalID: "101"}
	if err := p.Delete(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("DeleteVM should not be called when VM is already gone")
	}
}

func TestProxmoxDeleteVM_Running_StopsFirst(t *testing.T) {
	var callOrder []string
	p := newTestProvider(&mockProxmoxAPI{
		readVM: func(_ context.Context, _ int) (*VMInfo, error) {
			return &VMInfo{VMID: 101, Status: "running"}, nil
		},
		stopVM: func(_ context.Context, _ int) error {
			callOrder = append(callOrder, "stop")
			return nil
		},
		deleteVM: func(_ context.Context, _ int) error {
			callOrder = append(callOrder, "delete")
			return nil
		},
	})

	state := &core.ResourceState{Type: "proxmox:vm", ExternalID: "101"}
	if err := p.Delete(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callOrder) != 2 || callOrder[0] != "stop" || callOrder[1] != "delete" {
		t.Errorf("want [stop delete], got %v", callOrder)
	}
}

func TestProxmoxDeleteLXC_Running_StopsFirst(t *testing.T) {
	var callOrder []string
	p := newTestProvider(&mockProxmoxAPI{
		readLXC: func(_ context.Context, _ int) (*LXCInfo, error) {
			return &LXCInfo{VMID: 201, Status: "running"}, nil
		},
		stopLXC: func(_ context.Context, _ int) error {
			callOrder = append(callOrder, "stop")
			return nil
		},
		deleteLXC: func(_ context.Context, _ int) error {
			callOrder = append(callOrder, "delete")
			return nil
		},
	})

	state := &core.ResourceState{Type: "proxmox:lxc", ExternalID: "lxc/201"}
	if err := p.Delete(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callOrder) != 2 || callOrder[0] != "stop" || callOrder[1] != "delete" {
		t.Errorf("want [stop delete], got %v", callOrder)
	}
}

func TestProxmoxDeleteNetwork_ReloadsAfterDelete(t *testing.T) {
	reloadCalled := false
	p := newTestProvider(&mockProxmoxAPI{
		deleteNetwork: func(_ context.Context, _ string) error { return nil },
		reloadNetwork: func(_ context.Context) error {
			reloadCalled = true
			return nil
		},
	})

	state := &core.ResourceState{Type: "proxmox:network", ExternalID: "pve/vmbr1"}
	if err := p.Delete(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reloadCalled {
		t.Error("ReloadNetwork was not called after network delete")
	}
}

// ── Error propagation ─────────────────────────────────────────────────────────

func TestProxmoxCreateVM_APIError_Propagates(t *testing.T) {
	apiErr := errors.New("PVE API: permission denied")
	p := newTestProvider(&mockProxmoxAPI{
		nextVMID: func(_ context.Context) (int, error) { return 100, nil },
		createVM: func(_ context.Context, _ int, _ []proxmox.VirtualMachineOption) error {
			return apiErr
		},
	})

	args := core.ResourceArgs{Type: "proxmox:vm", Name: "web-01", Inputs: core.Inputs{}}
	_, err := p.Create(context.Background(), args)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("want wrapped apiErr, got %v", err)
	}
}
