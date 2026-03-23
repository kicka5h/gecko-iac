package foss

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// hostingerAPI is the interface over all Hostinger operations used by this provider.
// The real implementation delegates to the Hostinger REST API; a mock can be injected for tests.
type hostingerAPI interface {
	CreateVPS(ctx context.Context, hostname, plan, region string) (*VPSInfo, error)
	GetVPS(ctx context.Context, id string) (*VPSInfo, error)
	UpdateVPS(ctx context.Context, id string, hostname, plan, region string) (*VPSInfo, error)
	DeleteVPS(ctx context.Context, id string) error

	CreateDomain(ctx context.Context, name string) (*DomainInfo, error)
	GetDomain(ctx context.Context, id string) (*DomainInfo, error)
	DeleteDomain(ctx context.Context, id string) error
}

// ── Value types returned by the interface (no library type leakage) ────────────

// VPSInfo holds the fields gecko cares about for a Hostinger VPS.
type VPSInfo struct {
	ID       string
	Hostname string
	Status   string
	Plan     string
	Region   string
	IPv4     string
}

// DomainInfo holds the fields gecko cares about for a Hostinger domain.
type DomainInfo struct {
	ID     string
	Name   string
	Status string
}

// ── HostingerProvider ─────────────────────────────────────────────────────────

// HostingerProvider manages Hostinger resources via the Hostinger REST API.
type HostingerProvider struct {
	apiToken string
	api      hostingerAPI
}

// NewHostingerProvider creates a new Hostinger provider with optional config.
func NewHostingerProvider(config map[string]interface{}) *HostingerProvider {
	p := &HostingerProvider{}
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
	}
	return p
}

func (p *HostingerProvider) Name() string    { return "hostinger" }
func (p *HostingerProvider) Version() string { return "0.1.0" }
func (p *HostingerProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"hostinger:vps",
		"hostinger:domain",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *HostingerProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
	}
	return nil
}

func (p *HostingerProvider) connect() error {
	if p.api != nil {
		return nil
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	p.api = &realHostingerAPI{
		client: httpClient,
		token:  p.apiToken,
	}
	return nil
}

func (p *HostingerProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func hostingerResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *HostingerProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "hostinger:vps":
		return p.createVPS(ctx, args)
	case "hostinger:domain":
		return p.createDomain(ctx, args)
	default:
		return nil, fmt.Errorf("hostinger: unsupported resource type %q", args.Type)
	}
}

func (p *HostingerProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("hostinger: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "hostinger:vps":
		return p.readVPS(ctx, id, externalID)
	case "hostinger:domain":
		return p.readDomain(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("hostinger: unsupported resource type %q", rType)
	}
}

func (p *HostingerProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "hostinger:vps":
		return p.updateVPS(ctx, current, desired)
	case "hostinger:domain":
		return p.updateDomain(ctx, current, desired)
	default:
		return nil, fmt.Errorf("hostinger: unsupported resource type %q", desired.Type)
	}
}

func (p *HostingerProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "hostinger:vps":
		return p.deleteVPS(ctx, state)
	case "hostinger:domain":
		return p.deleteDomain(ctx, state)
	default:
		return fmt.Errorf("hostinger: unsupported resource type %q", state.Type)
	}
}

func (p *HostingerProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *HostingerProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := hostingerResourceID(desired)

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
	case "hostinger:vps":
		changes = append(changes, compareField("hostname", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("plan", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("region", current.Inputs, desired.Inputs)...)
	case "hostinger:domain":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── VPS CRUD ──────────────────────────────────────────────────────────────────

func (p *HostingerProvider) createVPS(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	hostname := getStringInputFallback(args.Inputs, "hostname", args.Name)
	plan := getStringInputFallback(args.Inputs, "plan", "")
	region := getStringInputFallback(args.Inputs, "region", "")

	info, err := p.api.CreateVPS(ctx, hostname, plan, region)
	if err != nil {
		return nil, fmt.Errorf("hostinger: create vps %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         hostingerResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID, "ipv4": info.IPv4},
		ExternalID: info.ID,
		ProviderID: "hostinger",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *HostingerProvider) readVPS(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetVPS(ctx, externalID)
	if err != nil {
		return nil, fmt.Errorf("hostinger: read vps %q: %w", externalID, err)
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
		Type:       "hostinger:vps",
		ExternalID: externalID,
		ProviderID: "hostinger",
		Status:     status,
		Inputs: core.Inputs{
			"hostname": info.Hostname,
			"plan":     info.Plan,
			"region":   info.Region,
		},
		Outputs:   core.Outputs{"id": info.ID, "ipv4": info.IPv4},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *HostingerProvider) updateVPS(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	hostname := getStringInputFallback(desired.Inputs, "hostname", desired.Name)
	plan := getStringInputFallback(desired.Inputs, "plan", "")
	region := getStringInputFallback(desired.Inputs, "region", "")

	_, err := p.api.UpdateVPS(ctx, current.ExternalID, hostname, plan, region)
	if err != nil {
		return nil, fmt.Errorf("hostinger: update vps %q: %w", current.ExternalID, err)
	}

	return p.readVPS(ctx, hostingerResourceID(desired), current.ExternalID)
}

func (p *HostingerProvider) deleteVPS(ctx context.Context, state *core.ResourceState) error {
	info, err := p.api.GetVPS(ctx, state.ExternalID)
	if err != nil {
		return fmt.Errorf("hostinger: delete vps %q: %w", state.ExternalID, err)
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteVPS(ctx, state.ExternalID)
}

// ── Domain CRUD ───────────────────────────────────────────────────────────────

func (p *HostingerProvider) createDomain(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	name := getStringInputFallback(args.Inputs, "name", args.Name)

	info, err := p.api.CreateDomain(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("hostinger: create domain %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         hostingerResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": info.ID, "name": info.Name},
		ExternalID: info.ID,
		ProviderID: "hostinger",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *HostingerProvider) readDomain(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetDomain(ctx, externalID)
	if err != nil {
		return nil, fmt.Errorf("hostinger: read domain %q: %w", externalID, err)
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "hostinger:domain",
		ExternalID: externalID,
		ProviderID: "hostinger",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name": info.Name,
		},
		Outputs:   core.Outputs{"id": info.ID, "name": info.Name},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *HostingerProvider) updateDomain(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// Domains cannot be updated in-place; they must be deleted and recreated.
	return nil, fmt.Errorf("hostinger: domain resources do not support in-place updates")
}

func (p *HostingerProvider) deleteDomain(ctx context.Context, state *core.ResourceState) error {
	info, err := p.api.GetDomain(ctx, state.ExternalID)
	if err != nil {
		return fmt.Errorf("hostinger: delete domain %q: %w", state.ExternalID, err)
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteDomain(ctx, state.ExternalID)
}

// ── Real API implementation (stub) ────────────────────────────────────────────

type realHostingerAPI struct {
	client *http.Client
	token  string
}

func (a *realHostingerAPI) CreateVPS(ctx context.Context, hostname, plan, region string) (*VPSInfo, error) {
	return nil, fmt.Errorf("hostinger: CreateVPS not yet implemented")
}

func (a *realHostingerAPI) GetVPS(ctx context.Context, id string) (*VPSInfo, error) {
	return nil, fmt.Errorf("hostinger: GetVPS not yet implemented")
}

func (a *realHostingerAPI) UpdateVPS(ctx context.Context, id string, hostname, plan, region string) (*VPSInfo, error) {
	return nil, fmt.Errorf("hostinger: UpdateVPS not yet implemented")
}

func (a *realHostingerAPI) DeleteVPS(ctx context.Context, id string) error {
	return fmt.Errorf("hostinger: DeleteVPS not yet implemented")
}

func (a *realHostingerAPI) CreateDomain(ctx context.Context, name string) (*DomainInfo, error) {
	return nil, fmt.Errorf("hostinger: CreateDomain not yet implemented")
}

func (a *realHostingerAPI) GetDomain(ctx context.Context, id string) (*DomainInfo, error) {
	return nil, fmt.Errorf("hostinger: GetDomain not yet implemented")
}

func (a *realHostingerAPI) DeleteDomain(ctx context.Context, id string) error {
	return fmt.Errorf("hostinger: DeleteDomain not yet implemented")
}
