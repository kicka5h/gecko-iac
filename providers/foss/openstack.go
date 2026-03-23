package foss

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// openstackAPI is the interface over all OpenStack operations used by this provider.
// The real implementation delegates to the OpenStack SDK; a mock can be injected for tests.
type openstackAPI interface {
	CreateServer(ctx context.Context, opts map[string]interface{}) (*ServerInfo, error)
	GetServer(ctx context.Context, id string) (*ServerInfo, error)
	UpdateServer(ctx context.Context, id string, opts map[string]interface{}) (*ServerInfo, error)
	DeleteServer(ctx context.Context, id string) error

	CreateNetwork(ctx context.Context, opts map[string]interface{}) (*OSNetworkInfo, error)
	GetNetwork(ctx context.Context, id string) (*OSNetworkInfo, error)
	DeleteNetwork(ctx context.Context, id string) error

	CreateSubnet(ctx context.Context, opts map[string]interface{}) (*SubnetInfo, error)
	GetSubnet(ctx context.Context, id string) (*SubnetInfo, error)
	DeleteSubnet(ctx context.Context, id string) error

	CreateSecurityGroup(ctx context.Context, opts map[string]interface{}) (*SecurityGroupInfo, error)
	GetSecurityGroup(ctx context.Context, id string) (*SecurityGroupInfo, error)
	DeleteSecurityGroup(ctx context.Context, id string) error

	CreateVolume(ctx context.Context, opts map[string]interface{}) (*OSVolumeInfo, error)
	GetVolume(ctx context.Context, id string) (*OSVolumeInfo, error)
	DeleteVolume(ctx context.Context, id string) error
}

// ── Value types returned by the interface (no library type leakage) ────────────

// ServerInfo holds the fields gecko cares about for an OpenStack server.
type ServerInfo struct {
	ID        string
	Name      string
	Status    string
	FlavorID  string
	ImageID   string
	NetworkID string
}

// OSNetworkInfo holds the fields gecko cares about for an OpenStack network.
type OSNetworkInfo struct {
	ID     string
	Name   string
	Status string
}

// SubnetInfo holds the fields gecko cares about for an OpenStack subnet.
type SubnetInfo struct {
	ID        string
	Name      string
	CIDR      string
	NetworkID string
}

// SecurityGroupInfo holds the fields gecko cares about for an OpenStack security group.
type SecurityGroupInfo struct {
	ID          string
	Name        string
	Description string
}

// OSVolumeInfo holds the fields gecko cares about for an OpenStack volume.
type OSVolumeInfo struct {
	ID     string
	Name   string
	Size   int
	Status string
}

// ── OpenStackProvider ─────────────────────────────────────────────────────────

// OpenStackProvider manages OpenStack resources via the OpenStack APIs.
type OpenStackProvider struct {
	authURL    string
	username   string
	password   string
	tenantName string
	region     string
	domainName string
	api        openstackAPI
}

// NewOpenStackProvider creates a new OpenStack provider with optional config.
func NewOpenStackProvider(config map[string]interface{}) *OpenStackProvider {
	p := &OpenStackProvider{}
	if config != nil {
		if v, ok := config["auth_url"].(string); ok && v != "" {
			p.authURL = v
		}
		if v, ok := config["username"].(string); ok && v != "" {
			p.username = v
		}
		if v, ok := config["password"].(string); ok && v != "" {
			p.password = v
		}
		if v, ok := config["tenant_name"].(string); ok && v != "" {
			p.tenantName = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
		if v, ok := config["domain_name"].(string); ok && v != "" {
			p.domainName = v
		}
	}
	return p
}

func (p *OpenStackProvider) Name() string    { return "openstack" }
func (p *OpenStackProvider) Version() string { return "0.1.0" }
func (p *OpenStackProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"openstack:instance",
		"openstack:network",
		"openstack:subnet",
		"openstack:security_group",
		"openstack:volume",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *OpenStackProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["auth_url"].(string); ok && v != "" {
			p.authURL = v
		}
		if v, ok := config["username"].(string); ok && v != "" {
			p.username = v
		}
		if v, ok := config["password"].(string); ok && v != "" {
			p.password = v
		}
		if v, ok := config["tenant_name"].(string); ok && v != "" {
			p.tenantName = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
		if v, ok := config["domain_name"].(string); ok && v != "" {
			p.domainName = v
		}
	}
	return nil
}

func (p *OpenStackProvider) connect() error {
	if p.api != nil {
		return nil
	}
	p.api = newRealOpenStackAPI(p.authURL, p.username, p.password, p.tenantName, p.region, p.domainName)
	return nil
}

func (p *OpenStackProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func openstackResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *OpenStackProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "openstack:instance":
		return p.createInstance(ctx, args)
	case "openstack:network":
		return p.createOSNetwork(ctx, args)
	case "openstack:subnet":
		return p.createSubnet(ctx, args)
	case "openstack:security_group":
		return p.createSecurityGroup(ctx, args)
	case "openstack:volume":
		return p.createVolume(ctx, args)
	default:
		return nil, fmt.Errorf("openstack: unsupported resource type %q", args.Type)
	}
}

func (p *OpenStackProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("openstack: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "openstack:instance":
		return p.readInstance(ctx, id, externalID)
	case "openstack:network":
		return p.readOSNetwork(ctx, id, externalID)
	case "openstack:subnet":
		return p.readSubnet(ctx, id, externalID)
	case "openstack:security_group":
		return p.readSecurityGroup(ctx, id, externalID)
	case "openstack:volume":
		return p.readVolume(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("openstack: unsupported resource type %q", rType)
	}
}

func (p *OpenStackProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "openstack:instance":
		return p.updateInstance(ctx, current, desired)
	case "openstack:network":
		return p.updateOSNetwork(ctx, current, desired)
	case "openstack:subnet":
		return p.updateSubnet(ctx, current, desired)
	case "openstack:security_group":
		return p.updateSecurityGroup(ctx, current, desired)
	case "openstack:volume":
		return p.updateVolume(ctx, current, desired)
	default:
		return nil, fmt.Errorf("openstack: unsupported resource type %q", desired.Type)
	}
}

func (p *OpenStackProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "openstack:instance":
		return p.deleteInstance(ctx, state)
	case "openstack:network":
		return p.deleteOSNetwork(ctx, state)
	case "openstack:subnet":
		return p.deleteSubnet(ctx, state)
	case "openstack:security_group":
		return p.deleteSecurityGroup(ctx, state)
	case "openstack:volume":
		return p.deleteVolume(ctx, state)
	default:
		return fmt.Errorf("openstack: unsupported resource type %q", state.Type)
	}
}

func (p *OpenStackProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *OpenStackProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := openstackResourceID(desired)

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
	case "openstack:instance":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("flavor_id", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("image_id", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("network_id", current.Inputs, desired.Inputs)...)
	case "openstack:network":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
	case "openstack:subnet":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cidr", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("network_id", current.Inputs, desired.Inputs)...)
	case "openstack:security_group":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("description", current.Inputs, desired.Inputs)...)
	case "openstack:volume":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("size", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── Instance (openstack:instance) ─────────────────────────────────────────────

func (p *OpenStackProvider) createInstance(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(args.Inputs, "name", args.Name),
	}
	if v, ok := args.Inputs["flavor_id"].(string); ok && v != "" {
		opts["flavor_id"] = v
	}
	if v, ok := args.Inputs["image_id"].(string); ok && v != "" {
		opts["image_id"] = v
	}
	if v, ok := args.Inputs["network_id"].(string); ok && v != "" {
		opts["network_id"] = v
	}

	info, err := p.api.CreateServer(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("openstack: create instance %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         openstackResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "openstack",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenStackProvider) readInstance(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetServer(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	status := core.StatusRunning
	if info.Status != "ACTIVE" {
		status = core.StatusPending
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "openstack:instance",
		ExternalID: externalID,
		ProviderID: "openstack",
		Status:     status,
		Inputs: core.Inputs{
			"name":       info.Name,
			"flavor_id":  info.FlavorID,
			"image_id":   info.ImageID,
			"network_id": info.NetworkID,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenStackProvider) updateInstance(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(desired.Inputs, "name", desired.Name),
	}
	if v, ok := desired.Inputs["flavor_id"].(string); ok && v != "" {
		opts["flavor_id"] = v
	}
	if v, ok := desired.Inputs["image_id"].(string); ok && v != "" {
		opts["image_id"] = v
	}
	if v, ok := desired.Inputs["network_id"].(string); ok && v != "" {
		opts["network_id"] = v
	}

	if _, err := p.api.UpdateServer(ctx, current.ExternalID, opts); err != nil {
		return nil, fmt.Errorf("openstack: update instance %q: %w", desired.Name, err)
	}

	return p.readInstance(ctx, openstackResourceID(desired), current.ExternalID)
}

func (p *OpenStackProvider) deleteInstance(ctx context.Context, state *core.ResourceState) error {
	if err := p.api.DeleteServer(ctx, state.ExternalID); err != nil {
		return fmt.Errorf("openstack: delete instance %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── Network (openstack:network) ───────────────────────────────────────────────

func (p *OpenStackProvider) createOSNetwork(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(args.Inputs, "name", args.Name),
	}

	info, err := p.api.CreateNetwork(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("openstack: create network %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         openstackResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "openstack",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenStackProvider) readOSNetwork(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetNetwork(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "openstack:network",
		ExternalID: externalID,
		ProviderID: "openstack",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name": info.Name,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenStackProvider) updateOSNetwork(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// OpenStack networks have limited update support; re-read after best-effort.
	return p.readOSNetwork(ctx, openstackResourceID(desired), current.ExternalID)
}

func (p *OpenStackProvider) deleteOSNetwork(ctx context.Context, state *core.ResourceState) error {
	if err := p.api.DeleteNetwork(ctx, state.ExternalID); err != nil {
		return fmt.Errorf("openstack: delete network %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── Subnet (openstack:subnet) ─────────────────────────────────────────────────

func (p *OpenStackProvider) createSubnet(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(args.Inputs, "name", args.Name),
	}
	if v, ok := args.Inputs["cidr"].(string); ok && v != "" {
		opts["cidr"] = v
	}
	if v, ok := args.Inputs["network_id"].(string); ok && v != "" {
		opts["network_id"] = v
	}

	info, err := p.api.CreateSubnet(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("openstack: create subnet %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         openstackResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "openstack",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenStackProvider) readSubnet(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetSubnet(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "openstack:subnet",
		ExternalID: externalID,
		ProviderID: "openstack",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":       info.Name,
			"cidr":       info.CIDR,
			"network_id": info.NetworkID,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenStackProvider) updateSubnet(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return p.readSubnet(ctx, openstackResourceID(desired), current.ExternalID)
}

func (p *OpenStackProvider) deleteSubnet(ctx context.Context, state *core.ResourceState) error {
	if err := p.api.DeleteSubnet(ctx, state.ExternalID); err != nil {
		return fmt.Errorf("openstack: delete subnet %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── Security Group (openstack:security_group) ─────────────────────────────────

func (p *OpenStackProvider) createSecurityGroup(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(args.Inputs, "name", args.Name),
	}
	if v, ok := args.Inputs["description"].(string); ok && v != "" {
		opts["description"] = v
	}

	info, err := p.api.CreateSecurityGroup(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("openstack: create security group %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         openstackResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "openstack",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenStackProvider) readSecurityGroup(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetSecurityGroup(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "openstack:security_group",
		ExternalID: externalID,
		ProviderID: "openstack",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":        info.Name,
			"description": info.Description,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenStackProvider) updateSecurityGroup(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return p.readSecurityGroup(ctx, openstackResourceID(desired), current.ExternalID)
}

func (p *OpenStackProvider) deleteSecurityGroup(ctx context.Context, state *core.ResourceState) error {
	if err := p.api.DeleteSecurityGroup(ctx, state.ExternalID); err != nil {
		return fmt.Errorf("openstack: delete security group %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── Volume (openstack:volume) ─────────────────────────────────────────────────

func (p *OpenStackProvider) createVolume(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	opts := map[string]interface{}{
		"name": getStringInputFallback(args.Inputs, "name", args.Name),
	}
	if v, ok := toInt(args.Inputs["size"]); ok {
		opts["size"] = v
	}

	info, err := p.api.CreateVolume(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("openstack: create volume %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         openstackResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "openstack",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenStackProvider) readVolume(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetVolume(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "openstack:volume",
		ExternalID: externalID,
		ProviderID: "openstack",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name": info.Name,
			"size": info.Size,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenStackProvider) updateVolume(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return p.readVolume(ctx, openstackResourceID(desired), current.ExternalID)
}

func (p *OpenStackProvider) deleteVolume(ctx context.Context, state *core.ResourceState) error {
	if err := p.api.DeleteVolume(ctx, state.ExternalID); err != nil {
		return fmt.Errorf("openstack: delete volume %q: %w", state.ExternalID, err)
	}
	return nil
}
