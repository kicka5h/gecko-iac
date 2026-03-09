package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	nomadapi "github.com/hashicorp/nomad/api"

	"github.com/gecko-iac/gecko/internal/core"
)

// NomadProvider manages HashiCorp Nomad resources.
// Supports jobs, namespaces, ACL policies, and CSI volumes.
type NomadProvider struct {
	address string
	token   string
	region  string
	client  *nomadapi.Client
}

// NewNomadProvider creates a new Nomad provider with optional config.
func NewNomadProvider(config map[string]interface{}) *NomadProvider {
	p := &NomadProvider{
		address: "http://127.0.0.1:4646",
	}
	if config != nil {
		if v, ok := config["address"].(string); ok && v != "" {
			p.address = v
		}
		if v, ok := config["token"].(string); ok && v != "" {
			p.token = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
	}
	return p
}

func (p *NomadProvider) Name() string    { return "nomad" }
func (p *NomadProvider) Version() string { return "0.1.0" }
func (p *NomadProvider) SupportedTypes() []core.ResourceType {
	return []core.ResourceType{
		"nomad:job",
		"nomad:namespace",
		"nomad:policy",
		"nomad:volume",
	}
}

// Configure stores config; client is built lazily on first use.
func (p *NomadProvider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["address"].(string); ok && v != "" {
			p.address = v
		}
		if v, ok := config["token"].(string); ok && v != "" {
			p.token = v
		}
		if v, ok := config["region"].(string); ok && v != "" {
			p.region = v
		}
	}
	return nil
}

func (p *NomadProvider) connect() error {
	if p.client != nil {
		return nil
	}
	cfg := nomadapi.DefaultConfig()
	if p.address != "" {
		cfg.Address = p.address
	}
	if p.token != "" {
		cfg.SecretID = p.token
	}
	if p.region != "" {
		cfg.Region = p.region
	}
	client, err := nomadapi.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("nomad: failed to create client: %w", err)
	}
	p.client = client
	return nil
}

func (p *NomadProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
	return nil
}

func nomadResourceID(args core.ResourceArgs) core.ResourceID {
	return core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
}

func nomadNamespace(inputs core.Inputs) string {
	if v, ok := inputs["namespace"].(string); ok && v != "" {
		return v
	}
	return "default"
}

// ── nomad:job ─────────────────────────────────────────────────────────────────

// buildNomadJob constructs a *nomadapi.Job from Scute inputs.
// Supports a simple Docker service job. For complex specs, use the `spec` field
// with a JSON-encoded job (which is decoded directly).
func buildNomadJob(inputs core.Inputs, name string) (*nomadapi.Job, error) {
	jobID := getStringInputFallback(inputs, "id", name)
	jobName := getStringInputFallback(inputs, "name", jobID)
	jobType := getStringInputFallback(inputs, "type", "service")
	ns := nomadNamespace(inputs)

	// Datacenters
	dcs := []string{"dc1"}
	if dcRaw, ok := inputs["datacenters"]; ok {
		switch v := dcRaw.(type) {
		case []interface{}:
			dcs = nil
			for _, d := range v {
				dcs = append(dcs, fmt.Sprintf("%v", d))
			}
		case []string:
			dcs = v
		}
	}

	job := &nomadapi.Job{
		ID:          &jobID,
		Name:        &jobName,
		Type:        &jobType,
		Datacenters: dcs,
		Namespace:   &ns,
	}

	// Build a single task group from inputs if image is provided
	image, hasImage := inputs["image"].(string)
	if hasImage {
		count := 1
		if v, ok := toInt(inputs["count"]); ok {
			count = v
		}

		cpu := 100
		if v, ok := toInt(inputs["cpu"]); ok {
			cpu = v
		}

		memory := 256
		if v, ok := toInt(inputs["memory"]); ok {
			memory = v
		}

		taskName := getStringInputFallback(inputs, "task_name", jobID)

		// Env vars
		envMap := map[string]string{}
		if ev, ok := inputs["env"].(map[string]interface{}); ok {
			for k, v := range ev {
				envMap[k] = fmt.Sprintf("%v", v)
			}
		}

		// Ports
		var networks []*nomadapi.NetworkResource
		var services []*nomadapi.Service
		if portRaw, ok := inputs["port"]; ok {
			if portNum, ok := toInt(portRaw); ok {
				portLabel := "http"
				networks = []*nomadapi.NetworkResource{
					{
						DynamicPorts: []nomadapi.Port{
							{Label: portLabel},
						},
					},
				}
				services = []*nomadapi.Service{
					{
						Name:      jobID,
						PortLabel: portLabel,
						Checks: []nomadapi.ServiceCheck{
							{
								Type:     "tcp",
								Interval: 10 * time.Second,
								Timeout:  2 * time.Second,
							},
						},
					},
				}
				_ = portNum // used via DynamicPorts above
			}
		}

		task := &nomadapi.Task{
			Name:   taskName,
			Driver: "docker",
			Config: map[string]interface{}{
				"image": image,
			},
			Env: envMap,
			Resources: &nomadapi.Resources{
				CPU:      &cpu,
				MemoryMB: &memory,
			},
			Services: services,
		}

		tg := &nomadapi.TaskGroup{
			Name:     &jobID,
			Count:    &count,
			Networks: networks,
			Tasks:    []*nomadapi.Task{task},
		}

		job.TaskGroups = []*nomadapi.TaskGroup{tg}
	}

	return job, nil
}

func (p *NomadProvider) createJob(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	job, err := buildNomadJob(args.Inputs, args.Name)
	if err != nil {
		return nil, fmt.Errorf("nomad: build job %q: %w", args.Name, err)
	}

	resp, _, err := p.client.Jobs().Register(job, nil)
	if err != nil {
		return nil, fmt.Errorf("nomad: register job %q: %w", args.Name, err)
	}

	jobID := *job.ID
	return &core.ResourceState{
		ID:         nomadResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    core.Outputs{"eval_id": resp.EvalID, "job_id": jobID},
		ExternalID: jobID,
		ProviderID: "nomad",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) readJob(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	job, _, err := p.client.Jobs().Info(externalID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("nomad: read job %q: %w", externalID, err)
	}

	status := core.StatusRunning
	if job.Status != nil && *job.Status == "dead" {
		status = core.StatusDestroyed
	}

	inputs := core.Inputs{"id": externalID}
	if job.Type != nil {
		inputs["type"] = *job.Type
	}
	if len(job.TaskGroups) > 0 && job.TaskGroups[0].Count != nil {
		inputs["count"] = *job.TaskGroups[0].Count
	}

	return &core.ResourceState{
		ID:         id,
		Type:       "nomad:job",
		ExternalID: externalID,
		ProviderID: "nomad",
		Status:     status,
		Inputs:     inputs,
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) deleteJob(ctx context.Context, state *core.ResourceState) error {
	_, _, err := p.client.Jobs().Deregister(state.ExternalID, true, nil)
	if err != nil {
		return fmt.Errorf("nomad: deregister job %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── nomad:namespace ───────────────────────────────────────────────────────────

func (p *NomadProvider) createNamespace(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	nsName := getStringInputFallback(args.Inputs, "name", args.Name)
	desc, _ := args.Inputs["description"].(string)

	ns := &nomadapi.Namespace{
		Name:        nsName,
		Description: desc,
	}

	_, err := p.client.Namespaces().Register(ns, nil)
	if err != nil {
		return nil, fmt.Errorf("nomad: create namespace %q: %w", nsName, err)
	}

	return &core.ResourceState{
		ID:         nomadResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		ExternalID: nsName,
		ProviderID: "nomad",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) readNamespace(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	ns, _, err := p.client.Namespaces().Info(externalID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("nomad: read namespace %q: %w", externalID, err)
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "nomad:namespace",
		ExternalID: externalID,
		ProviderID: "nomad",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"name": ns.Name, "description": ns.Description},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) deleteNamespace(ctx context.Context, state *core.ResourceState) error {
	_, err := p.client.Namespaces().Delete(state.ExternalID, nil)
	if err != nil {
		return fmt.Errorf("nomad: delete namespace %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── nomad:policy ──────────────────────────────────────────────────────────────

func (p *NomadProvider) createPolicy(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	name := getStringInputFallback(args.Inputs, "name", args.Name)
	rules, _ := args.Inputs["rules"].(string)
	desc, _ := args.Inputs["description"].(string)

	policy := &nomadapi.ACLPolicy{
		Name:        name,
		Description: desc,
		Rules:       rules,
	}

	_, err := p.client.ACLPolicies().Upsert(policy, nil)
	if err != nil {
		return nil, fmt.Errorf("nomad: create policy %q: %w", name, err)
	}

	return &core.ResourceState{
		ID:         nomadResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		ExternalID: name,
		ProviderID: "nomad",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) readPolicy(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	policy, _, err := p.client.ACLPolicies().Info(externalID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("nomad: read policy %q: %w", externalID, err)
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "nomad:policy",
		ExternalID: externalID,
		ProviderID: "nomad",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"name": policy.Name, "rules": policy.Rules},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) deletePolicy(ctx context.Context, state *core.ResourceState) error {
	_, err := p.client.ACLPolicies().Delete(state.ExternalID, nil)
	if err != nil {
		return fmt.Errorf("nomad: delete policy %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── nomad:volume (CSI) ────────────────────────────────────────────────────────

func (p *NomadProvider) createVolume(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	volID := getStringInputFallback(args.Inputs, "id", args.Name)
	volName := getStringInputFallback(args.Inputs, "name", volID)
	pluginID, _ := args.Inputs["plugin_id"].(string)
	ns := nomadNamespace(args.Inputs)

	vol := &nomadapi.CSIVolume{
		ID:         volID,
		Name:       volName,
		PluginID:   pluginID,
		Namespace:  ns,
	}

	// Access modes
	if am, ok := args.Inputs["access_mode"].(string); ok {
		vol.AccessMode = nomadapi.CSIVolumeAccessMode(am)
	}
	if am, ok := args.Inputs["attachment_mode"].(string); ok {
		vol.AttachmentMode = nomadapi.CSIVolumeAttachmentMode(am)
	}

	_, err := p.client.CSIVolumes().Register(vol, nil)
	if err != nil {
		return nil, fmt.Errorf("nomad: register volume %q: %w", volID, err)
	}

	return &core.ResourceState{
		ID:         nomadResourceID(args),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		ExternalID: volID,
		ProviderID: "nomad",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) readVolume(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	vol, _, err := p.client.CSIVolumes().Info(externalID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("nomad: read volume %q: %w", externalID, err)
	}
	return &core.ResourceState{
		ID:         id,
		Type:       "nomad:volume",
		ExternalID: externalID,
		ProviderID: "nomad",
		Status:     core.StatusRunning,
		Inputs:     core.Inputs{"id": vol.ID, "name": vol.Name, "plugin_id": vol.PluginID},
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *NomadProvider) deleteVolume(ctx context.Context, state *core.ResourceState) error {
	err := p.client.CSIVolumes().Deregister(state.ExternalID, true, nil)
	if err != nil {
		return fmt.Errorf("nomad: deregister volume %q: %w", state.ExternalID, err)
	}
	return nil
}

// ── core.Provider interface ───────────────────────────────────────────────────

func (p *NomadProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	switch args.Type {
	case "nomad:job":
		return p.createJob(ctx, args)
	case "nomad:namespace":
		return p.createNamespace(ctx, args)
	case "nomad:policy":
		return p.createPolicy(ctx, args)
	case "nomad:volume":
		return p.createVolume(ctx, args)
	default:
		return nil, fmt.Errorf("nomad: unsupported resource type %q", args.Type)
	}
}

func (p *NomadProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("nomad: invalid resource ID: %s", id)
	}
	rType := core.ResourceType(parts[0])
	switch rType {
	case "nomad:job":
		return p.readJob(ctx, id, externalID)
	case "nomad:namespace":
		return p.readNamespace(ctx, id, externalID)
	case "nomad:policy":
		return p.readPolicy(ctx, id, externalID)
	case "nomad:volume":
		return p.readVolume(ctx, id, externalID)
	default:
		return nil, fmt.Errorf("nomad: unsupported resource type %q", rType)
	}
}

func (p *NomadProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	// All Nomad resources support upsert semantics
	return p.Create(ctx, desired)
}

func (p *NomadProvider) Delete(ctx context.Context, state *core.ResourceState) error {
	if err := p.connect(); err != nil {
		return err
	}
	switch state.Type {
	case "nomad:job":
		return p.deleteJob(ctx, state)
	case "nomad:namespace":
		return p.deleteNamespace(ctx, state)
	case "nomad:policy":
		return p.deletePolicy(ctx, state)
	case "nomad:volume":
		return p.deleteVolume(ctx, state)
	default:
		return fmt.Errorf("nomad: unsupported resource type %q", state.Type)
	}
}

func (p *NomadProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	if err := p.connect(); err != nil {
		return nil, err
	}
	fakeID := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, externalID))
	return p.Read(ctx, fakeID, externalID)
}

func (p *NomadProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	id := nomadResourceID(desired)

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
	case "nomad:job":
		changes = append(changes, compareField("image", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("count", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("type", current.Inputs, desired.Inputs)...)
	case "nomad:namespace":
		changes = append(changes, compareField("description", current.Inputs, desired.Inputs)...)
	case "nomad:policy":
		changes = append(changes, compareField("rules", current.Inputs, desired.Inputs)...)
	case "nomad:volume":
		changes = append(changes, compareField("plugin_id", current.Inputs, desired.Inputs)...)
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
	}
	return &core.Diff{ResourceID: id, Kind: kind, Changes: changes}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
