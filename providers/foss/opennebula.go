package foss

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// opennebulaAPI is the interface over all OpenNebula operations used by this provider.
// The real implementation delegates to the OpenNebula XML-RPC API; a mock can be injected for tests.
type opennebulaAPI interface {
	CreateVM(ctx context.Context, template string, name string) (int, error)
	GetVM(ctx context.Context, id int) (*OneVMInfo, error)
	UpdateVM(ctx context.Context, id int, template string) error
	DeleteVM(ctx context.Context, id int) error

	CreateVNet(ctx context.Context, template string) (int, error)
	GetVNet(ctx context.Context, id int) (*OneVNetInfo, error)
	DeleteVNet(ctx context.Context, id int) error

	CreateImage(ctx context.Context, template string, datastoreID int) (int, error)
	GetImage(ctx context.Context, id int) (*OneImageInfo, error)
	DeleteImage(ctx context.Context, id int) error

	CreateTemplate(ctx context.Context, template string) (int, error)
	GetTemplate(ctx context.Context, id int) (*OneTemplateInfo, error)
	DeleteTemplate(ctx context.Context, id int) error
}

// ── Value types returned by the interface (no library type leakage) ────────────

// OneVMInfo holds the fields gecko cares about for an OpenNebula VM.
type OneVMInfo struct {
	ID     int
	Name   string
	Status string
	CPU    int
	Memory int
}

// OneVNetInfo holds the fields gecko cares about for an OpenNebula virtual network.
type OneVNetInfo struct {
	ID     int
	Name   string
	Bridge string
}

// OneImageInfo holds the fields gecko cares about for an OpenNebula image.
type OneImageInfo struct {
	ID   int
	Name string
	Type string
	Size int
}

// OneTemplateInfo holds the fields gecko cares about for an OpenNebula template.
type OneTemplateInfo struct {
	ID          int
	Name        string
	Description string
}

// ── OpenNebulaProvider ────────────────────────────────────────────────────────

// OpenNebulaProvider manages OpenNebula resources via the XML-RPC API.
type OpenNebulaProvider struct {
	endpoint string
	username string
	password string
	api      opennebulaAPI
}

// NewOpenNebulaProvider creates a new OpenNebula provider with optional config.
func NewOpenNebulaProvider(config map[string]interface{}) *OpenNebulaProvider {
	p := &OpenNebulaProvider{
		endpoint: "http://localhost:2633/RPC2",
	}
	if config != nil {
		if v, ok := config["endpoint"].(string); ok && v != "" {
			p.endpoint = v
		}
		if v, ok := config["username"].(string); ok && v != "" {
			p.username = v
		}
		if v, ok := config["password"].(string); ok && v != "" {
			p.password = v
		}
	}
	return p
}

func (p *OpenNebulaProvider) Name() string    { return "opennebula" }
func (p *OpenNebulaProvider) Version() string { return "0.1.0" }
func (p *OpenNebulaProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"opennebula:vm",
		"opennebula:vnet",
		"opennebula:image",
		"opennebula:template",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *OpenNebulaProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["endpoint"].(string); ok && v != "" {
			p.endpoint = v
		}
		if v, ok := config["username"].(string); ok && v != "" {
			p.username = v
		}
		if v, ok := config["password"].(string); ok && v != "" {
			p.password = v
		}
	}
	return nil
}

func (p *OpenNebulaProvider) connect() error {
	if p.api != nil {
		return nil
	}
	p.api = &realOpenNebulaAPI{
		client:   &http.Client{},
		endpoint: p.endpoint,
		username: p.username,
		password: p.password,
	}
	return nil
}

func (p *OpenNebulaProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func opennebulaResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *OpenNebulaProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "opennebula:vm":
		return p.createVM(ctx, args)
	case "opennebula:vnet":
		return p.createVNet(ctx, args)
	case "opennebula:image":
		return p.createImage(ctx, args)
	case "opennebula:template":
		return p.createTemplate(ctx, args)
	default:
		return nil, fmt.Errorf("opennebula: unsupported resource type %q", args.Type)
	}
}

func (p *OpenNebulaProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("opennebula: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "opennebula:vm":
		return p.readVM(ctx, id, externalID)
	case "opennebula:vnet":
		return p.readVNet(ctx, id, externalID)
	case "opennebula:image":
		return p.readImage(ctx, id, externalID)
	case "opennebula:template":
		return p.readTemplate(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("opennebula: unsupported resource type %q", rType)
	}
}

func (p *OpenNebulaProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "opennebula:vm":
		return p.updateVM(ctx, current, desired)
	case "opennebula:vnet":
		return p.updateVNet(ctx, current, desired)
	case "opennebula:image":
		return p.updateImage(ctx, current, desired)
	case "opennebula:template":
		return p.updateTemplate(ctx, current, desired)
	default:
		return nil, fmt.Errorf("opennebula: unsupported resource type %q", desired.Type)
	}
}

func (p *OpenNebulaProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "opennebula:vm":
		return p.deleteVM(ctx, state)
	case "opennebula:vnet":
		return p.deleteVNet(ctx, state)
	case "opennebula:image":
		return p.deleteImage(ctx, state)
	case "opennebula:template":
		return p.deleteTemplate(ctx, state)
	default:
		return fmt.Errorf("opennebula: unsupported resource type %q", state.Type)
	}
}

func (p *OpenNebulaProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *OpenNebulaProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := opennebulaResourceID(desired)

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
	case "opennebula:vm":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cpu", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("memory", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("template", current.Inputs, desired.Inputs)...)
	case "opennebula:vnet":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("bridge", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("template", current.Inputs, desired.Inputs)...)
	case "opennebula:image":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("type", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("size", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("datastore_id", current.Inputs, desired.Inputs)...)
	case "opennebula:template":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("description", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("template", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── VM CRUD ───────────────────────────────────────────────────────────────────

func (p *OpenNebulaProvider) createVM(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	template := getStringInputFallback(args.Inputs, "template", "")
	vmName := getStringInputFallback(args.Inputs, "name", args.Name)

	id, err := p.api.CreateVM(ctx, template, vmName)
	if err != nil {
		return nil, fmt.Errorf("opennebula: create vm %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         opennebulaResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": id},
		ExternalID: fmt.Sprintf("%d", id),
		ProviderID: "opennebula",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) readVM(ctx context.Context, rid core.ResourceID, externalID string) (*core.ResourceState, error) {
	vmid, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: invalid vm externalID %q: %w", externalID, err)
	}

	info, err := p.api.GetVM(ctx, vmid)
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
		ID:         rid,
		Type:       "opennebula:vm",
		ExternalID: externalID,
		ProviderID: "opennebula",
		Status:     status,
		Inputs: core.Inputs{
			"name":   info.Name,
			"cpu":    info.CPU,
			"memory": info.Memory,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) updateVM(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	vmid, err := strconv.Atoi(current.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: invalid vm externalID %q: %w", current.ExternalID, err)
	}

	template := getStringInputFallback(desired.Inputs, "template", "")
	if err := p.api.UpdateVM(ctx, vmid, template); err != nil {
		return nil, fmt.Errorf("opennebula: update vm %q: %w", desired.Name, err)
	}

	return p.readVM(ctx, opennebulaResourceID(desired), current.ExternalID)
}

func (p *OpenNebulaProvider) deleteVM(ctx context.Context, state *core.ResourceState) error {
	vmid, err := strconv.Atoi(state.ExternalID)
	if err != nil {
		return fmt.Errorf("opennebula: invalid vm externalID %q: %w", state.ExternalID, err)
	}

	info, err := p.api.GetVM(ctx, vmid)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteVM(ctx, vmid)
}

// ── VNet CRUD ─────────────────────────────────────────────────────────────────

func (p *OpenNebulaProvider) createVNet(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	template := getStringInputFallback(args.Inputs, "template", "")

	id, err := p.api.CreateVNet(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("opennebula: create vnet %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         opennebulaResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": id},
		ExternalID: fmt.Sprintf("%d", id),
		ProviderID: "opennebula",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) readVNet(ctx context.Context, rid core.ResourceID, externalID string) (*core.ResourceState, error) {
	vnetID, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: invalid vnet externalID %q: %w", externalID, err)
	}

	info, err := p.api.GetVNet(ctx, vnetID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         rid,
		Type:       "opennebula:vnet",
		ExternalID: externalID,
		ProviderID: "opennebula",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":   info.Name,
			"bridge": info.Bridge,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) updateVNet(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return nil, fmt.Errorf("opennebula: vnet update is not supported; delete and recreate instead")
}

func (p *OpenNebulaProvider) deleteVNet(ctx context.Context, state *core.ResourceState) error {
	vnetID, err := strconv.Atoi(state.ExternalID)
	if err != nil {
		return fmt.Errorf("opennebula: invalid vnet externalID %q: %w", state.ExternalID, err)
	}

	info, err := p.api.GetVNet(ctx, vnetID)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteVNet(ctx, vnetID)
}

// ── Image CRUD ────────────────────────────────────────────────────────────────

func (p *OpenNebulaProvider) createImage(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	template := getStringInputFallback(args.Inputs, "template", "")
	datastoreID, _ := toInt(args.Inputs["datastore_id"])

	id, err := p.api.CreateImage(ctx, template, datastoreID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: create image %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         opennebulaResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": id},
		ExternalID: fmt.Sprintf("%d", id),
		ProviderID: "opennebula",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) readImage(ctx context.Context, rid core.ResourceID, externalID string) (*core.ResourceState, error) {
	imageID, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: invalid image externalID %q: %w", externalID, err)
	}

	info, err := p.api.GetImage(ctx, imageID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         rid,
		Type:       "opennebula:image",
		ExternalID: externalID,
		ProviderID: "opennebula",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name": info.Name,
			"type": info.Type,
			"size": info.Size,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) updateImage(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return nil, fmt.Errorf("opennebula: image update is not supported; delete and recreate instead")
}

func (p *OpenNebulaProvider) deleteImage(ctx context.Context, state *core.ResourceState) error {
	imageID, err := strconv.Atoi(state.ExternalID)
	if err != nil {
		return fmt.Errorf("opennebula: invalid image externalID %q: %w", state.ExternalID, err)
	}

	info, err := p.api.GetImage(ctx, imageID)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteImage(ctx, imageID)
}

// ── Template CRUD ─────────────────────────────────────────────────────────────

func (p *OpenNebulaProvider) createTemplate(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	template := getStringInputFallback(args.Inputs, "template", "")

	id, err := p.api.CreateTemplate(ctx, template)
	if err != nil {
		return nil, fmt.Errorf("opennebula: create template %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         opennebulaResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"id": id},
		ExternalID: fmt.Sprintf("%d", id),
		ProviderID: "opennebula",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) readTemplate(ctx context.Context, rid core.ResourceID, externalID string) (*core.ResourceState, error) {
	templateID, err := strconv.Atoi(externalID)
	if err != nil {
		return nil, fmt.Errorf("opennebula: invalid template externalID %q: %w", externalID, err)
	}

	info, err := p.api.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         rid,
		Type:       "opennebula:template",
		ExternalID: externalID,
		ProviderID: "opennebula",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name":        info.Name,
			"description": info.Description,
		},
		Outputs:   core.Outputs{"id": info.ID},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *OpenNebulaProvider) updateTemplate(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	return nil, fmt.Errorf("opennebula: template update is not supported; delete and recreate instead")
}

func (p *OpenNebulaProvider) deleteTemplate(ctx context.Context, state *core.ResourceState) error {
	templateID, err := strconv.Atoi(state.ExternalID)
	if err != nil {
		return fmt.Errorf("opennebula: invalid template externalID %q: %w", state.ExternalID, err)
	}

	info, err := p.api.GetTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // already gone
	}

	return p.api.DeleteTemplate(ctx, templateID)
}

// ── Real API implementation (stub) ────────────────────────────────────────────

type realOpenNebulaAPI struct {
	client   *http.Client
	endpoint string
	username string
	password string
}

func (r *realOpenNebulaAPI) CreateVM(ctx context.Context, template string, name string) (int, error) {
	return 0, fmt.Errorf("opennebula: CreateVM not yet implemented")
}

func (r *realOpenNebulaAPI) GetVM(ctx context.Context, id int) (*OneVMInfo, error) {
	return nil, fmt.Errorf("opennebula: GetVM not yet implemented")
}

func (r *realOpenNebulaAPI) UpdateVM(ctx context.Context, id int, template string) error {
	return fmt.Errorf("opennebula: UpdateVM not yet implemented")
}

func (r *realOpenNebulaAPI) DeleteVM(ctx context.Context, id int) error {
	return fmt.Errorf("opennebula: DeleteVM not yet implemented")
}

func (r *realOpenNebulaAPI) CreateVNet(ctx context.Context, template string) (int, error) {
	return 0, fmt.Errorf("opennebula: CreateVNet not yet implemented")
}

func (r *realOpenNebulaAPI) GetVNet(ctx context.Context, id int) (*OneVNetInfo, error) {
	return nil, fmt.Errorf("opennebula: GetVNet not yet implemented")
}

func (r *realOpenNebulaAPI) DeleteVNet(ctx context.Context, id int) error {
	return fmt.Errorf("opennebula: DeleteVNet not yet implemented")
}

func (r *realOpenNebulaAPI) CreateImage(ctx context.Context, template string, datastoreID int) (int, error) {
	return 0, fmt.Errorf("opennebula: CreateImage not yet implemented")
}

func (r *realOpenNebulaAPI) GetImage(ctx context.Context, id int) (*OneImageInfo, error) {
	return nil, fmt.Errorf("opennebula: GetImage not yet implemented")
}

func (r *realOpenNebulaAPI) DeleteImage(ctx context.Context, id int) error {
	return fmt.Errorf("opennebula: DeleteImage not yet implemented")
}

func (r *realOpenNebulaAPI) CreateTemplate(ctx context.Context, template string) (int, error) {
	return 0, fmt.Errorf("opennebula: CreateTemplate not yet implemented")
}

func (r *realOpenNebulaAPI) GetTemplate(ctx context.Context, id int) (*OneTemplateInfo, error) {
	return nil, fmt.Errorf("opennebula: GetTemplate not yet implemented")
}

func (r *realOpenNebulaAPI) DeleteTemplate(ctx context.Context, id int) error {
	return fmt.Errorf("opennebula: DeleteTemplate not yet implemented")
}
