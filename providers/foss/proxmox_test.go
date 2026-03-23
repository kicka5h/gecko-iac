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
