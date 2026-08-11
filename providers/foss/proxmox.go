package foss

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"

	"github.com/gecko-iac/gecko/internal/core"
)

// proxmoxAPI is the interface over all Proxmox operations used by this provider.
// The real implementation delegates to go-proxmox; a mock can be injected for tests.
type proxmoxAPI interface {
	NextVMID(ctx context.Context) (int, error)

	CreateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error
	ReadVM(ctx context.Context, vmid int) (*VMInfo, error)
	UpdateVM(ctx context.Context, vmid int, opts []proxmox.VirtualMachineOption) error
	StartVM(ctx context.Context, vmid int) error
	StopVM(ctx context.Context, vmid int) error
	DeleteVM(ctx context.Context, vmid int) error

	CreateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error
	ReadLXC(ctx context.Context, vmid int) (*LXCInfo, error)
	UpdateLXC(ctx context.Context, vmid int, opts []proxmox.ContainerOption) error
	StartLXC(ctx context.Context, vmid int) error
	StopLXC(ctx context.Context, vmid int) error
	DeleteLXC(ctx context.Context, vmid int) error

	CreateStorage(ctx context.Context, opts []proxmox.ClusterStorageOptions) error
	ReadStorage(ctx context.Context, name string) (*StorageInfo, error)
	UpdateStorage(ctx context.Context, name string, opts []proxmox.ClusterStorageOptions) error
	DeleteStorage(ctx context.Context, name string) error

	CreateNetwork(ctx context.Context, cfg NetworkConfig) error
	ReadNetwork(ctx context.Context, iface string) (*NetworkConfig, error)
	UpdateNetwork(ctx context.Context, iface string, cfg NetworkConfig) error
	DeleteNetwork(ctx context.Context, iface string) error
	ReloadNetwork(ctx context.Context) error

	CreateFirewallRule(ctx context.Context, scope string, rule FirewallRuleInfo) (int, error)
	ReadFirewallRule(ctx context.Context, scope string, pos int) (*FirewallRuleInfo, error)
	UpdateFirewallRule(ctx context.Context, scope string, pos int, rule FirewallRuleInfo) error
	DeleteFirewallRule(ctx context.Context, scope string, pos int) error

	CreateSnapshot(ctx context.Context, vmid int, name, description string, vmstate bool) error
	ReadSnapshot(ctx context.Context, vmid int, name string) (*SnapshotInfo, error)
	DeleteSnapshot(ctx context.Context, vmid int, name string) error

	CreatePool(ctx context.Context, poolid, comment string) error
	ReadPool(ctx context.Context, poolid string) (*PoolInfo, error)
	UpdatePool(ctx context.Context, poolid, comment, members string) error
	DeletePool(ctx context.Context, poolid string) error

	CreateBackupJob(ctx context.Context, job BackupJobInfo) (string, error)
	ReadBackupJob(ctx context.Context, id string) (*BackupJobInfo, error)
	UpdateBackupJob(ctx context.Context, id string, job BackupJobInfo) error
	DeleteBackupJob(ctx context.Context, id string) error

	// SDN
	CreateSDNZone(ctx context.Context, zone SDNZoneInfo) error
	ReadSDNZone(ctx context.Context, zone string) (*SDNZoneInfo, error)
	UpdateSDNZone(ctx context.Context, zone string, info SDNZoneInfo) error
	DeleteSDNZone(ctx context.Context, zone string) error

	CreateSDNVnet(ctx context.Context, vnet SDNVnetInfo) error
	ReadSDNVnet(ctx context.Context, vnet string) (*SDNVnetInfo, error)
	UpdateSDNVnet(ctx context.Context, vnet string, info SDNVnetInfo) error
	DeleteSDNVnet(ctx context.Context, vnet string) error

	CreateSDNSubnet(ctx context.Context, vnet string, subnet SDNSubnetInfo) error
	ReadSDNSubnet(ctx context.Context, vnet, subnet string) (*SDNSubnetInfo, error)
	UpdateSDNSubnet(ctx context.Context, vnet, subnet string, info SDNSubnetInfo) error
	DeleteSDNSubnet(ctx context.Context, vnet, subnet string) error

	// HA
	CreateHAGroup(ctx context.Context, group HAGroupInfo) error
	ReadHAGroup(ctx context.Context, group string) (*HAGroupInfo, error)
	UpdateHAGroup(ctx context.Context, group string, info HAGroupInfo) error
	DeleteHAGroup(ctx context.Context, group string) error

	CreateHAResource(ctx context.Context, res HAResourceInfo) error
	ReadHAResource(ctx context.Context, sid string) (*HAResourceInfo, error)
	UpdateHAResource(ctx context.Context, sid string, info HAResourceInfo) error
	DeleteHAResource(ctx context.Context, sid string) error

	// ACME
	CreateACMEAccount(ctx context.Context, acct ACMEAccountInfo) error
	ReadACMEAccount(ctx context.Context, name string) (*ACMEAccountInfo, error)
	DeleteACMEAccount(ctx context.Context, name string) error

	// Users/Roles/ACLs
	CreateUser(ctx context.Context, user PVEUserInfo) error
	ReadUser(ctx context.Context, userid string) (*PVEUserInfo, error)
	UpdateUser(ctx context.Context, userid string, user PVEUserInfo) error
	DeleteUser(ctx context.Context, userid string) error

	CreateRole(ctx context.Context, role PVERoleInfo) error
	ReadRole(ctx context.Context, roleid string) (*PVERoleInfo, error)
	UpdateRole(ctx context.Context, roleid string, role PVERoleInfo) error
	DeleteRole(ctx context.Context, roleid string) error

	SetACL(ctx context.Context, acl PVEACLInfo) error
	ReadACL(ctx context.Context, path string) (*PVEACLInfo, error)
	DeleteACL(ctx context.Context, path string) error

	// Cluster
	ReadClusterOptions(ctx context.Context) (*ClusterOptionsInfo, error)
	UpdateClusterOptions(ctx context.Context, opts ClusterOptionsInfo) error
}

// ── Value types returned by the interface (no library type leakage) ────────────

// VMInfo holds the fields gecko cares about for a Proxmox VM.
type VMInfo struct {
	VMID   int
	Name   string
	Status string
	Memory int
	Cores  int

	// CPU & compute
	Sockets  int
	CPU      string // cpu type/model
	Balloon  int
	Numa     bool
	Vcpus    int

	// Boot & firmware
	Boot    string
	OnBoot  bool
	OSType  string
	Machine string
	Bios    string

	// Display & devices
	VGA     string
	Agent   string
	Serial0 string
	SCSIHW  string

	// Metadata
	Tags        string
	Protection  bool
	Description string

	// Disks
	SCSI0 string
	SCSI1 string
	SCSI2 string
	SCSI3 string
	IDE2  string

	// Network
	Net0 string
	Net1 string
	Net2 string
	Net3 string

	// Cloud-init
	CIUser       string
	CIPassword   string
	IPConfig0    string
	SSHKeys      string
	Nameserver   string
	Searchdomain string
	CICustom     string
}

// LXCInfo holds the fields gecko cares about for a Proxmox LXC container.
type LXCInfo struct {
	VMID     int
	Hostname string
	Status   string
	Memory   int
	Cores    int

	// Resources
	Swap     int
	CPULimit float64
	CPUUnits int

	// Config
	Storage      string
	OnBoot       bool
	Protection   bool
	Unprivileged bool
	Description  string
	Tags         string
	Features     string

	// DNS
	Nameserver   string
	Searchdomain string

	// SSH
	SSHPublicKeys string

	// Mount points
	RootFS string
	MP0    string
	MP1    string
	MP2    string
	MP3    string

	// Network
	Net0 string
	Net1 string
	Net2 string
	Net3 string
}

// StorageInfo holds the fields gecko cares about for a Proxmox cluster storage.
type StorageInfo struct {
	Name    string
	Type    string
	Content string

	Shared       bool
	Disable      bool
	PruneBackups string
	Pool         string
	MonHost      string
	Username     string
	IsMountpoint bool
}

// NetworkConfig holds the fields for a Proxmox node network interface.
type NetworkConfig struct {
	Iface       string
	Type        string
	CIDR        string
	Gateway     string
	BridgePorts string
	Comments    string
	Autostart   bool

	VlanID          int
	MTU             int
	BridgeVlanAware bool
	BondMode        string
	BondPrimary     string
	Address         string
	Netmask         string
	Address6        string
	Gateway6        string
}

// FirewallRuleInfo holds a single Proxmox firewall rule.
type FirewallRuleInfo struct {
	Pos     int
	Type    string // "in" or "out"
	Action  string // ACCEPT, DROP, REJECT
	Source  string
	Dest    string
	Proto   string
	DPort   string
	SPort   string
	Enable  bool
	Comment string
	Log     string
	Macro   string
	Iface   string
}

// SnapshotInfo holds a VM/CT snapshot.
type SnapshotInfo struct {
	Name        string
	Description string
	VMState     bool
}

// PoolInfo holds a Proxmox resource pool.
type PoolInfo struct {
	PoolID  string
	Comment string
	Members string // comma-separated VM/CT IDs; "" means unmanaged
}

// BackupJobInfo holds a Proxmox backup job configuration.
type BackupJobInfo struct {
	ID           string
	VMID         string // VM/CT ID or "all"
	Storage      string
	Mode         string // snapshot, suspend, stop
	Compress     string // zstd, lzo, gzip, 0
	Mailto       string
	Schedule     string
	Enabled      bool
	PruneBackups string
}

// SDNZoneInfo holds a Proxmox SDN zone.
type SDNZoneInfo struct {
	Zone       string
	Type       string // simple, vlan, qinq, vxlan, evpn
	Bridge     string
	MTU        int
	Nodes      string
	DNS        string
	ReverseDNS string
	DNSZone    string
	IPAM       string
}

// SDNVnetInfo holds a Proxmox SDN virtual network.
type SDNVnetInfo struct {
	Vnet      string
	Zone      string
	Tag       int
	Alias     string
	VlanAware bool
}

// SDNSubnetInfo holds a Proxmox SDN subnet.
type SDNSubnetInfo struct {
	Subnet        string // CIDR
	Vnet          string
	Gateway       string
	SNAT          bool
	DNSZonePrefix string
}

// HAGroupInfo holds a Proxmox HA group.
type HAGroupInfo struct {
	Group      string
	Nodes      string // e.g. "node1:2,node2:1"
	Restricted bool
	NoFailback bool
	Comment    string
}

// HAResourceInfo holds a Proxmox HA resource.
type HAResourceInfo struct {
	SID          string // e.g. "vm:100", "ct:101"
	Group        string
	MaxRelocate  int
	MaxRestart   int
	State        string // started, stopped, disabled
	Comment      string
}

// ACMEAccountInfo holds a Proxmox ACME account.
type ACMEAccountInfo struct {
	Name      string
	Contact   string
	Directory string
}

// PVEUserInfo holds a Proxmox user.
type PVEUserInfo struct {
	UserID    string // e.g. "user@pam"
	Password  string // write-only: used on create, never read back
	Email     string
	FirstName string
	LastName  string
	Groups    string
	Enable    bool
	Expire    int
	Comment   string
}

// PVERoleInfo holds a Proxmox role.
type PVERoleInfo struct {
	RoleID string
	Privs  string
}

// PVEACLInfo holds a Proxmox ACL entry.
type PVEACLInfo struct {
	Path      string
	Roles     string
	Users     string
	Groups    string
	Propagate bool
}

// ClusterOptionsInfo holds Proxmox cluster-wide options.
type ClusterOptionsInfo struct {
	Keyboard        string
	Language         string
	EmailFrom       string
	MigrationType   string
	MigrationNetwork string
}

// ── ProxmoxProvider ───────────────────────────────────────────────────────────

// ProxmoxProvider manages Proxmox VE resources via the PVE REST API.
type ProxmoxProvider struct {
	endpoint    string
	tokenID     string
	tokenSecret string
	node        string
	insecure    bool
	api         proxmoxAPI
}

// NewProxmoxProvider creates a new Proxmox provider with optional config.
func NewProxmoxProvider(config map[string]interface{}) *ProxmoxProvider {
	p := &ProxmoxProvider{
		endpoint: "https://127.0.0.1:8006",
		node:     "pve",
	}
	if config != nil {
		if v, ok := config["endpoint"].(string); ok && v != "" {
			p.endpoint = v
		}
		if v, ok := config["token_id"].(string); ok && v != "" {
			p.tokenID = v
		}
		if v, ok := config["token_secret"].(string); ok && v != "" {
			p.tokenSecret = v
		}
		if v, ok := config["node"].(string); ok && v != "" {
			p.node = v
		}
		if v, ok := config["insecure"].(bool); ok {
			p.insecure = v
		}
	}
	return p
}

func (p *ProxmoxProvider) Name() string    { return "proxmox" }
func (p *ProxmoxProvider) Version() string { return "0.1.0" }
func (p *ProxmoxProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"proxmox:vm",
		"proxmox:lxc",
		"proxmox:storage",
		"proxmox:network",
		"proxmox:firewall_rule",
		"proxmox:snapshot",
		"proxmox:pool",
		"proxmox:backup",
		"proxmox:sdn_zone",
		"proxmox:sdn_vnet",
		"proxmox:sdn_subnet",
		"proxmox:ha_group",
		"proxmox:ha_resource",
		"proxmox:acme_account",
		"proxmox:user",
		"proxmox:role",
		"proxmox:acl",
		"proxmox:cluster_options",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *ProxmoxProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["endpoint"].(string); ok && v != "" {
			p.endpoint = v
		}
		if v, ok := config["token_id"].(string); ok && v != "" {
			p.tokenID = v
		}
		if v, ok := config["token_secret"].(string); ok && v != "" {
			p.tokenSecret = v
		}
		if v, ok := config["node"].(string); ok && v != "" {
			p.node = v
		}
		if v, ok := config["insecure"].(bool); ok {
			p.insecure = v
		}
	}
	return nil
}

func (p *ProxmoxProvider) connect() error {
	if p.api != nil {
		return nil
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.insecure}, //nolint:gosec
		},
	}
	client := proxmox.NewClient(p.endpoint,
		proxmox.WithHTTPClient(httpClient),
		proxmox.WithAPIToken(p.tokenID, p.tokenSecret),
	)
	p.api = newRealProxmoxAPI(client, p.node)
	return nil
}

func (p *ProxmoxProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func proxmoxResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *ProxmoxProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "proxmox:vm":
		return p.createVM(ctx, args)
	case "proxmox:lxc":
		return p.createLXC(ctx, args)
	case "proxmox:storage":
		return p.createStorage(ctx, args)
	case "proxmox:network":
		return p.createNetwork(ctx, args)
	case "proxmox:firewall_rule":
		return p.createFirewallRule(ctx, args)
	case "proxmox:snapshot":
		return p.createSnapshot(ctx, args)
	case "proxmox:pool":
		return p.createPool(ctx, args)
	case "proxmox:backup":
		return p.createBackupJob(ctx, args)
	case "proxmox:sdn_zone":
		return p.createSDNZone(ctx, args)
	case "proxmox:sdn_vnet":
		return p.createSDNVnet(ctx, args)
	case "proxmox:sdn_subnet":
		return p.createSDNSubnet(ctx, args)
	case "proxmox:ha_group":
		return p.createHAGroup(ctx, args)
	case "proxmox:ha_resource":
		return p.createHAResource(ctx, args)
	case "proxmox:acme_account":
		return p.createACMEAccount(ctx, args)
	case "proxmox:user":
		return p.createUser(ctx, args)
	case "proxmox:role":
		return p.createRole(ctx, args)
	case "proxmox:acl":
		return p.createACL(ctx, args)
	case "proxmox:cluster_options":
		return p.createClusterOptions(ctx, args)
	default:
		return nil, fmt.Errorf("proxmox: unsupported resource type %q", args.Type)
	}
}

func (p *ProxmoxProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("proxmox: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "proxmox:vm":
		return p.readVM(ctx, id, externalID)
	case "proxmox:lxc":
		return p.readLXC(ctx, id, externalID)
	case "proxmox:storage":
		return p.readStorage(ctx, id, externalID)
	case "proxmox:network":
		return p.readNetwork(ctx, id, externalID)
	case "proxmox:firewall_rule":
		return p.readFirewallRule(ctx, id, externalID)
	case "proxmox:snapshot":
		return p.readSnapshot(ctx, id, externalID)
	case "proxmox:pool":
		return p.readPool(ctx, id, externalID)
	case "proxmox:backup":
		return p.readBackupJob(ctx, id, externalID)
	case "proxmox:sdn_zone":
		return p.readSDNZoneRes(ctx, id, externalID)
	case "proxmox:sdn_vnet":
		return p.readSDNVnetRes(ctx, id, externalID)
	case "proxmox:sdn_subnet":
		return p.readSDNSubnetRes(ctx, id, externalID)
	case "proxmox:ha_group":
		return p.readHAGroupRes(ctx, id, externalID)
	case "proxmox:ha_resource":
		return p.readHAResourceRes(ctx, id, externalID)
	case "proxmox:acme_account":
		return p.readACMEAccountRes(ctx, id, externalID)
	case "proxmox:user":
		return p.readUserRes(ctx, id, externalID)
	case "proxmox:role":
		return p.readRoleRes(ctx, id, externalID)
	case "proxmox:acl":
		return p.readACLRes(ctx, id, externalID)
	case "proxmox:cluster_options":
		return p.readClusterOptionsRes(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("proxmox: unsupported resource type %q", rType)
	}
}

func (p *ProxmoxProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "proxmox:vm":
		return p.updateVM(ctx, current, desired)
	case "proxmox:lxc":
		return p.updateLXC(ctx, current, desired)
	case "proxmox:storage":
		return p.updateStorage(ctx, current, desired)
	case "proxmox:network":
		return p.updateNetwork(ctx, current, desired)
	case "proxmox:firewall_rule":
		return p.updateFirewallRule(ctx, current, desired)
	case "proxmox:pool":
		return p.updatePool(ctx, current, desired)
	case "proxmox:backup":
		return p.updateBackupJob(ctx, current, desired)
	case "proxmox:snapshot":
		return nil, fmt.Errorf("proxmox: snapshots are immutable — delete and recreate")
	case "proxmox:sdn_zone":
		return p.updateSDNZoneRes(ctx, current, desired)
	case "proxmox:sdn_vnet":
		return p.updateSDNVnetRes(ctx, current, desired)
	case "proxmox:sdn_subnet":
		return p.updateSDNSubnetRes(ctx, current, desired)
	case "proxmox:ha_group":
		return p.updateHAGroupRes(ctx, current, desired)
	case "proxmox:ha_resource":
		return p.updateHAResourceRes(ctx, current, desired)
	case "proxmox:user":
		return p.updateUserRes(ctx, current, desired)
	case "proxmox:role":
		return p.updateRoleRes(ctx, current, desired)
	case "proxmox:cluster_options":
		return p.updateClusterOptionsRes(ctx, current, desired)
	case "proxmox:acme_account", "proxmox:acl":
		return nil, fmt.Errorf("proxmox: %s is immutable — delete and recreate", desired.Type)
	default:
		return nil, fmt.Errorf("proxmox: unsupported resource type %q", desired.Type)
	}
}

func (p *ProxmoxProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "proxmox:vm":
		return p.deleteVM(ctx, state)
	case "proxmox:lxc":
		return p.deleteLXC(ctx, state)
	case "proxmox:storage":
		return p.deleteStorage(ctx, state)
	case "proxmox:network":
		return p.deleteNetwork(ctx, state)
	case "proxmox:firewall_rule":
		return p.deleteFirewallRule(ctx, state)
	case "proxmox:snapshot":
		return p.deleteSnapshot(ctx, state)
	case "proxmox:pool":
		return p.deletePool(ctx, state)
	case "proxmox:backup":
		return p.deleteBackupJob(ctx, state)
	case "proxmox:sdn_zone":
		return p.api.DeleteSDNZone(ctx, state.ExternalID)
	case "proxmox:sdn_vnet":
		return p.api.DeleteSDNVnet(ctx, state.ExternalID)
	case "proxmox:sdn_subnet":
		parts := strings.SplitN(state.ExternalID, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("proxmox: invalid sdn_subnet externalID %q", state.ExternalID)
		}
		return p.api.DeleteSDNSubnet(ctx, parts[0], parts[1])
	case "proxmox:ha_group":
		return p.api.DeleteHAGroup(ctx, state.ExternalID)
	case "proxmox:ha_resource":
		return p.api.DeleteHAResource(ctx, state.ExternalID)
	case "proxmox:acme_account":
		return p.api.DeleteACMEAccount(ctx, state.ExternalID)
	case "proxmox:user":
		return p.api.DeleteUser(ctx, state.ExternalID)
	case "proxmox:role":
		return p.api.DeleteRole(ctx, state.ExternalID)
	case "proxmox:acl":
		return p.api.DeleteACL(ctx, state.ExternalID)
	case "proxmox:cluster_options":
		return nil // cluster options cannot be deleted, only updated
	default:
		return fmt.Errorf("proxmox: unsupported resource type %q", state.Type)
	}
}

func (p *ProxmoxProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *ProxmoxProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := proxmoxResourceID(desired)

	if current == nil {
		return &core.Diff{
			ResourceID: id,
			Kind:       core.ChangeAdd,
			Changes: []core.FieldChange{
				{Field: "name", Kind: core.ChangeAdd, NewValue: desired.Name},
			},
		}, nil
	}

	var changes []core.FieldChange
	switch desired.Type {
	case "proxmox:vm":
		for _, f := range []string{
			"name", "memory", "cores", "sockets", "cpu", "balloon", "numa", "vcpus",
			"boot", "onboot", "ostype", "machine", "bios",
			"vga", "agent", "serial0", "scsihw",
			"tags", "protection", "description",
			"scsi0", "scsi1", "scsi2", "scsi3", "ide2",
			"net0", "net1", "net2", "net3",
			"ciuser", "ipconfig0", "sshkeys", "nameserver", "searchdomain", "cicustom",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:lxc":
		for _, f := range []string{
			"hostname", "memory", "cores", "swap", "cpuunits",
			"onboot", "protection", "unprivileged",
			"description", "tags", "features",
			"nameserver", "searchdomain", "ssh_public_keys", "storage",
			"rootfs", "mp0", "mp1", "mp2", "mp3",
			"net0", "net1", "net2", "net3",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:storage":
		for _, f := range []string{
			"content", "nodes", "shared", "disable", "prune_backups",
			"pool", "monhost", "username", "is_mountpoint",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:network":
		for _, f := range []string{
			"cidr", "gateway", "bridge_ports", "autostart", "comments",
			"vlan_id", "mtu", "bridge_vlan_aware",
			"bond_mode", "bond_primary",
			"address", "netmask", "address6", "gateway6",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:firewall_rule":
		for _, f := range []string{
			"type", "action", "source", "dest", "proto",
			"dport", "sport", "enable", "comment", "log", "macro", "iface",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:snapshot":
		for _, f := range []string{"description", "vmstate"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:pool":
		for _, f := range []string{"comment", "members"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:backup":
		for _, f := range []string{
			"vmid", "storage", "mode", "compress",
			"mailto", "schedule", "enabled", "prune_backups",
		} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:sdn_zone":
		for _, f := range []string{"type", "bridge", "mtu", "nodes", "dns", "reversedns", "dnszone", "ipam"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:sdn_vnet":
		for _, f := range []string{"zone", "tag", "alias", "vlanaware"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:sdn_subnet":
		for _, f := range []string{"gateway", "snat", "dnszoneprefix"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:ha_group":
		for _, f := range []string{"nodes", "restricted", "nofailback", "comment"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:ha_resource":
		for _, f := range []string{"group", "max_relocate", "max_restart", "state", "comment"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:acme_account":
		for _, f := range []string{"contact", "directory"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:user":
		for _, f := range []string{"email", "firstname", "lastname", "groups", "enable", "expire", "comment"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:role":
		changes = append(changes, compareField("privs", current.Inputs, desired.Inputs)...)
	case "proxmox:acl":
		for _, f := range []string{"roles", "users", "groups", "propagate"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	case "proxmox:cluster_options":
		for _, f := range []string{"keyboard", "language", "email_from", "migration_type", "migration_network"} {
			changes = append(changes, compareField(f, current.Inputs, desired.Inputs)...)
		}
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── VM resource handlers ────────────────────────────────────────────────────

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

	inputs := core.Inputs{
		"vmid":   info.VMID,
		"name":   info.Name,
		"memory": info.Memory,
		"cores":  info.Cores,
	}
	// Only set non-zero/non-empty fields to keep state clean.
	setIfNonZero := func(k string, v int) { if v != 0 { inputs[k] = v } }
	setIfTrue := func(k string, v bool) { if v { inputs[k] = v } }
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }

	setIfNonZero("sockets", info.Sockets)
	setIfNonEmpty("cpu", info.CPU)
	setIfNonZero("balloon", info.Balloon)
	setIfTrue("numa", info.Numa)
	setIfNonZero("vcpus", info.Vcpus)
	setIfNonEmpty("boot", info.Boot)
	setIfTrue("onboot", info.OnBoot)
	setIfNonEmpty("ostype", info.OSType)
	setIfNonEmpty("machine", info.Machine)
	setIfNonEmpty("bios", info.Bios)
	setIfNonEmpty("vga", info.VGA)
	setIfNonEmpty("agent", info.Agent)
	setIfNonEmpty("serial0", info.Serial0)
	setIfNonEmpty("scsihw", info.SCSIHW)
	setIfNonEmpty("tags", info.Tags)
	setIfTrue("protection", info.Protection)
	setIfNonEmpty("description", info.Description)
	setIfNonEmpty("scsi0", info.SCSI0)
	setIfNonEmpty("scsi1", info.SCSI1)
	setIfNonEmpty("scsi2", info.SCSI2)
	setIfNonEmpty("scsi3", info.SCSI3)
	setIfNonEmpty("ide2", info.IDE2)
	setIfNonEmpty("net0", info.Net0)
	setIfNonEmpty("net1", info.Net1)
	setIfNonEmpty("net2", info.Net2)
	setIfNonEmpty("net3", info.Net3)
	setIfNonEmpty("ciuser", info.CIUser)
	setIfNonEmpty("ipconfig0", info.IPConfig0)
	setIfNonEmpty("sshkeys", info.SSHKeys)
	setIfNonEmpty("nameserver", info.Nameserver)
	setIfNonEmpty("searchdomain", info.Searchdomain)
	setIfNonEmpty("cicustom", info.CICustom)

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:vm",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     status,
		Inputs:     inputs,
		Outputs:    core.Outputs{"vmid": vmid},
		UpdatedAt:  time.Now(),
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

	// CPU & compute
	if v, ok := toInt(inputs["memory"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "memory", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["cores"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cores", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["sockets"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "sockets", Value: strconv.Itoa(v)})
	}
	if v, ok := inputs["cpu"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cpu", Value: v})
	}
	if v, ok := toInt(inputs["balloon"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "balloon", Value: strconv.Itoa(v)})
	}
	if v, ok := inputs["numa"].(bool); ok && v {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "numa", Value: "1"})
	}
	if v, ok := toInt(inputs["vcpus"]); ok {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "vcpus", Value: strconv.Itoa(v)})
	}

	// Boot & firmware
	if v, ok := inputs["boot"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "boot", Value: v})
	}
	if v, ok := inputs["onboot"].(bool); ok && v {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "onboot", Value: "1"})
	}
	if v, ok := inputs["ostype"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "ostype", Value: v})
	}
	if v, ok := inputs["machine"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "machine", Value: v})
	}
	if v, ok := inputs["bios"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "bios", Value: v})
	}

	// Display & devices
	if v, ok := inputs["vga"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "vga", Value: v})
	}
	if v, ok := inputs["agent"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "agent", Value: v})
	}
	if v, ok := inputs["serial0"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "serial0", Value: v})
	}
	if v, ok := inputs["scsihw"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "scsihw", Value: v})
	}

	// Metadata
	if v, ok := inputs["tags"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "tags", Value: v})
	}
	if v, ok := inputs["protection"].(bool); ok && v {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "protection", Value: "1"})
	}
	if v, ok := inputs["description"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "description", Value: v})
	}

	// Disks
	if v, ok := inputs["iso"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cdrom", Value: v})
	}
	if v, ok := inputs["ide2"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "ide2", Value: v})
	}
	if v, ok := inputs["disk_size"].(string); ok && v != "" {
		storage := getStringInputFallback(inputs, "storage", "local-lvm")
		scsihw := getStringInputFallback(inputs, "scsihw", "virtio-scsi-pci")
		opts = append(opts, proxmox.VirtualMachineOption{Name: "scsi0", Value: storage + ":" + v})
		opts = append(opts, proxmox.VirtualMachineOption{Name: "scsihw", Value: scsihw})
	}
	for _, disk := range []string{"scsi1", "scsi2", "scsi3"} {
		if v, ok := inputs[disk].(string); ok && v != "" {
			opts = append(opts, proxmox.VirtualMachineOption{Name: disk, Value: v})
		}
	}

	// Network
	for _, nic := range []string{"net0", "net1", "net2", "net3"} {
		if v, ok := inputs[nic].(string); ok && v != "" {
			opts = append(opts, proxmox.VirtualMachineOption{Name: nic, Value: v})
		}
	}

	// Cloud-init
	if v, ok := inputs["ciuser"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "ciuser", Value: v})
	}
	if v, ok := inputs["cipassword"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cipassword", Value: v})
	}
	if v, ok := inputs["ipconfig0"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "ipconfig0", Value: v})
	}
	if v, ok := inputs["sshkeys"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "sshkeys", Value: v})
	}
	if v, ok := inputs["nameserver"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "nameserver", Value: v})
	}
	if v, ok := inputs["searchdomain"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "searchdomain", Value: v})
	}
	if v, ok := inputs["cicustom"].(string); ok && v != "" {
		opts = append(opts, proxmox.VirtualMachineOption{Name: "cicustom", Value: v})
	}

	return opts
}

// ── LXC resource handlers ───────────────────────────────────────────────────

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

	inputs := core.Inputs{
		"vmid":     info.VMID,
		"hostname": info.Hostname,
		"memory":   info.Memory,
		"cores":    info.Cores,
	}
	setIfNonZero := func(k string, v int) { if v != 0 { inputs[k] = v } }
	setIfTrue := func(k string, v bool) { if v { inputs[k] = v } }
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }

	setIfNonZero("swap", info.Swap)
	setIfNonZero("cpuunits", info.CPUUnits)
	setIfTrue("onboot", info.OnBoot)
	setIfTrue("protection", info.Protection)
	setIfTrue("unprivileged", info.Unprivileged)
	setIfNonEmpty("description", info.Description)
	setIfNonEmpty("tags", info.Tags)
	setIfNonEmpty("features", info.Features)
	setIfNonEmpty("nameserver", info.Nameserver)
	setIfNonEmpty("searchdomain", info.Searchdomain)
	setIfNonEmpty("ssh_public_keys", info.SSHPublicKeys)
	setIfNonEmpty("storage", info.Storage)
	setIfNonEmpty("rootfs", info.RootFS)
	setIfNonEmpty("mp0", info.MP0)
	setIfNonEmpty("mp1", info.MP1)
	setIfNonEmpty("mp2", info.MP2)
	setIfNonEmpty("mp3", info.MP3)
	setIfNonEmpty("net0", info.Net0)
	setIfNonEmpty("net1", info.Net1)
	setIfNonEmpty("net2", info.Net2)
	setIfNonEmpty("net3", info.Net3)

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:lxc",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     status,
		Inputs:     inputs,
		Outputs:    core.Outputs{"vmid": vmid},
		UpdatedAt:  time.Now(),
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
	if v, ok := toInt(inputs["swap"]); ok {
		opts = append(opts, proxmox.ContainerOption{Name: "swap", Value: strconv.Itoa(v)})
	}
	if v, ok := toInt(inputs["cpuunits"]); ok {
		opts = append(opts, proxmox.ContainerOption{Name: "cpuunits", Value: strconv.Itoa(v)})
	}
	if v, ok := inputs["rootfs"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "rootfs", Value: v})
	}
	if v, ok := inputs["password"].(string); ok && v != "" {
		opts = append(opts, proxmox.ContainerOption{Name: "password", Value: v})
	}
	if v, ok := inputs["onboot"].(bool); ok && v {
		opts = append(opts, proxmox.ContainerOption{Name: "onboot", Value: "1"})
	}
	if v, ok := inputs["protection"].(bool); ok && v {
		opts = append(opts, proxmox.ContainerOption{Name: "protection", Value: "1"})
	}
	if v, ok := inputs["unprivileged"].(bool); ok && v {
		opts = append(opts, proxmox.ContainerOption{Name: "unprivileged", Value: "1"})
	}
	for _, s := range []string{"description", "tags", "features", "nameserver", "searchdomain", "ssh_public_keys", "storage"} {
		if v, ok := inputs[s].(string); ok && v != "" {
			apiKey := s
			if s == "ssh_public_keys" {
				apiKey = "ssh-public-keys"
			}
			opts = append(opts, proxmox.ContainerOption{Name: apiKey, Value: v})
		}
	}
	for _, nic := range []string{"net0", "net1", "net2", "net3"} {
		if v, ok := inputs[nic].(string); ok && v != "" {
			opts = append(opts, proxmox.ContainerOption{Name: nic, Value: v})
		}
	}
	for _, mp := range []string{"mp0", "mp1", "mp2", "mp3"} {
		if v, ok := inputs[mp].(string); ok && v != "" {
			opts = append(opts, proxmox.ContainerOption{Name: mp, Value: v})
		}
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

// ── Storage resource handlers ───────────────────────────────────────────────

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

	inputs := core.Inputs{
		"storage": info.Name,
		"type":    info.Type,
		"content": info.Content,
	}
	setIfTrue := func(k string, v bool) { if v { inputs[k] = v } }
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }

	setIfTrue("shared", info.Shared)
	setIfTrue("disable", info.Disable)
	setIfTrue("is_mountpoint", info.IsMountpoint)
	setIfNonEmpty("prune_backups", info.PruneBackups)
	setIfNonEmpty("pool", info.Pool)
	setIfNonEmpty("monhost", info.MonHost)
	setIfNonEmpty("username", info.Username)

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:storage",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"storage": info.Name},
		UpdatedAt:  time.Now(),
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

	for _, s := range []string{"type", "content", "path", "server", "export", "nodes", "prune_backups", "pool", "monhost", "username"} {
		if v, ok := inputs[s].(string); ok && v != "" {
			opts = append(opts, proxmox.ClusterStorageOptions{Name: s, Value: v})
		}
	}
	if v, ok := inputs["shared"].(bool); ok && v {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "shared", Value: "1"})
	}
	if v, ok := inputs["disable"].(bool); ok && v {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "disable", Value: "1"})
	}
	if v, ok := inputs["is_mountpoint"].(bool); ok && v {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "is_mountpoint", Value: "1"})
	}

	return opts
}

// ── Network resource handlers ───────────────────────────────────────────────

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
	inputs := core.Inputs{
		"iface":        cfg.Iface,
		"type":         cfg.Type,
		"cidr":         cfg.CIDR,
		"gateway":      cfg.Gateway,
		"bridge_ports": cfg.BridgePorts,
		"comments":     cfg.Comments,
		"autostart":    cfg.Autostart,
	}
	setIfNonZero := func(k string, v int) { if v != 0 { inputs[k] = v } }
	setIfTrue := func(k string, v bool) { if v { inputs[k] = v } }
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }

	setIfNonZero("vlan_id", cfg.VlanID)
	setIfNonZero("mtu", cfg.MTU)
	setIfTrue("bridge_vlan_aware", cfg.BridgeVlanAware)
	setIfNonEmpty("bond_mode", cfg.BondMode)
	setIfNonEmpty("bond_primary", cfg.BondPrimary)
	setIfNonEmpty("address", cfg.Address)
	setIfNonEmpty("netmask", cfg.Netmask)
	setIfNonEmpty("address6", cfg.Address6)
	setIfNonEmpty("gateway6", cfg.Gateway6)

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:network",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"iface": cfg.Iface, "node": nodeName},
		UpdatedAt:  time.Now(),
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
	getString := func(k string) string { v, _ := inputs[k].(string); return v }
	getBool := func(k string) bool { v, _ := inputs[k].(bool); return v }
	getInt := func(k string) int { v, _ := toInt(inputs[k]); return v }

	return NetworkConfig{
		Iface:           getStringInputFallback(inputs, "iface", name),
		Type:            getStringInputFallback(inputs, "type", "bridge"),
		CIDR:            getString("cidr"),
		Gateway:         getString("gateway"),
		BridgePorts:     getString("bridge_ports"),
		Comments:        getString("comments"),
		Autostart:       getBool("autostart"),
		VlanID:          getInt("vlan_id"),
		MTU:             getInt("mtu"),
		BridgeVlanAware: getBool("bridge_vlan_aware"),
		BondMode:        getString("bond_mode"),
		BondPrimary:     getString("bond_primary"),
		Address:         getString("address"),
		Netmask:         getString("netmask"),
		Address6:        getString("address6"),
		Gateway6:        getString("gateway6"),
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

// ── Firewall rule resource handlers ─────────────────────────────────────────

func firewallRuleFromInputs(inputs core.Inputs) FirewallRuleInfo {
	getString := func(k string) string { v, _ := inputs[k].(string); return v }
	getBool := func(k string) bool { v, _ := inputs[k].(bool); return v }
	return FirewallRuleInfo{
		Type:    getString("type"),
		Action:  getString("action"),
		Source:  getString("source"),
		Dest:    getString("dest"),
		Proto:   getString("proto"),
		DPort:   getString("dport"),
		SPort:   getString("sport"),
		Enable:  getBool("enable"),
		Comment: getString("comment"),
		Log:     getString("log"),
		Macro:   getString("macro"),
		Iface:   getString("iface"),
	}
}

// firewallScope returns the API scope path for a firewall rule.
// Cluster-level if no node/vmid, node-level if node set, VM-level if vmid set.
func firewallScope(inputs core.Inputs, node string) string {
	if vmid, ok := toInt(inputs["vmid"]); ok && vmid > 0 {
		return fmt.Sprintf("%s/%d", node, vmid)
	}
	if n, ok := inputs["node"].(string); ok && n != "" {
		return n
	}
	return "cluster"
}

func (p *ProxmoxProvider) createFirewallRule(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	scope := firewallScope(args.Inputs, p.node)
	rule := firewallRuleFromInputs(args.Inputs)

	pos, err := p.api.CreateFirewallRule(ctx, scope, rule)
	if err != nil {
		return nil, fmt.Errorf("proxmox: create firewall rule %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"pos": pos, "scope": scope},
		ExternalID: fmt.Sprintf("%s/%d", scope, pos),
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readFirewallRule(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// externalID format: "<scope>/<pos>"
	idx := strings.LastIndex(externalID, "/")
	if idx <= 0 {
		return nil, fmt.Errorf("proxmox: invalid firewall rule externalID %q", externalID)
	}
	scope := externalID[:idx]
	pos, err := strconv.Atoi(externalID[idx+1:])
	if err != nil {
		return nil, fmt.Errorf("proxmox: invalid firewall rule pos in %q: %w", externalID, err)
	}

	info, err := p.api.ReadFirewallRule(ctx, scope, pos)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	inputs := core.Inputs{
		"type":   info.Type,
		"action": info.Action,
		"enable": info.Enable,
	}
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }
	setIfNonEmpty("source", info.Source)
	setIfNonEmpty("dest", info.Dest)
	setIfNonEmpty("proto", info.Proto)
	setIfNonEmpty("dport", info.DPort)
	setIfNonEmpty("sport", info.SPort)
	setIfNonEmpty("comment", info.Comment)
	setIfNonEmpty("log", info.Log)
	setIfNonEmpty("macro", info.Macro)
	setIfNonEmpty("iface", info.Iface)

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:firewall_rule",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"pos": pos, "scope": scope},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateFirewallRule(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	idx := strings.LastIndex(current.ExternalID, "/")
	if idx <= 0 {
		return nil, fmt.Errorf("proxmox: invalid firewall rule externalID %q", current.ExternalID)
	}
	scope := current.ExternalID[:idx]
	pos, err := strconv.Atoi(current.ExternalID[idx+1:])
	if err != nil {
		return nil, err
	}

	rule := firewallRuleFromInputs(desired.Inputs)
	if err := p.api.UpdateFirewallRule(ctx, scope, pos, rule); err != nil {
		return nil, err
	}

	return p.readFirewallRule(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deleteFirewallRule(ctx context.Context, state *core.ResourceState) error {
	idx := strings.LastIndex(state.ExternalID, "/")
	if idx <= 0 {
		return fmt.Errorf("proxmox: invalid firewall rule externalID %q", state.ExternalID)
	}
	scope := state.ExternalID[:idx]
	pos, err := strconv.Atoi(state.ExternalID[idx+1:])
	if err != nil {
		return err
	}
	return p.api.DeleteFirewallRule(ctx, scope, pos)
}

// ── Snapshot resource handlers ──────────────────────────────────────────────

func (p *ProxmoxProvider) createSnapshot(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	vmid, ok := toInt(args.Inputs["vmid"])
	if !ok || vmid == 0 {
		return nil, fmt.Errorf("proxmox: snapshot requires vmid")
	}
	snapName := getStringInputFallback(args.Inputs, "name", args.Name)
	description, _ := args.Inputs["description"].(string)
	vmstate, _ := args.Inputs["vmstate"].(bool)

	if err := p.api.CreateSnapshot(ctx, vmid, snapName, description, vmstate); err != nil {
		return nil, fmt.Errorf("proxmox: create snapshot %q on vm %d: %w", snapName, vmid, err)
	}

	externalID := fmt.Sprintf("%d/%s", vmid, snapName)
	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"vmid": vmid, "snapshot": snapName},
		ExternalID: externalID,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readSnapshot(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// externalID format: "<vmid>/<snapname>"
	idx := strings.Index(externalID, "/")
	if idx <= 0 {
		return nil, fmt.Errorf("proxmox: invalid snapshot externalID %q", externalID)
	}
	vmid, err := strconv.Atoi(externalID[:idx])
	if err != nil {
		return nil, fmt.Errorf("proxmox: invalid snapshot vmid in %q: %w", externalID, err)
	}
	snapName := externalID[idx+1:]

	info, err := p.api.ReadSnapshot(ctx, vmid, snapName)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	inputs := core.Inputs{
		"vmid":    vmid,
		"name":    info.Name,
		"vmstate": info.VMState,
	}
	if info.Description != "" {
		inputs["description"] = info.Description
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:snapshot",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"vmid": vmid, "snapshot": snapName},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) deleteSnapshot(ctx context.Context, state *core.ResourceState) error {
	idx := strings.Index(state.ExternalID, "/")
	if idx <= 0 {
		return fmt.Errorf("proxmox: invalid snapshot externalID %q", state.ExternalID)
	}
	vmid, err := strconv.Atoi(state.ExternalID[:idx])
	if err != nil {
		return err
	}
	snapName := state.ExternalID[idx+1:]
	return p.api.DeleteSnapshot(ctx, vmid, snapName)
}

// ── Pool resource handlers ──────────────────────────────────────────────────

func (p *ProxmoxProvider) createPool(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	poolid := getStringInputFallback(args.Inputs, "poolid", args.Name)
	comment, _ := args.Inputs["comment"].(string)

	if err := p.api.CreatePool(ctx, poolid, comment); err != nil {
		return nil, fmt.Errorf("proxmox: create pool %q: %w", poolid, err)
	}

	// Pool creation cannot set members; assign them in a follow-up update.
	if members, _ := args.Inputs["members"].(string); members != "" {
		if err := p.api.UpdatePool(ctx, poolid, comment, members); err != nil {
			return nil, fmt.Errorf("proxmox: assign members to pool %q: %w", poolid, err)
		}
	}

	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"poolid": poolid},
		ExternalID: poolid,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readPool(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadPool(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	inputs := core.Inputs{"poolid": info.PoolID}
	if info.Comment != "" {
		inputs["comment"] = info.Comment
	}
	if info.Members != "" {
		inputs["members"] = info.Members
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:pool",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"poolid": info.PoolID},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updatePool(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	comment, _ := desired.Inputs["comment"].(string)
	members, _ := desired.Inputs["members"].(string)
	if err := p.api.UpdatePool(ctx, current.ExternalID, comment, members); err != nil {
		return nil, err
	}
	return p.readPool(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deletePool(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeletePool(ctx, state.ExternalID)
}

// ── Backup job resource handlers ────────────────────────────────────────────

func backupJobFromInputs(inputs core.Inputs) BackupJobInfo {
	getString := func(k string) string { v, _ := inputs[k].(string); return v }
	getBool := func(k string) bool { v, _ := inputs[k].(bool); return v }
	return BackupJobInfo{
		VMID:         getString("vmid"),
		Storage:      getString("storage"),
		Mode:         getString("mode"),
		Compress:     getString("compress"),
		Mailto:       getString("mailto"),
		Schedule:     getString("schedule"),
		Enabled:      getBool("enabled"),
		PruneBackups: getString("prune_backups"),
	}
}

func (p *ProxmoxProvider) createBackupJob(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	job := backupJobFromInputs(args.Inputs)

	id, err := p.api.CreateBackupJob(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("proxmox: create backup job %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         proxmoxResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"job_id": id},
		ExternalID: id,
		ProviderID: "proxmox",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) readBackupJob(ctx context.Context, resID core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadBackupJob(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	inputs := core.Inputs{"enabled": info.Enabled}
	setIfNonEmpty := func(k, v string) { if v != "" { inputs[k] = v } }
	setIfNonEmpty("vmid", info.VMID)
	setIfNonEmpty("storage", info.Storage)
	setIfNonEmpty("mode", info.Mode)
	setIfNonEmpty("compress", info.Compress)
	setIfNonEmpty("mailto", info.Mailto)
	setIfNonEmpty("schedule", info.Schedule)
	setIfNonEmpty("prune_backups", info.PruneBackups)

	return &core.ResourceState{
		ID:         resID,
		Type:       "proxmox:backup",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    core.Outputs{"job_id": externalID},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *ProxmoxProvider) updateBackupJob(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	job := backupJobFromInputs(desired.Inputs)
	if err := p.api.UpdateBackupJob(ctx, current.ExternalID, job); err != nil {
		return nil, err
	}
	return p.readBackupJob(ctx, proxmoxResourceID(desired), current.ExternalID)
}

func (p *ProxmoxProvider) deleteBackupJob(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteBackupJob(ctx, state.ExternalID)
}

// ── SDN Zone resource handlers ──────────────────────────────────────────────

func sdnZoneFromInputs(inputs core.Inputs) SDNZoneInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	gi := func(k string) int { v, _ := toInt(inputs[k]); return v }
	return SDNZoneInfo{Zone: gs("zone"), Type: gs("type"), Bridge: gs("bridge"), MTU: gi("mtu"), Nodes: gs("nodes"), DNS: gs("dns"), ReverseDNS: gs("reversedns"), DNSZone: gs("dnszone"), IPAM: gs("ipam")}
}

func sdnZoneToInputs(info *SDNZoneInfo) core.Inputs {
	inputs := core.Inputs{"zone": info.Zone, "type": info.Type}
	s := func(k, v string) { if v != "" { inputs[k] = v } }
	s("bridge", info.Bridge); s("nodes", info.Nodes); s("dns", info.DNS); s("reversedns", info.ReverseDNS); s("dnszone", info.DNSZone); s("ipam", info.IPAM)
	if info.MTU != 0 { inputs["mtu"] = info.MTU }
	return inputs
}

func (p *ProxmoxProvider) createSDNZone(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info := sdnZoneFromInputs(args.Inputs)
	if info.Zone == "" { info.Zone = args.Name }
	if err := p.api.CreateSDNZone(ctx, info); err != nil {
		return nil, fmt.Errorf("proxmox: create sdn_zone %q: %w", info.Zone, err)
	}
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"zone": info.Zone}, ExternalID: info.Zone, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readSDNZoneRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadSDNZone(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:sdn_zone", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: sdnZoneToInputs(info), Outputs: core.Outputs{"zone": info.Zone}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateSDNZoneRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	info := sdnZoneFromInputs(desired.Inputs)
	if err := p.api.UpdateSDNZone(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readSDNZoneRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── SDN Vnet resource handlers ──────────────────────────────────────────────

func sdnVnetFromInputs(inputs core.Inputs) SDNVnetInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	gi := func(k string) int { v, _ := toInt(inputs[k]); return v }
	gb := func(k string) bool { v, _ := inputs[k].(bool); return v }
	return SDNVnetInfo{Vnet: gs("vnet"), Zone: gs("zone"), Tag: gi("tag"), Alias: gs("alias"), VlanAware: gb("vlanaware")}
}

func sdnVnetToInputs(info *SDNVnetInfo) core.Inputs {
	inputs := core.Inputs{"vnet": info.Vnet, "zone": info.Zone}
	if info.Tag != 0 { inputs["tag"] = info.Tag }
	if info.Alias != "" { inputs["alias"] = info.Alias }
	if info.VlanAware { inputs["vlanaware"] = true }
	return inputs
}

func (p *ProxmoxProvider) createSDNVnet(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info := sdnVnetFromInputs(args.Inputs)
	if info.Vnet == "" { info.Vnet = args.Name }
	if err := p.api.CreateSDNVnet(ctx, info); err != nil {
		return nil, fmt.Errorf("proxmox: create sdn_vnet %q: %w", info.Vnet, err)
	}
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"vnet": info.Vnet}, ExternalID: info.Vnet, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readSDNVnetRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadSDNVnet(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:sdn_vnet", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: sdnVnetToInputs(info), Outputs: core.Outputs{"vnet": info.Vnet}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateSDNVnetRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	info := sdnVnetFromInputs(desired.Inputs)
	if err := p.api.UpdateSDNVnet(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readSDNVnetRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── SDN Subnet resource handlers ────────────────────────────────────────────

func (p *ProxmoxProvider) createSDNSubnet(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	gs := func(k string) string { v, _ := args.Inputs[k].(string); return v }
	gb := func(k string) bool { v, _ := args.Inputs[k].(bool); return v }
	vnet := gs("vnet")
	info := SDNSubnetInfo{Subnet: gs("subnet"), Vnet: vnet, Gateway: gs("gateway"), SNAT: gb("snat"), DNSZonePrefix: gs("dnszoneprefix")}
	if err := p.api.CreateSDNSubnet(ctx, vnet, info); err != nil {
		return nil, fmt.Errorf("proxmox: create sdn_subnet %q: %w", info.Subnet, err)
	}
	eid := vnet + "/" + info.Subnet
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"subnet": info.Subnet, "vnet": vnet}, ExternalID: eid, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readSDNSubnetRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 { return nil, fmt.Errorf("proxmox: invalid sdn_subnet externalID %q", externalID) }
	info, err := p.api.ReadSDNSubnet(ctx, parts[0], parts[1])
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	inputs := core.Inputs{"subnet": info.Subnet, "vnet": info.Vnet}
	if info.Gateway != "" { inputs["gateway"] = info.Gateway }
	if info.SNAT { inputs["snat"] = true }
	if info.DNSZonePrefix != "" { inputs["dnszoneprefix"] = info.DNSZonePrefix }
	return &core.ResourceState{ID: id, Type: "proxmox:sdn_subnet", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: inputs, Outputs: core.Outputs{"subnet": info.Subnet, "vnet": info.Vnet}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateSDNSubnetRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	parts := strings.SplitN(current.ExternalID, "/", 2)
	if len(parts) != 2 { return nil, fmt.Errorf("proxmox: invalid sdn_subnet externalID %q", current.ExternalID) }
	gs := func(k string) string { v, _ := desired.Inputs[k].(string); return v }
	gb := func(k string) bool { v, _ := desired.Inputs[k].(bool); return v }
	info := SDNSubnetInfo{Subnet: gs("subnet"), Vnet: parts[0], Gateway: gs("gateway"), SNAT: gb("snat"), DNSZonePrefix: gs("dnszoneprefix")}
	if err := p.api.UpdateSDNSubnet(ctx, parts[0], parts[1], info); err != nil { return nil, err }
	return p.readSDNSubnetRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── HA Group resource handlers ──────────────────────────────────────────────

func haGroupFromInputs(inputs core.Inputs) HAGroupInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	gb := func(k string) bool { v, _ := inputs[k].(bool); return v }
	return HAGroupInfo{Group: gs("group"), Nodes: gs("nodes"), Restricted: gb("restricted"), NoFailback: gb("nofailback"), Comment: gs("comment")}
}

func haGroupToInputs(info *HAGroupInfo) core.Inputs {
	inputs := core.Inputs{"group": info.Group}
	if info.Nodes != "" { inputs["nodes"] = info.Nodes }
	if info.Restricted { inputs["restricted"] = true }
	if info.NoFailback { inputs["nofailback"] = true }
	if info.Comment != "" { inputs["comment"] = info.Comment }
	return inputs
}

func (p *ProxmoxProvider) createHAGroup(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info := haGroupFromInputs(args.Inputs)
	if info.Group == "" { info.Group = args.Name }
	if err := p.api.CreateHAGroup(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: create ha_group %q: %w", info.Group, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"group": info.Group}, ExternalID: info.Group, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readHAGroupRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadHAGroup(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:ha_group", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: haGroupToInputs(info), Outputs: core.Outputs{"group": info.Group}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateHAGroupRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	info := haGroupFromInputs(desired.Inputs)
	if err := p.api.UpdateHAGroup(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readHAGroupRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── HA Resource handlers ────────────────────────────────────────────────────

func haResourceFromInputs(inputs core.Inputs) HAResourceInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	gi := func(k string) int { v, _ := toInt(inputs[k]); return v }
	return HAResourceInfo{SID: gs("sid"), Group: gs("group"), MaxRelocate: gi("max_relocate"), MaxRestart: gi("max_restart"), State: gs("state"), Comment: gs("comment")}
}

func haResourceToInputs(info *HAResourceInfo) core.Inputs {
	inputs := core.Inputs{"sid": info.SID}
	if info.Group != "" { inputs["group"] = info.Group }
	if info.MaxRelocate != 0 { inputs["max_relocate"] = info.MaxRelocate }
	if info.MaxRestart != 0 { inputs["max_restart"] = info.MaxRestart }
	if info.State != "" { inputs["state"] = info.State }
	if info.Comment != "" { inputs["comment"] = info.Comment }
	return inputs
}

func (p *ProxmoxProvider) createHAResource(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info := haResourceFromInputs(args.Inputs)
	if err := p.api.CreateHAResource(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: create ha_resource %q: %w", info.SID, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"sid": info.SID}, ExternalID: info.SID, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readHAResourceRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadHAResource(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:ha_resource", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: haResourceToInputs(info), Outputs: core.Outputs{"sid": info.SID}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateHAResourceRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	info := haResourceFromInputs(desired.Inputs)
	if err := p.api.UpdateHAResource(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readHAResourceRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── ACME Account resource handlers ──────────────────────────────────────────

func (p *ProxmoxProvider) createACMEAccount(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	gs := func(k string) string { v, _ := args.Inputs[k].(string); return v }
	info := ACMEAccountInfo{Name: getStringInputFallback(args.Inputs, "name", args.Name), Contact: gs("contact"), Directory: gs("directory")}
	if err := p.api.CreateACMEAccount(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: create acme_account %q: %w", info.Name, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"name": info.Name}, ExternalID: info.Name, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readACMEAccountRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadACMEAccount(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	inputs := core.Inputs{"name": info.Name}
	if info.Contact != "" { inputs["contact"] = info.Contact }
	if info.Directory != "" { inputs["directory"] = info.Directory }
	return &core.ResourceState{ID: id, Type: "proxmox:acme_account", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: inputs, Outputs: core.Outputs{"name": info.Name}, UpdatedAt: time.Now()}, nil
}

// ── User resource handlers ──────────────────────────────────────────────────

func pveUserFromInputs(inputs core.Inputs) PVEUserInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	gb := func(k string) bool { v, _ := inputs[k].(bool); return v }
	gi := func(k string) int { v, _ := toInt(inputs[k]); return v }
	return PVEUserInfo{UserID: gs("userid"), Password: gs("password"), Email: gs("email"), FirstName: gs("firstname"), LastName: gs("lastname"), Groups: gs("groups"), Enable: gb("enable"), Expire: gi("expire"), Comment: gs("comment")}
}

func pveUserToInputs(info *PVEUserInfo) core.Inputs {
	inputs := core.Inputs{"userid": info.UserID, "enable": info.Enable}
	s := func(k, v string) { if v != "" { inputs[k] = v } }
	s("email", info.Email); s("firstname", info.FirstName); s("lastname", info.LastName); s("groups", info.Groups); s("comment", info.Comment)
	if info.Expire != 0 { inputs["expire"] = info.Expire }
	return inputs
}

func (p *ProxmoxProvider) createUser(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info := pveUserFromInputs(args.Inputs)
	if err := p.api.CreateUser(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: create user %q: %w", info.UserID, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"userid": info.UserID}, ExternalID: info.UserID, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readUserRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadUser(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:user", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: pveUserToInputs(info), Outputs: core.Outputs{"userid": info.UserID}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateUserRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	info := pveUserFromInputs(desired.Inputs)
	if err := p.api.UpdateUser(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readUserRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── Role resource handlers ──────────────────────────────────────────────────

func (p *ProxmoxProvider) createRole(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	gs := func(k string) string { v, _ := args.Inputs[k].(string); return v }
	info := PVERoleInfo{RoleID: getStringInputFallback(args.Inputs, "roleid", args.Name), Privs: gs("privs")}
	if err := p.api.CreateRole(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: create role %q: %w", info.RoleID, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"roleid": info.RoleID}, ExternalID: info.RoleID, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readRoleRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadRole(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	inputs := core.Inputs{"roleid": info.RoleID}
	if info.Privs != "" { inputs["privs"] = info.Privs }
	return &core.ResourceState{ID: id, Type: "proxmox:role", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: inputs, Outputs: core.Outputs{"roleid": info.RoleID}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateRoleRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	gs := func(k string) string { v, _ := desired.Inputs[k].(string); return v }
	info := PVERoleInfo{RoleID: current.ExternalID, Privs: gs("privs")}
	if err := p.api.UpdateRole(ctx, current.ExternalID, info); err != nil { return nil, err }
	return p.readRoleRes(ctx, proxmoxResourceID(desired), current.ExternalID)
}

// ── ACL resource handlers ───────────────────────────────────────────────────

func (p *ProxmoxProvider) createACL(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	gs := func(k string) string { v, _ := args.Inputs[k].(string); return v }
	gb := func(k string) bool { v, _ := args.Inputs[k].(bool); return v }
	info := PVEACLInfo{Path: gs("path"), Roles: gs("roles"), Users: gs("users"), Groups: gs("groups"), Propagate: gb("propagate")}
	if err := p.api.SetACL(ctx, info); err != nil { return nil, fmt.Errorf("proxmox: set acl on %q: %w", info.Path, err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{"path": info.Path}, ExternalID: info.Path, ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readACLRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadACL(ctx, externalID)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	inputs := core.Inputs{"path": info.Path}
	if info.Roles != "" { inputs["roles"] = info.Roles }
	if info.Users != "" { inputs["users"] = info.Users }
	if info.Groups != "" { inputs["groups"] = info.Groups }
	if info.Propagate { inputs["propagate"] = true }
	return &core.ResourceState{ID: id, Type: "proxmox:acl", ExternalID: externalID, ProviderID: "proxmox", Status: core.StatusRunning, Inputs: inputs, Outputs: core.Outputs{"path": info.Path}, UpdatedAt: time.Now()}, nil
}

// ── Cluster Options resource handlers ───────────────────────────────────────

func clusterOptsFromInputs(inputs core.Inputs) ClusterOptionsInfo {
	gs := func(k string) string { v, _ := inputs[k].(string); return v }
	return ClusterOptionsInfo{Keyboard: gs("keyboard"), Language: gs("language"), EmailFrom: gs("email_from"), MigrationType: gs("migration_type"), MigrationNetwork: gs("migration_network")}
}

func clusterOptsToInputs(info *ClusterOptionsInfo) core.Inputs {
	inputs := core.Inputs{}
	s := func(k, v string) { if v != "" { inputs[k] = v } }
	s("keyboard", info.Keyboard); s("language", info.Language); s("email_from", info.EmailFrom); s("migration_type", info.MigrationType); s("migration_network", info.MigrationNetwork)
	return inputs
}

func (p *ProxmoxProvider) createClusterOptions(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := clusterOptsFromInputs(args.Inputs)
	if err := p.api.UpdateClusterOptions(ctx, opts); err != nil { return nil, fmt.Errorf("proxmox: set cluster options: %w", err) }
	return &core.ResourceState{ID: proxmoxResourceID(args), Type: args.Type, Name: args.Name, Status: core.StatusRunning, Inputs: args.Inputs, Outputs: core.Outputs{}, ExternalID: "cluster", ProviderID: "proxmox", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) readClusterOptionsRes(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.ReadClusterOptions(ctx)
	if err != nil { return nil, err }
	if info == nil { return nil, nil }
	return &core.ResourceState{ID: id, Type: "proxmox:cluster_options", ExternalID: "cluster", ProviderID: "proxmox", Status: core.StatusRunning, Inputs: clusterOptsToInputs(info), Outputs: core.Outputs{}, UpdatedAt: time.Now()}, nil
}

func (p *ProxmoxProvider) updateClusterOptionsRes(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	opts := clusterOptsFromInputs(desired.Inputs)
	if err := p.api.UpdateClusterOptions(ctx, opts); err != nil { return nil, err }
	return p.readClusterOptionsRes(ctx, proxmoxResourceID(desired), "cluster")
}
