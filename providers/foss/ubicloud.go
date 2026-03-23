package foss

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// ubicloudAPI is the interface over all Ubicloud operations used by this provider.
// The real implementation delegates to the Ubicloud REST API; a mock can be injected for tests.
type ubicloudAPI interface {
	CreateVM(ctx context.Context, name string, opts map[string]interface{}) (*UbiVMInfo, error)
	GetVM(ctx context.Context, id string) (*UbiVMInfo, error)
	UpdateVM(ctx context.Context, id string, opts map[string]interface{}) (*UbiVMInfo, error)
	DeleteVM(ctx context.Context, id string) error

	CreateFirewall(ctx context.Context, name string, opts map[string]interface{}) (*UbiFirewallInfo, error)
	GetFirewall(ctx context.Context, id string) (*UbiFirewallInfo, error)
	UpdateFirewall(ctx context.Context, id string, opts map[string]interface{}) (*UbiFirewallInfo, error)
	DeleteFirewall(ctx context.Context, id string) error

	CreateSubnet(ctx context.Context, name string, opts map[string]interface{}) (*UbiSubnetInfo, error)
	GetSubnet(ctx context.Context, id string) (*UbiSubnetInfo, error)
	DeleteSubnet(ctx context.Context, id string) error
}

// ── Value types returned by the interface (no library type leakage) ────────────

// UbiVMInfo holds the fields gecko cares about for a Ubicloud VM.
type UbiVMInfo struct {
	ID       string
	Name     string
	Status   string
	Size     string
	Location string
}

// UbiFirewallInfo holds the fields gecko cares about for a Ubicloud firewall.
type UbiFirewallInfo struct {
	ID          string
	Name        string
	Description string
}

// UbiSubnetInfo holds the fields gecko cares about for a Ubicloud subnet.
type UbiSubnetInfo struct {
	ID       string
	Name     string
	CIDR     string
	Location string
}

// ── UbicloudProvider ──────────────────────────────────────────────────────────

// UbicloudProvider manages Ubicloud resources via the Ubicloud REST API.
type UbicloudProvider struct {
	apiToken  string
	projectID string
	api       ubicloudAPI
}

// NewUbicloudProvider creates a new Ubicloud provider with optional config.
func NewUbicloudProvider(config map[string]interface{}) *UbicloudProvider {
	p := &UbicloudProvider{}
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
		if v, ok := config["project_id"].(string); ok && v != "" {
			p.projectID = v
		}
	}
	return p
}

func (p *UbicloudProvider) Name() string    { return "ubicloud" }
func (p *UbicloudProvider) Version() string { return "0.1.0" }
func (p *UbicloudProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"ubicloud:vm",
		"ubicloud:firewall",
		"ubicloud:subnet",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *UbicloudProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
		if v, ok := config["project_id"].(string); ok && v != "" {
			p.projectID = v
		}
	}
	return nil
}

func (p *UbicloudProvider) connect() error {
	if p.api != nil {
		return nil
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	p.api = &realUbicloudAPI{
		client:    httpClient,
		token:     p.apiToken,
		projectID: p.projectID,
	}
	return nil
}

func (p *UbicloudProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func ubicloudResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *UbicloudProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "ubicloud:vm":
		return p.createVM(ctx, args)
	case "ubicloud:firewall":
		return p.createFirewall(ctx, args)
	case "ubicloud:subnet":
		return p.createSubnet(ctx, args)
	default:
		return nil, fmt.Errorf("ubicloud: unsupported resource type %q", args.Type)
	}
}

func (p *UbicloudProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("ubicloud: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "ubicloud:vm":
		return p.readVM(ctx, id, externalID)
	case "ubicloud:firewall":
		return p.readFirewall(ctx, id, externalID)
	case "ubicloud:subnet":
		return p.readSubnet(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("ubicloud: unsupported resource type %q", rType)
	}
}

func (p *UbicloudProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "ubicloud:vm":
		return p.updateVM(ctx, current, desired)
	case "ubicloud:firewall":
		return p.updateFirewall(ctx, current, desired)
	case "ubicloud:subnet":
		return p.updateSubnet(ctx, current, desired)
	default:
		return nil, fmt.Errorf("ubicloud: unsupported resource type %q", desired.Type)
	}
}

func (p *UbicloudProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "ubicloud:vm":
		return p.deleteVM(ctx, state)
	case "ubicloud:firewall":
		return p.deleteFirewall(ctx, state)
	case "ubicloud:subnet":
		return p.deleteSubnet(ctx, state)
	default:
		return fmt.Errorf("ubicloud: unsupported resource type %q", state.Type)
	}
}

func (p *UbicloudProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *UbicloudProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := ubicloudResourceID(desired)

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
	case "ubicloud:vm":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("size", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("location", current.Inputs, desired.Inputs)...)
	case "ubicloud:firewall":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("description", current.Inputs, desired.Inputs)...)
	case "ubicloud:subnet":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cidr", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("location", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── VM CRUD ───────────────────────────────────────────────────────────────────

func (p *UbicloudProvider) createVM(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info, err := p.api.CreateVM(ctx, args.Name, args.Inputs)
	if err != nil {
		return nil, fmt.Errorf("ubicloud: create vm %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         ubicloudResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "ubicloud",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *UbicloudProvider) readVM(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetVM(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	status := core.StatusRunning
	if info.Status != "running" {
		status = core.StatusPending
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "ubicloud:vm",
		ExternalID: externalID,
		ProviderID: "ubicloud",
		Status:     status,
		Inputs: core.Inputs{
			"name":     info.Name,
			"size":     info.Size,
			"location": info.Location,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *UbicloudProvider) updateVM(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	_, err := p.api.UpdateVM(ctx, current.ExternalID, desired.Inputs)
	if err != nil {
		return nil, fmt.Errorf("ubicloud: update vm %q: %w", current.ExternalID, err)
	}

	return p.readVM(ctx, ubicloudResourceID(desired), current.ExternalID)
}

func (p *UbicloudProvider) deleteVM(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteVM(ctx, state.ExternalID)
}

// ── Firewall CRUD ─────────────────────────────────────────────────────────────

func (p *UbicloudProvider) createFirewall(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info, err := p.api.CreateFirewall(ctx, args.Name, args.Inputs)
	if err != nil {
		return nil, fmt.Errorf("ubicloud: create firewall %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         ubicloudResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "ubicloud",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *UbicloudProvider) readFirewall(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetFirewall(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "ubicloud:firewall",
		ExternalID: externalID,
		ProviderID: "ubicloud",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":        info.Name,
			"description": info.Description,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *UbicloudProvider) updateFirewall(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	_, err := p.api.UpdateFirewall(ctx, current.ExternalID, desired.Inputs)
	if err != nil {
		return nil, fmt.Errorf("ubicloud: update firewall %q: %w", current.ExternalID, err)
	}

	return p.readFirewall(ctx, ubicloudResourceID(desired), current.ExternalID)
}

func (p *UbicloudProvider) deleteFirewall(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteFirewall(ctx, state.ExternalID)
}

// ── Subnet CRUD ───────────────────────────────────────────────────────────────

func (p *UbicloudProvider) createSubnet(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	info, err := p.api.CreateSubnet(ctx, args.Name, args.Inputs)
	if err != nil {
		return nil, fmt.Errorf("ubicloud: create subnet %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         ubicloudResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID},
		ExternalID: info.ID,
		ProviderID: "ubicloud",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *UbicloudProvider) readSubnet(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetSubnet(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "ubicloud:subnet",
		ExternalID: externalID,
		ProviderID: "ubicloud",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":     info.Name,
			"cidr":     info.CIDR,
			"location": info.Location,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *UbicloudProvider) updateSubnet(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// Subnets typically require delete+recreate; delegate to read after the API handles it.
	return nil, fmt.Errorf("ubicloud: subnets do not support in-place updates")
}

func (p *UbicloudProvider) deleteSubnet(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteSubnet(ctx, state.ExternalID)
}

// ── Real API implementation (stub) ────────────────────────────────────────────

type realUbicloudAPI struct {
	client    *http.Client
	token     string
	projectID string
}

func (r *realUbicloudAPI) CreateVM(ctx context.Context, name string, opts map[string]interface{}) (*UbiVMInfo, error) {
	return nil, fmt.Errorf("ubicloud: CreateVM not yet implemented")
}

func (r *realUbicloudAPI) GetVM(ctx context.Context, id string) (*UbiVMInfo, error) {
	return nil, fmt.Errorf("ubicloud: GetVM not yet implemented")
}

func (r *realUbicloudAPI) UpdateVM(ctx context.Context, id string, opts map[string]interface{}) (*UbiVMInfo, error) {
	return nil, fmt.Errorf("ubicloud: UpdateVM not yet implemented")
}

func (r *realUbicloudAPI) DeleteVM(ctx context.Context, id string) error {
	return fmt.Errorf("ubicloud: DeleteVM not yet implemented")
}

func (r *realUbicloudAPI) CreateFirewall(ctx context.Context, name string, opts map[string]interface{}) (*UbiFirewallInfo, error) {
	return nil, fmt.Errorf("ubicloud: CreateFirewall not yet implemented")
}

func (r *realUbicloudAPI) GetFirewall(ctx context.Context, id string) (*UbiFirewallInfo, error) {
	return nil, fmt.Errorf("ubicloud: GetFirewall not yet implemented")
}

func (r *realUbicloudAPI) UpdateFirewall(ctx context.Context, id string, opts map[string]interface{}) (*UbiFirewallInfo, error) {
	return nil, fmt.Errorf("ubicloud: UpdateFirewall not yet implemented")
}

func (r *realUbicloudAPI) DeleteFirewall(ctx context.Context, id string) error {
	return fmt.Errorf("ubicloud: DeleteFirewall not yet implemented")
}

func (r *realUbicloudAPI) CreateSubnet(ctx context.Context, name string, opts map[string]interface{}) (*UbiSubnetInfo, error) {
	return nil, fmt.Errorf("ubicloud: CreateSubnet not yet implemented")
}

func (r *realUbicloudAPI) GetSubnet(ctx context.Context, id string) (*UbiSubnetInfo, error) {
	return nil, fmt.Errorf("ubicloud: GetSubnet not yet implemented")
}

func (r *realUbicloudAPI) DeleteSubnet(ctx context.Context, id string) error {
	return fmt.Errorf("ubicloud: DeleteSubnet not yet implemented")
}
