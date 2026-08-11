package foss

import (
	"context"
	"errors"
	"strings"
	"testing"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

// ── notFoundErr ───────────────────────────────────────────────────────────────

func TestProxmoxNotFoundErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"library sentinel", proxmox.ErrNotFound, true},
		{"pve vm missing", errors.New("500 Configuration file 'nodes/pve/qemu-server/100.conf' does not exist"), true},
		{"pve doesn't exist", errors.New("500 zone 'z1' doesn't exist"), true},
		{"pve no such", errors.New("500 no such user ('nobody@pam')"), true},
		{"pve job not found", errors.New("500 vzdump job 'backup-x' not found"), true},
		{"firewall pos", errors.New("400 bad request: no rule at position 3"), true},
		{"unrelated", errors.New("connection refused"), false},
		{"auth", errors.New("not authorized to access endpoint"), false},
	}
	for _, tc := range cases {
		if got := notFoundErr(tc.err); got != tc.want {
			t.Errorf("%s: notFoundErr(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

// ── Firewall rules ────────────────────────────────────────────────────────────

func TestProxmoxCreateFirewallRule_ClusterScope(t *testing.T) {
	var gotScope string
	p := newTestProvider(&mockProxmoxAPI{
		createFirewallRule: func(_ context.Context, scope string, rule FirewallRuleInfo) (int, error) {
			gotScope = scope
			return 0, nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:firewall_rule", Name: "allow-ssh",
		Inputs: core.Inputs{"type": "in", "action": "ACCEPT", "proto": "tcp", "dport": "22", "enable": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotScope != "cluster" {
		t.Errorf("want scope %q, got %q", "cluster", gotScope)
	}
	if state.ExternalID != "cluster/0" {
		t.Errorf("want externalID %q, got %q", "cluster/0", state.ExternalID)
	}
}

func TestProxmoxCreateFirewallRule_VMScope_UsesProviderNode(t *testing.T) {
	var gotScope string
	p := newTestProvider(&mockProxmoxAPI{
		createFirewallRule: func(_ context.Context, scope string, _ FirewallRuleInfo) (int, error) {
			gotScope = scope
			return 0, nil
		},
	})
	_, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:firewall_rule", Name: "vm-rule",
		Inputs: core.Inputs{"vmid": 100, "type": "in", "action": "DROP"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotScope != "pve/100" {
		t.Errorf("want scope %q, got %q", "pve/100", gotScope)
	}
}

func TestProxmoxCreateFirewallRule_NodeScope(t *testing.T) {
	var gotScope string
	p := newTestProvider(&mockProxmoxAPI{
		createFirewallRule: func(_ context.Context, scope string, _ FirewallRuleInfo) (int, error) {
			gotScope = scope
			return 0, nil
		},
	})
	_, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:firewall_rule", Name: "node-rule",
		Inputs: core.Inputs{"node": "pve2", "type": "in", "action": "ACCEPT"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotScope != "pve2" {
		t.Errorf("want scope %q, got %q", "pve2", gotScope)
	}
}

func TestProxmoxReadFirewallRule_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readFirewallRule: func(_ context.Context, _ string, _ int) (*FirewallRuleInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:firewall_rule::allow-ssh", "cluster/0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing rule, got %+v", state)
	}
}

func TestProxmoxReadFirewallRule_VMScope_ParsesExternalID(t *testing.T) {
	var gotScope string
	var gotPos int
	p := newTestProvider(&mockProxmoxAPI{
		readFirewallRule: func(_ context.Context, scope string, pos int) (*FirewallRuleInfo, error) {
			gotScope, gotPos = scope, pos
			return &FirewallRuleInfo{Pos: pos, Type: "in", Action: "ACCEPT", DPort: "22", Enable: true}, nil
		},
	})
	state, err := p.Read(context.Background(), "proxmox:firewall_rule::allow-ssh", "pve/100/3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotScope != "pve/100" || gotPos != 3 {
		t.Errorf("want scope pve/100 pos 3, got %s pos %d", gotScope, gotPos)
	}
	if state.Inputs["dport"] != "22" || state.Inputs["action"] != "ACCEPT" {
		t.Errorf("inputs not mapped: %+v", state.Inputs)
	}
}

func TestProxmoxDeleteFirewallRule_ParsesExternalID(t *testing.T) {
	var gotScope string
	var gotPos int
	p := newTestProvider(&mockProxmoxAPI{
		deleteFirewallRule: func(_ context.Context, scope string, pos int) error {
			gotScope, gotPos = scope, pos
			return nil
		},
	})
	err := p.Delete(context.Background(), &core.ResourceState{
		Type: "proxmox:firewall_rule", ExternalID: "cluster/2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotScope != "cluster" || gotPos != 2 {
		t.Errorf("want cluster/2, got %s/%d", gotScope, gotPos)
	}
}

// ── Snapshots ─────────────────────────────────────────────────────────────────

func TestProxmoxCreateSnapshot_RequiresVMID(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	_, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:snapshot", Name: "pre-upgrade", Inputs: core.Inputs{},
	})
	if err == nil || !strings.Contains(err.Error(), "requires vmid") {
		t.Fatalf("want vmid-required error, got %v", err)
	}
}

func TestProxmoxCreateSnapshot_NameFallbackAndExternalID(t *testing.T) {
	var gotName string
	var gotVMState bool
	p := newTestProvider(&mockProxmoxAPI{
		createSnapshot: func(_ context.Context, vmid int, name, _ string, vmstate bool) error {
			gotName, gotVMState = name, vmstate
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:snapshot", Name: "pre-upgrade",
		Inputs: core.Inputs{"vmid": 100, "vmstate": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "pre-upgrade" || !gotVMState {
		t.Errorf("want name pre-upgrade vmstate true, got %s %v", gotName, gotVMState)
	}
	if state.ExternalID != "100/pre-upgrade" {
		t.Errorf("want externalID 100/pre-upgrade, got %q", state.ExternalID)
	}
}

func TestProxmoxReadSnapshot_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readSnapshot: func(_ context.Context, _ int, _ string) (*SnapshotInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:snapshot::pre-upgrade", "100/pre-upgrade")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing snapshot, got %+v", state)
	}
}

func TestProxmoxUpdateSnapshot_ReturnsImmutableError(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	_, err := p.Update(context.Background(),
		&core.ResourceState{Type: "proxmox:snapshot", ExternalID: "100/s1"},
		core.ResourceArgs{Type: "proxmox:snapshot", Name: "s1"})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("want immutable error, got %v", err)
	}
}

func TestProxmoxDeleteSnapshot_ParsesExternalID(t *testing.T) {
	var gotVMID int
	var gotName string
	p := newTestProvider(&mockProxmoxAPI{
		deleteSnapshot: func(_ context.Context, vmid int, name string) error {
			gotVMID, gotName = vmid, name
			return nil
		},
	})
	err := p.Delete(context.Background(), &core.ResourceState{
		Type: "proxmox:snapshot", ExternalID: "100/pre-upgrade",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotVMID != 100 || gotName != "pre-upgrade" {
		t.Errorf("want 100/pre-upgrade, got %d/%s", gotVMID, gotName)
	}
}

// ── Pools ─────────────────────────────────────────────────────────────────────

func TestProxmoxCreatePool_NameFallback(t *testing.T) {
	var gotPool, gotComment string
	p := newTestProvider(&mockProxmoxAPI{
		createPool: func(_ context.Context, poolid, comment string) error {
			gotPool, gotComment = poolid, comment
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:pool", Name: "prod", Inputs: core.Inputs{"comment": "production"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPool != "prod" || gotComment != "production" {
		t.Errorf("want prod/production, got %s/%s", gotPool, gotComment)
	}
	if state.ExternalID != "prod" {
		t.Errorf("want externalID prod, got %q", state.ExternalID)
	}
}

func TestProxmoxReadPool_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readPool: func(_ context.Context, _ string) (*PoolInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:pool::prod", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state for missing pool, got %+v", state)
	}
}

func TestProxmoxUpdatePool_SendsComment(t *testing.T) {
	var gotComment string
	p := newTestProvider(&mockProxmoxAPI{
		updatePool: func(_ context.Context, _, comment string) error {
			gotComment = comment
			return nil
		},
		readPool: func(_ context.Context, poolid string) (*PoolInfo, error) {
			return &PoolInfo{PoolID: poolid, Comment: gotComment}, nil
		},
	})
	state, err := p.Update(context.Background(),
		&core.ResourceState{Type: "proxmox:pool", ExternalID: "prod"},
		core.ResourceArgs{Type: "proxmox:pool", Name: "prod", Inputs: core.Inputs{"comment": "updated"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotComment != "updated" {
		t.Errorf("want comment updated, got %q", gotComment)
	}
	if state.Inputs["comment"] != "updated" {
		t.Errorf("state not refreshed: %+v", state.Inputs)
	}
}

// ── Backup jobs ───────────────────────────────────────────────────────────────

func TestProxmoxCreateBackupJob_StoresReturnedID(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		createBackupJob: func(_ context.Context, job BackupJobInfo) (string, error) {
			if job.Storage != "local" || job.Schedule != "02:00" || !job.Enabled {
				return "", errors.New("job fields not mapped")
			}
			return "backup-abc123", nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:backup", Name: "nightly",
		Inputs: core.Inputs{"vmid": "all", "storage": "local", "schedule": "02:00", "enabled": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ExternalID != "backup-abc123" {
		t.Errorf("want externalID backup-abc123, got %q", state.ExternalID)
	}
}

func TestProxmoxReadBackupJob_MapsFields(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readBackupJob: func(_ context.Context, id string) (*BackupJobInfo, error) {
			return &BackupJobInfo{ID: id, VMID: "100,101", Storage: "local", Mode: "snapshot", Enabled: true}, nil
		},
	})
	state, err := p.Read(context.Background(), "proxmox:backup::nightly", "backup-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Inputs["vmid"] != "100,101" || state.Inputs["mode"] != "snapshot" {
		t.Errorf("inputs not mapped: %+v", state.Inputs)
	}
	if state.Inputs["enabled"] != true {
		t.Errorf("want enabled true, got %v", state.Inputs["enabled"])
	}
}

func TestProxmoxDeleteBackupJob_UsesExternalID(t *testing.T) {
	var gotID string
	p := newTestProvider(&mockProxmoxAPI{
		deleteBackupJob: func(_ context.Context, id string) error {
			gotID = id
			return nil
		},
	})
	err := p.Delete(context.Background(), &core.ResourceState{Type: "proxmox:backup", ExternalID: "backup-abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotID != "backup-abc123" {
		t.Errorf("want backup-abc123, got %q", gotID)
	}
}

// ── SDN ───────────────────────────────────────────────────────────────────────

func TestProxmoxCreateSDNZone_NameFallback(t *testing.T) {
	var got SDNZoneInfo
	p := newTestProvider(&mockProxmoxAPI{
		createSDNZone: func(_ context.Context, zone SDNZoneInfo) error {
			got = zone
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:sdn_zone", Name: "lab",
		Inputs: core.Inputs{"type": "simple", "mtu": 1400},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Zone != "lab" || got.Type != "simple" || got.MTU != 1400 {
		t.Errorf("zone not mapped: %+v", got)
	}
	if state.ExternalID != "lab" {
		t.Errorf("want externalID lab, got %q", state.ExternalID)
	}
}

func TestProxmoxReadSDNZone_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readSDNZone: func(_ context.Context, _ string) (*SDNZoneInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:sdn_zone::lab", "lab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state, got %+v", state)
	}
}

func TestProxmoxCreateSDNSubnet_ExternalIDCombinesVnet(t *testing.T) {
	var gotVnet string
	var gotInfo SDNSubnetInfo
	p := newTestProvider(&mockProxmoxAPI{
		createSDNSubnet: func(_ context.Context, vnet string, info SDNSubnetInfo) error {
			gotVnet, gotInfo = vnet, info
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:sdn_subnet", Name: "lab-subnet",
		Inputs: core.Inputs{"vnet": "vnet1", "subnet": "10.0.0.0/24", "gateway": "10.0.0.1", "snat": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotVnet != "vnet1" || gotInfo.Subnet != "10.0.0.0/24" || !gotInfo.SNAT {
		t.Errorf("subnet not mapped: vnet=%s info=%+v", gotVnet, gotInfo)
	}
	if state.ExternalID != "vnet1/10.0.0.0/24" {
		t.Errorf("want externalID vnet1/10.0.0.0/24, got %q", state.ExternalID)
	}
}

func TestProxmoxDeleteSDNSubnet_ParsesExternalID(t *testing.T) {
	var gotVnet, gotSubnet string
	p := newTestProvider(&mockProxmoxAPI{
		deleteSDNSubnet: func(_ context.Context, vnet, subnet string) error {
			gotVnet, gotSubnet = vnet, subnet
			return nil
		},
	})
	err := p.Delete(context.Background(), &core.ResourceState{
		Type: "proxmox:sdn_subnet", ExternalID: "vnet1/10.0.0.0/24",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotVnet != "vnet1" || gotSubnet != "10.0.0.0/24" {
		t.Errorf("want vnet1 + 10.0.0.0/24, got %s + %s", gotVnet, gotSubnet)
	}
}

// ── HA ────────────────────────────────────────────────────────────────────────

func TestProxmoxCreateHAGroup_NameFallback(t *testing.T) {
	var got HAGroupInfo
	p := newTestProvider(&mockProxmoxAPI{
		createHAGroup: func(_ context.Context, group HAGroupInfo) error {
			got = group
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:ha_group", Name: "critical",
		Inputs: core.Inputs{"nodes": "pve1:2,pve2:1", "restricted": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Group != "critical" || got.Nodes != "pve1:2,pve2:1" || !got.Restricted {
		t.Errorf("group not mapped: %+v", got)
	}
	if state.ExternalID != "critical" {
		t.Errorf("want externalID critical, got %q", state.ExternalID)
	}
}

func TestProxmoxReadHAResource_MapsFields(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readHAResource: func(_ context.Context, sid string) (*HAResourceInfo, error) {
			return &HAResourceInfo{SID: sid, Group: "critical", State: "started", MaxRestart: 2}, nil
		},
	})
	state, err := p.Read(context.Background(), "proxmox:ha_resource::web", "vm:100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Inputs["group"] != "critical" || state.Inputs["state"] != "started" || state.Inputs["max_restart"] != 2 {
		t.Errorf("inputs not mapped: %+v", state.Inputs)
	}
}

// ── ACME ──────────────────────────────────────────────────────────────────────

func TestProxmoxCreateACMEAccount_NameFallback(t *testing.T) {
	var got ACMEAccountInfo
	p := newTestProvider(&mockProxmoxAPI{
		createACMEAccount: func(_ context.Context, acct ACMEAccountInfo) error {
			got = acct
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:acme_account", Name: "default",
		Inputs: core.Inputs{"contact": "admin@example.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "default" || got.Contact != "admin@example.com" {
		t.Errorf("account not mapped: %+v", got)
	}
	if state.ExternalID != "default" {
		t.Errorf("want externalID default, got %q", state.ExternalID)
	}
}

func TestProxmoxUpdateACMEAccount_ReturnsImmutableError(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	_, err := p.Update(context.Background(),
		&core.ResourceState{Type: "proxmox:acme_account", ExternalID: "default"},
		core.ResourceArgs{Type: "proxmox:acme_account", Name: "default"})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("want immutable error, got %v", err)
	}
}

// ── Users / Roles / ACLs ──────────────────────────────────────────────────────

func TestProxmoxCreateUser_MapsFields(t *testing.T) {
	var got PVEUserInfo
	p := newTestProvider(&mockProxmoxAPI{
		createUser: func(_ context.Context, user PVEUserInfo) error {
			got = user
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:user", Name: "deploy",
		Inputs: core.Inputs{"userid": "deploy@pve", "email": "d@example.com", "enable": true, "groups": "admins"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "deploy@pve" || got.Email != "d@example.com" || !got.Enable || got.Groups != "admins" {
		t.Errorf("user not mapped: %+v", got)
	}
	if state.ExternalID != "deploy@pve" {
		t.Errorf("want externalID deploy@pve, got %q", state.ExternalID)
	}
}

func TestProxmoxReadUser_NotFound_ReturnsNil(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{
		readUser: func(_ context.Context, _ string) (*PVEUserInfo, error) { return nil, nil },
	})
	state, err := p.Read(context.Background(), "proxmox:user::deploy", "deploy@pve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Errorf("want nil state, got %+v", state)
	}
}

func TestProxmoxCreateRole_NameFallback(t *testing.T) {
	var got PVERoleInfo
	p := newTestProvider(&mockProxmoxAPI{
		createRole: func(_ context.Context, role PVERoleInfo) error {
			got = role
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:role", Name: "vm-operator",
		Inputs: core.Inputs{"privs": "VM.Audit,VM.PowerMgmt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RoleID != "vm-operator" || got.Privs != "VM.Audit,VM.PowerMgmt" {
		t.Errorf("role not mapped: %+v", got)
	}
	if state.ExternalID != "vm-operator" {
		t.Errorf("want externalID vm-operator, got %q", state.ExternalID)
	}
}

func TestProxmoxCreateACL_UsesPathAsExternalID(t *testing.T) {
	var got PVEACLInfo
	p := newTestProvider(&mockProxmoxAPI{
		setACL: func(_ context.Context, acl PVEACLInfo) error {
			got = acl
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:acl", Name: "vm-access",
		Inputs: core.Inputs{"path": "/vms/100", "roles": "PVEVMUser", "users": "deploy@pve", "propagate": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != "/vms/100" || got.Roles != "PVEVMUser" || got.Users != "deploy@pve" || !got.Propagate {
		t.Errorf("acl not mapped: %+v", got)
	}
	if state.ExternalID != "/vms/100" {
		t.Errorf("want externalID /vms/100, got %q", state.ExternalID)
	}
}

func TestProxmoxUpdateACL_ReturnsImmutableError(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	_, err := p.Update(context.Background(),
		&core.ResourceState{Type: "proxmox:acl", ExternalID: "/vms/100"},
		core.ResourceArgs{Type: "proxmox:acl", Name: "vm-access"})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("want immutable error, got %v", err)
	}
}

// ── Cluster options ───────────────────────────────────────────────────────────

func TestProxmoxCreateClusterOptions_CallsUpdate(t *testing.T) {
	var got ClusterOptionsInfo
	p := newTestProvider(&mockProxmoxAPI{
		updateClusterOptions: func(_ context.Context, opts ClusterOptionsInfo) error {
			got = opts
			return nil
		},
	})
	state, err := p.Create(context.Background(), core.ResourceArgs{
		Type: "proxmox:cluster_options", Name: "cluster",
		Inputs: core.Inputs{"email_from": "pve@example.com", "migration_type": "secure"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EmailFrom != "pve@example.com" || got.MigrationType != "secure" {
		t.Errorf("options not mapped: %+v", got)
	}
	if state.ExternalID != "cluster" {
		t.Errorf("want externalID cluster, got %q", state.ExternalID)
	}
}

func TestProxmoxDeleteClusterOptions_IsNoOp(t *testing.T) {
	// No mock fields set: any API call would panic.
	p := newTestProvider(&mockProxmoxAPI{})
	err := p.Delete(context.Background(), &core.ResourceState{Type: "proxmox:cluster_options", ExternalID: "cluster"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Diff coverage for new resource types ──────────────────────────────────────

func TestProxmoxDiff_NewResourceTypes_DetectChanges(t *testing.T) {
	cases := []struct {
		rtype    core.ResourceType
		field    string
		oldValue interface{}
		newValue interface{}
	}{
		{"proxmox:firewall_rule", "action", "ACCEPT", "DROP"},
		{"proxmox:snapshot", "description", "before", "after"},
		{"proxmox:pool", "comment", "a", "b"},
		{"proxmox:backup", "schedule", "02:00", "03:00"},
		{"proxmox:sdn_zone", "mtu", 1400, 1500},
		{"proxmox:sdn_vnet", "tag", 10, 20},
		{"proxmox:sdn_subnet", "gateway", "10.0.0.1", "10.0.0.254"},
		{"proxmox:ha_group", "nodes", "pve1", "pve1,pve2"},
		{"proxmox:ha_resource", "state", "started", "stopped"},
		{"proxmox:acme_account", "contact", "a@example.com", "b@example.com"},
		{"proxmox:user", "email", "a@example.com", "b@example.com"},
		{"proxmox:role", "privs", "VM.Audit", "VM.Audit,VM.PowerMgmt"},
		{"proxmox:acl", "roles", "PVEVMUser", "PVEAdmin"},
		{"proxmox:cluster_options", "email_from", "a@example.com", "b@example.com"},
	}
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()
	for _, tc := range cases {
		current := &core.ResourceState{Inputs: core.Inputs{tc.field: tc.oldValue}}
		desired := core.ResourceArgs{Type: tc.rtype, Name: "x", Inputs: core.Inputs{tc.field: tc.newValue}}
		diff, err := p.Diff(ctx, current, desired)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.rtype, err)
		}
		if diff.Kind != core.ChangeUpdate {
			t.Errorf("%s: want ChangeUpdate on %s change, got %v", tc.rtype, tc.field, diff.Kind)
		}
	}
}

func TestProxmoxDiff_NewResourceTypes_NoChangeIsNoOp(t *testing.T) {
	p := newTestProvider(&mockProxmoxAPI{})
	ctx := context.Background()
	types := []core.ResourceType{
		"proxmox:firewall_rule", "proxmox:snapshot", "proxmox:pool", "proxmox:backup",
		"proxmox:sdn_zone", "proxmox:sdn_vnet", "proxmox:sdn_subnet",
		"proxmox:ha_group", "proxmox:ha_resource", "proxmox:acme_account",
		"proxmox:user", "proxmox:role", "proxmox:acl", "proxmox:cluster_options",
	}
	for _, rt := range types {
		inputs := core.Inputs{"comment": "same"}
		diff, err := p.Diff(ctx, &core.ResourceState{Inputs: inputs}, core.ResourceArgs{Type: rt, Name: "x", Inputs: inputs})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", rt, err)
		}
		if diff.Kind != core.ChangeNoOp {
			t.Errorf("%s: want ChangeNoOp, got %v", rt, diff.Kind)
		}
	}
}
