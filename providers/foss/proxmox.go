package k8s

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

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
}

// ── Value types returned by the interface (no library type leakage) ────────────

// VMInfo holds the fields gecko cares about for a Proxmox VM.
type VMInfo struct {
	VMID   int
	Name   string
	Status string
	Memory int
	Cores  int
	Net0   string
}

// LXCInfo holds the fields gecko cares about for a Proxmox LXC container.
type LXCInfo struct {
	VMID     int
	Hostname string
	Status   string
	Memory   int
	Cores    int
}

// StorageInfo holds the fields gecko cares about for a Proxmox cluster storage.
type StorageInfo struct {
	Name    string
	Type    string
	Content string
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
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("memory", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cores", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("net0", current.Inputs, desired.Inputs)...)
	case "proxmox:lxc":
		changes = append(changes, compareField("hostname", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("memory", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cores", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("net0", current.Inputs, desired.Inputs)...)
	case "proxmox:storage":
		changes = append(changes, compareField("content", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("nodes", current.Inputs, desired.Inputs)...)
	case "proxmox:network":
		changes = append(changes, compareField("cidr", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("gateway", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("bridge_ports", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("autostart", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("comments", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}
