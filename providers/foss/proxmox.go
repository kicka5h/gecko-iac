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

	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:storage",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"storage": info.Name,
			"type":    info.Type,
			"content": info.Content,
		},
		Outputs:   core.Outputs{"storage": info.Name},
		UpdatedAt: time.Now(),
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

	if v, ok := inputs["type"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "type", Value: v})
	}
	if v, ok := inputs["content"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "content", Value: v})
	}
	if v, ok := inputs["path"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "path", Value: v})
	}
	if v, ok := inputs["server"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "server", Value: v})
	}
	if v, ok := inputs["export"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "export", Value: v})
	}
	if v, ok := inputs["nodes"].(string); ok && v != "" {
		opts = append(opts, proxmox.ClusterStorageOptions{Name: "nodes", Value: v})
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
	return &core.ResourceState{
		ID:         id,
		Type:       "proxmox:network",
		ExternalID: externalID,
		ProviderID: "proxmox",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"iface":        cfg.Iface,
			"type":         cfg.Type,
			"cidr":         cfg.CIDR,
			"gateway":      cfg.Gateway,
			"bridge_ports": cfg.BridgePorts,
			"comments":     cfg.Comments,
			"autostart":    cfg.Autostart,
		},
		Outputs:   core.Outputs{"iface": cfg.Iface, "node": nodeName},
		UpdatedAt: time.Now(),
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
	return NetworkConfig{
		Iface:       getStringInputFallback(inputs, "iface", name),
		Type:        getStringInputFallback(inputs, "type", "bridge"),
		CIDR:        func() string { v, _ := inputs["cidr"].(string); return v }(),
		Gateway:     func() string { v, _ := inputs["gateway"].(string); return v }(),
		BridgePorts: func() string { v, _ := inputs["bridge_ports"].(string); return v }(),
		Comments:    func() string { v, _ := inputs["comments"].(string); return v }(),
		Autostart:   func() bool { v, _ := inputs["autostart"].(bool); return v }(),
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
