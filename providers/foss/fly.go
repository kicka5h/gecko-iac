package foss

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// flyAPI is the interface over all Fly.io operations used by this provider.
// The real implementation delegates to the Fly.io Machines API; a mock can be injected for tests.
type flyAPI interface {
	CreateApp(ctx context.Context, name, org string) (*AppInfo, error)
	GetApp(ctx context.Context, name string) (*AppInfo, error)
	DeleteApp(ctx context.Context, name string) error

	CreateMachine(ctx context.Context, appName string, config map[string]interface{}) (*MachineInfo, error)
	GetMachine(ctx context.Context, appName, machineID string) (*MachineInfo, error)
	UpdateMachine(ctx context.Context, appName, machineID string, config map[string]interface{}) (*MachineInfo, error)
	DeleteMachine(ctx context.Context, appName, machineID string) error

	CreateVolume(ctx context.Context, appName string, config map[string]interface{}) (*FlyVolumeInfo, error)
	GetVolume(ctx context.Context, appName, volumeID string) (*FlyVolumeInfo, error)
	DeleteVolume(ctx context.Context, appName, volumeID string) error

	SetSecrets(ctx context.Context, appName string, secrets map[string]string) error
	UnsetSecret(ctx context.Context, appName, key string) error
	ListSecrets(ctx context.Context, appName string) ([]SecretInfo, error)
}

// ── Value types returned by the interface (no library type leakage) ────────────

// AppInfo holds the fields gecko cares about for a Fly.io app.
type AppInfo struct {
	Name   string
	Org    string
	Status string
}

// MachineInfo holds the fields gecko cares about for a Fly.io machine.
type MachineInfo struct {
	ID     string
	Name   string
	State  string
	Region string
	Image  string
	CPUs   int
	Memory int
}

// FlyVolumeInfo holds the fields gecko cares about for a Fly.io volume.
type FlyVolumeInfo struct {
	ID     string
	Name   string
	Region string
	SizeGB int
	State  string
}

// SecretInfo holds the fields gecko cares about for a Fly.io secret.
type SecretInfo struct {
	Name      string
	Digest    string
	CreatedAt time.Time
}

// ── FlyProvider ───────────────────────────────────────────────────────────────

// FlyProvider manages Fly.io resources via the Machines REST API.
type FlyProvider struct {
	apiToken string
	org      string
	region   string
	api      flyAPI
}

// NewFlyProvider creates a new Fly.io provider with optional config.
func NewFlyProvider(config map[string]interface{}) *FlyProvider {
	p := &FlyProvider{
		org:    "personal",
		region: "iad",
	}
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
		if v, ok := config["org"].(string); ok && v != "" {
			p.org = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
	}
	return p
}

func (p *FlyProvider) Name() string    { return "fly" }
func (p *FlyProvider) Version() string { return "0.1.0" }
func (p *FlyProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"fly:app",
		"fly:machine",
		"fly:volume",
		"fly:secret",
	}
}

// Configure stores config; the API client is built lazily on first use.
func (p *FlyProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["api_token"].(string); ok && v != "" {
			p.apiToken = v
		}
		if v, ok := config["org"].(string); ok && v != "" {
			p.org = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
	}
	return nil
}

func (p *FlyProvider) connect() error {
	if p.api != nil {
		return nil
	}
	if p.apiToken == "" {
		return fmt.Errorf("fly: api_token is required")
	}
	httpClient := &http.Client{}
	p.api = newRealFlyAPI(httpClient, "https://api.machines.dev/v1", p.apiToken)
	return nil
}

func (p *FlyProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func flyResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *FlyProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "fly:app":
		return p.createApp(ctx, args)
	case "fly:machine":
		return p.createMachine(ctx, args)
	case "fly:volume":
		return p.createVolume(ctx, args)
	case "fly:secret":
		return p.createSecret(ctx, args)
	default:
		return nil, fmt.Errorf("fly: unsupported resource type %q", args.Type)
	}
}

func (p *FlyProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fly: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "fly:app":
		return p.readApp(ctx, id, externalID)
	case "fly:machine":
		return p.readMachine(ctx, id, externalID)
	case "fly:volume":
		return p.readVolume(ctx, id, externalID)
	case "fly:secret":
		return p.readSecret(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("fly: unsupported resource type %q", rType)
	}
}

func (p *FlyProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch desired.Type {
	case "fly:app":
		return p.updateApp(ctx, current, desired)
	case "fly:machine":
		return p.updateMachine(ctx, current, desired)
	case "fly:volume":
		return p.updateVolume(ctx, current, desired)
	case "fly:secret":
		return p.updateSecret(ctx, current, desired)
	default:
		return nil, fmt.Errorf("fly: unsupported resource type %q", desired.Type)
	}
}

func (p *FlyProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "fly:app":
		return p.deleteApp(ctx, state)
	case "fly:machine":
		return p.deleteMachine(ctx, state)
	case "fly:volume":
		return p.deleteVolume(ctx, state)
	case "fly:secret":
		return p.deleteSecret(ctx, state)
	default:
		return fmt.Errorf("fly: unsupported resource type %q", state.Type)
	}
}

func (p *FlyProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *FlyProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := flyResourceID(desired)

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
	case "fly:app":
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("org", current.Inputs, desired.Inputs)...)
	case "fly:machine":
		changes = append(changes, compareField("app", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("image", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("region", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("cpus", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("memory", current.Inputs, desired.Inputs)...)
	case "fly:volume":
		changes = append(changes, compareField("app", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("name", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("region", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("size_gb", current.Inputs, desired.Inputs)...)
	case "fly:secret":
		changes = append(changes, compareField("app", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("key", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("value", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── App CRUD ──────────────────────────────────────────────────────────────────

func (p *FlyProvider) createApp(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	appName := getStringInputFallback(args.Inputs, "name", args.Name)
	org := getStringInputFallback(args.Inputs, "org", p.org)

	info, err := p.api.CreateApp(ctx, appName, org)
	if err != nil {
		return nil, fmt.Errorf("fly: create app %q: %w", appName, err)
	}

	return &core.ResourceState{
		ID:         flyResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"name": info.Name, "org": info.Org},
		ExternalID: info.Name,
		ProviderID: "fly",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *FlyProvider) readApp(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	info, err := p.api.GetApp(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "fly:app",
		ExternalID: externalID,
		ProviderID: "fly",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"name": info.Name,
			"org":  info.Org,
		},
		Outputs:   core.Outputs{"name": info.Name, "org": info.Org},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *FlyProvider) updateApp(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// Fly.io apps cannot be updated in place; return current state.
	return current, nil
}

func (p *FlyProvider) deleteApp(ctx context.Context, state *core.ResourceState) error {
	return p.api.DeleteApp(ctx, state.ExternalID)
}

// ── Machine CRUD ──────────────────────────────────────────────────────────────

func (p *FlyProvider) createMachine(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	appName, _ := args.Inputs["app"].(string)
	if appName == "" {
		return nil, fmt.Errorf("fly: machine requires 'app' input")
	}

	config := map[string]interface{}{
		"name":   getStringInputFallback(args.Inputs, "name", args.Name),
		"region": getStringInputFallback(args.Inputs, "region", p.region),
	}
	if v, ok := args.Inputs["image"].(string); ok && v != "" {
		config["image"] = v
	}
	if v, ok := toInt(args.Inputs["cpus"]); ok {
		config["cpus"] = v
	}
	if v, ok := toInt(args.Inputs["memory"]); ok {
		config["memory"] = v
	}

	info, err := p.api.CreateMachine(ctx, appName, config)
	if err != nil {
		return nil, fmt.Errorf("fly: create machine %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         flyResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"machine_id": info.ID, "name": info.Name, "region": info.Region},
		ExternalID: info.ID,
		ProviderID: "fly",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *FlyProvider) readMachine(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// externalID format: "appName/machineID"
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fly: invalid machine externalID %q (expected app/machineID)", externalID)
	}
	appName, machineID := parts[0], parts[1]

	info, err := p.api.GetMachine(ctx, appName, machineID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	status := core.StatusRunning
	if info.State == "stopped" {
		status = core.StatusPending
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "fly:machine",
		ExternalID: externalID,
		ProviderID: "fly",
		Status:     status,
		Inputs: core.Inputs{
			"app":    appName,
			"name":   info.Name,
			"image":  info.Image,
			"region": info.Region,
			"cpus":   info.CPUs,
			"memory": info.Memory,
		},
		Outputs:   core.Outputs{"machine_id": info.ID, "name": info.Name, "region": info.Region},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *FlyProvider) updateMachine(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	parts := strings.SplitN(current.ExternalID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fly: invalid machine externalID %q", current.ExternalID)
	}
	appName, machineID := parts[0], parts[1]

	config := map[string]interface{}{
		"name":   getStringInputFallback(desired.Inputs, "name", desired.Name),
		"region": getStringInputFallback(desired.Inputs, "region", p.region),
	}
	if v, ok := desired.Inputs["image"].(string); ok && v != "" {
		config["image"] = v
	}
	if v, ok := toInt(desired.Inputs["cpus"]); ok {
		config["cpus"] = v
	}
	if v, ok := toInt(desired.Inputs["memory"]); ok {
		config["memory"] = v
	}

	if _, err := p.api.UpdateMachine(ctx, appName, machineID, config); err != nil {
		return nil, err
	}

	return p.readMachine(ctx, flyResourceID(desired), current.ExternalID)
}

func (p *FlyProvider) deleteMachine(ctx context.Context, state *core.ResourceState) error {
	parts := strings.SplitN(state.ExternalID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("fly: invalid machine externalID %q", state.ExternalID)
	}
	return p.api.DeleteMachine(ctx, parts[0], parts[1])
}

// ── Volume CRUD ───────────────────────────────────────────────────────────────

func (p *FlyProvider) createVolume(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	appName, _ := args.Inputs["app"].(string)
	if appName == "" {
		return nil, fmt.Errorf("fly: volume requires 'app' input")
	}

	config := map[string]interface{}{
		"name":   getStringInputFallback(args.Inputs, "name", args.Name),
		"region": getStringInputFallback(args.Inputs, "region", p.region),
	}
	if v, ok := toInt(args.Inputs["size_gb"]); ok {
		config["size_gb"] = v
	}

	info, err := p.api.CreateVolume(ctx, appName, config)
	if err != nil {
		return nil, fmt.Errorf("fly: create volume %q: %w", args.Name, err)
	}

	return &core.ResourceState{
		ID:         flyResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"volume_id": info.ID, "name": info.Name, "region": info.Region, "size_gb": info.SizeGB},
		ExternalID: appName + "/" + info.ID,
		ProviderID: "fly",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *FlyProvider) readVolume(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fly: invalid volume externalID %q (expected app/volumeID)", externalID)
	}
	appName, volumeID := parts[0], parts[1]

	info, err := p.api.GetVolume(ctx, appName, volumeID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "fly:volume",
		ExternalID: externalID,
		ProviderID: "fly",
		Status:     core.StatusRunning,
		Inputs: core.Inputs{
			"app":     appName,
			"name":    info.Name,
			"region":  info.Region,
			"size_gb": info.SizeGB,
		},
		Outputs:   core.Outputs{"volume_id": info.ID, "name": info.Name, "region": info.Region, "size_gb": info.SizeGB},
		UpdatedAt: time.Now(),
	}, nil
}

func (p *FlyProvider) updateVolume(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	// Fly.io volumes cannot be updated in place; return current state.
	return current, nil
}

func (p *FlyProvider) deleteVolume(ctx context.Context, state *core.ResourceState) error {
	parts := strings.SplitN(state.ExternalID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("fly: invalid volume externalID %q", state.ExternalID)
	}
	return p.api.DeleteVolume(ctx, parts[0], parts[1])
}

// ── Secret CRUD ───────────────────────────────────────────────────────────────

func (p *FlyProvider) createSecret(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	appName, _ := args.Inputs["app"].(string)
	if appName == "" {
		return nil, fmt.Errorf("fly: secret requires 'app' input")
	}
	key, _ := args.Inputs["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("fly: secret requires 'key' input")
	}
	value, _ := args.Inputs["value"].(string)

	if err := p.api.SetSecrets(ctx, appName, map[string]string{key: value}); err != nil {
		return nil, fmt.Errorf("fly: set secret %q: %w", key, err)
	}

	return &core.ResourceState{
		ID:         flyResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"app": appName, "key": key},
		ExternalID: appName + "/" + key,
		ProviderID: "fly",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *FlyProvider) readSecret(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("fly: invalid secret externalID %q (expected app/key)", externalID)
	}
	appName, key := parts[0], parts[1]

	secrets, err := p.api.ListSecrets(ctx, appName)
	if err != nil {
		return nil, err
	}

	for _, s := range secrets {
		if s.Name == key {
			return &core.ResourceState{
				ID:         id,
				Type:       "fly:secret",
				ExternalID: externalID,
				ProviderID: "fly",
				Status:     core.StatusRunning,
				Inputs: core.Inputs{
					"app": appName,
					"key": key,
				},
				Outputs:   core.Outputs{"app": appName, "key": key, "digest": s.Digest},
				UpdatedAt: time.Now(),
			}, nil
		}
	}

	return nil, nil // not found
}

func (p *FlyProvider) updateSecret(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	appName, _ := desired.Inputs["app"].(string)
	if appName == "" {
		return nil, fmt.Errorf("fly: secret requires 'app' input")
	}
	key, _ := desired.Inputs["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("fly: secret requires 'key' input")
	}
	value, _ := desired.Inputs["value"].(string)

	if err := p.api.SetSecrets(ctx, appName, map[string]string{key: value}); err != nil {
		return nil, fmt.Errorf("fly: update secret %q: %w", key, err)
	}

	return p.readSecret(ctx, flyResourceID(desired), current.ExternalID)
}

func (p *FlyProvider) deleteSecret(ctx context.Context, state *core.ResourceState) error {
	parts := strings.SplitN(state.ExternalID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("fly: invalid secret externalID %q", state.ExternalID)
	}
	return p.api.UnsetSecret(ctx, parts[0], parts[1])
}
