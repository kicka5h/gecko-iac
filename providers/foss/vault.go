package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/gecko-iac/gecko/internal/core"
)

// VaultProvider manages HashiCorp Vault resources.
// Supports secrets (KV v2), policies, auth methods, and secrets engine mounts.
type VaultProvider struct {
	address   string
	token     string
	namespace string // Vault Enterprise namespace
	client    *vaultapi.Client
}

// NewVaultProvider creates a new Vault provider with optional config.
func NewVaultProvider(config map[string]interface{}) *VaultProvider {
	p := &VaultProvider{
		address: "https://127.0.0.1:8200",
	}
	if config != nil {
		if v, ok := config["address"].(string); ok && v != "" {
			p.address = v
		}
		if v, ok := config["token"].(string); ok && v != "" {
			p.token = v
		}
		if v, ok := config["namespace"].(string); ok && v != "" {
			p.namespace = v
		}
	}
	return p
}

func (p *VaultProvider) Name() string    { return "vault" }
func (p *VaultProvider) Version() string { return "0.1.0" }
func (p *VaultProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"vault:secret",
		"vault:policy",
		"vault:auth",
		"vault:mount",
	}
}

// Configure stores config; client is built lazily on first use.
func (p *VaultProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["address"].(string); ok && v != "" {
			p.address = v
		}
		if v, ok := config["token"].(string); ok && v != "" {
			p.token = v
		}
		if v, ok := config["namespace"].(string); ok && v != "" {
			p.namespace = v
		}
	}
	return nil
}

func (p *VaultProvider) connect() error {
	if p.client != nil {
		return nil
	}
	cfg := vaultapi.DefaultConfig()
	if p.address != "" {
		cfg.Address = p.address
	}
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("vault: failed to create client: %w", err)
	}
	if p.token != "" {
		client.SetToken(p.token)
	}
	if p.namespace != "" {
		client.SetNamespace(p.namespace)
	}
	p.client = client
	return nil
}

func (p *VaultProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func vaultResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

// ── vault:secret (KV v2) ──────────────────────────────────────────────────────

func (p *VaultProvider) secretMount(inputs core.Inputs) string {
	if v, ok := inputs["mount"].(string); ok && v != "" {
		return v
	}
	return "secret"
}

func (p *VaultProvider) secretPath(inputs core.Inputs, name string) string {
	if v, ok := inputs["path"].(string); ok && v != "" {
		return v
	}
	return name
}

func (p *VaultProvider) createSecret(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	mount := p.secretMount(args.Inputs)
	path := p.secretPath(args.Inputs, args.Name)

	data, _ := args.Inputs["data"].(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}

	_, err := p.client.KVv2(mount).Put(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf("vault: write secret %s/%s: %w", mount, path, err)
	}

	return &core.ResourceState{
		ID:         vaultResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"path": mount + "/data/" + path},
		ExternalID: mount + "/" + path,
		ProviderID: "vault",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) readSecret(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// externalID = "mount/path"
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	mount, path := parts[0], parts[1]

	secret, err := p.client.KVv2(mount).Get(ctx, path)
	if err != nil {
		if strings.Contains(err.Error(), "secret not found") || strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: read secret %s/%s: %w", mount, path, err)
	}
	if secret == nil {
		return nil, nil
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "vault:secret",
		ExternalID: externalID,
		ProviderID: "vault",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"data": secret.Data},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) deleteSecret(ctx context.Context, state *core.ResourceState) error {
	externalID := state.ExternalID
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("vault: invalid secret externalID: %s", externalID)
	}
	mount, path := parts[0], parts[1]
	if err := p.client.KVv2(mount).Delete(ctx, path); err != nil {
		return fmt.Errorf("vault: delete secret %s/%s: %w", mount, path, err)
	}
	return nil
}

// ── vault:policy ──────────────────────────────────────────────────────────────

func (p *VaultProvider) createPolicy(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	name := getStringInputFallback(args.Inputs, "name", args.Name)
	rules, _ := args.Inputs["rules"].(string)

	if err := p.client.Sys().PutPolicy(name, rules); err != nil {
		return nil, fmt.Errorf("vault: put policy %q: %w", name, err)
	}

	return &core.ResourceState{
		ID:         vaultResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		ExternalID: name,
		ProviderID: "vault",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) readPolicy(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	rules, err := p.client.Sys().GetPolicy(externalID)
	if err != nil {
		return nil, fmt.Errorf("vault: get policy %q: %w", externalID, err)
	}
	if rules == "" {
		return nil, nil // not found
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "vault:policy",
		ExternalID: externalID,
		ProviderID: "vault",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"name": externalID, "rules": rules},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) deletePolicy(ctx context.Context, state *core.ResourceState) error {
	if err := p.client.Sys().DeletePolicy(state.ExternalID); err != nil {
		return fmt.Errorf("vault: delete policy %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── vault:auth ────────────────────────────────────────────────────────────────

func (p *VaultProvider) createAuth(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	path := getStringInputFallback(args.Inputs, "path", args.Name)
	authType := getStringInputFallback(args.Inputs, "type", path)
	desc, _ := args.Inputs["description"].(string)

	if err := p.client.Sys().EnableAuthWithOptions(path, &vaultapi.EnableAuthOptions{
		Type:        authType,
		Description: desc,
	}); err != nil {
		// Already enabled is not an error
		if !strings.Contains(err.Error(), "already in use") {
			return nil, fmt.Errorf("vault: enable auth %q: %w", path, err)
		}
	}

	return &core.ResourceState{
		ID:         vaultResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"path": "auth/" + path},
		ExternalID: path,
		ProviderID: "vault",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) readAuth(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	auths, err := p.client.Sys().ListAuth()
	if err != nil {
		return nil, fmt.Errorf("vault: list auth methods: %w", err)
	}
	key := strings.TrimSuffix(externalID, "/") + "/"
	info, ok := auths[key]
	if !ok {
		return nil, nil
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "vault:auth",
		ExternalID: externalID,
		ProviderID: "vault",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"path": externalID, "type": info.Type},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) deleteAuth(ctx context.Context, state *core.ResourceState) error {
	if err := p.client.Sys().DisableAuth(state.ExternalID); err != nil {
		return fmt.Errorf("vault: disable auth %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── vault:mount ───────────────────────────────────────────────────────────────

func (p *VaultProvider) createMount(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	path := getStringInputFallback(args.Inputs, "path", args.Name)
	mountType := getStringInputFallback(args.Inputs, "type", "kv")
	desc, _ := args.Inputs["description"].(string)

	options := map[string]string{}
	if opts, ok := args.Inputs["options"].(map[string]interface{}); ok {
		for k, v := range opts {
			options[k] = fmt.Sprintf("%v", v)
		}
	}

	if err := p.client.Sys().Mount(path, &vaultapi.MountInput{
		Type:        mountType,
		Description: desc,
		Options:     options,
	}); err != nil {
		if !strings.Contains(err.Error(), "already in use") {
			return nil, fmt.Errorf("vault: mount %q: %w", path, err)
		}
	}

	return &core.ResourceState{
		ID:         vaultResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"path": path},
		ExternalID: path,
		ProviderID: "vault",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) readMount(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	mounts, err := p.client.Sys().ListMounts()
	if err != nil {
		return nil, fmt.Errorf("vault: list mounts: %w", err)
	}
	key := strings.TrimSuffix(externalID, "/") + "/"
	info, ok := mounts[key]
	if !ok {
		return nil, nil
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "vault:mount",
		ExternalID: externalID,
		ProviderID: "vault",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"path": externalID, "type": info.Type},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *VaultProvider) deleteMount(ctx context.Context, state *core.ResourceState) error {
	if err := p.client.Sys().Unmount(state.ExternalID); err != nil {
		return fmt.Errorf("vault: unmount %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *VaultProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "vault:secret":
		return p.createSecret(ctx, args)
	case "vault:policy":
		return p.createPolicy(ctx, args)
	case "vault:auth":
		return p.createAuth(ctx, args)
	case "vault:mount":
		return p.createMount(ctx, args)
	default:
		return nil, fmt.Errorf("vault: unsupported resource type %q", args.Type)
	}
}

func (p *VaultProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	// Derive type from ID ("vault:secret::name" → "vault:secret")
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("vault: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "vault:secret":
		return p.readSecret(ctx, id, externalID)
	case "vault:policy":
		return p.readPolicy(ctx, id, externalID)
	case "vault:auth":
		return p.readAuth(ctx, id, externalID)
	case "vault:mount":
		return p.readMount(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("vault: unsupported resource type %q", rType)
	}
}

func (p *VaultProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	// All vault resources are idempotent on create — re-create = update
	return p.Create(ctx, desired)
}

func (p *VaultProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "vault:secret":
		return p.deleteSecret(ctx, state)
	case "vault:policy":
		return p.deletePolicy(ctx, state)
	case "vault:auth":
		return p.deleteAuth(ctx, state)
	case "vault:mount":
		return p.deleteMount(ctx, state)
	default:
		return fmt.Errorf("vault: unsupported resource type %q", state.Type)
	}
}

func (p *VaultProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *VaultProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := vaultResourceID(desired)

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
	case "vault:secret":
		changes = append(changes, compareMapField("data", current.Inputs, desired.Inputs)...)
	case "vault:policy":
		changes = append(changes, compareField("rules", current.Inputs, desired.Inputs)...)
	case "vault:auth":
		changes = append(changes, compareField("type", current.Inputs, desired.Inputs)...)
	case "vault:mount":
		changes = append(changes, compareField("type", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func getStringInputFallback(inputs core.Inputs, key, fallback string) string {
	if v, ok := inputs[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func compareMapField(field string, current, desired core.Inputs) []core.FieldChange {
	cm, _ := current[field].(map[string]interface{})
	dm, _ := desired[field].(map[string]interface{})
	var changes []core.FieldChange
	for k, dv := range dm {
		cv, exists := cm[k]
		if !exists {
			changes = append(changes, core.FieldChange{Field: field + "." + k, Kind: core.ChangeAdd, NewValue: dv})
		} else if fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", dv) {
			changes = append(changes, core.FieldChange{Field: field + "." + k, Kind: core.ChangeUpdate, OldValue: cv, NewValue: dv})
		}
	}
	for k, cv := range cm {
		if _, exists := dm[k]; !exists {
			changes = append(changes, core.FieldChange{Field: field + "." + k, Kind: core.ChangeDelete, OldValue: cv})
		}
	}
	return changes
}
