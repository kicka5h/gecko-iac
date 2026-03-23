// Package core defines the fundamental abstractions for Gecko's resource engine.
// Resources, Providers, Stacks, and the dependency graph all live here.
package core

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Resource Types ────────────────────────────────────────────────────────────

// ResourceType is a namespaced string like "proxmox:vm" or "fly:app"
type ResourceType string

// ResourceID uniquely identifies a resource within a stack
type ResourceID string

// ResourceStatus reflects the real-world state of a resource
type ResourceStatus string

const (
	StatusUnknown   ResourceStatus = "unknown"
	StatusPending   ResourceStatus = "pending"
	StatusCreating  ResourceStatus = "creating"
	StatusRunning   ResourceStatus = "running"
	StatusUpdating  ResourceStatus = "updating"
	StatusDestroyed ResourceStatus = "destroyed"
	StatusFailed    ResourceStatus = "failed"
	StatusDrifted   ResourceStatus = "drifted"
)

// Inputs is a map of configuration values for a resource
type Inputs map[string]interface{}

// Outputs is a map of values returned after a resource is created
type Outputs map[string]interface{}

// ResourceArgs holds everything needed to declare a resource
type ResourceArgs struct {
	Type    ResourceType
	Name    string
	Inputs  Inputs
	DependsOn []ResourceID
	Tags    map[string]string
	Protected bool // prevent accidental deletion
}

// ResourceState represents the full state of a resource at a point in time
type ResourceState struct {
	ID          ResourceID     `json:"id"`
	Type        ResourceType   `json:"type"`
	Name        string         `json:"name"`
	Status      ResourceStatus `json:"status"`
	Inputs      Inputs         `json:"inputs"`
	Outputs     Outputs        `json:"outputs"`
	Tags        map[string]string `json:"tags,omitempty"`
	Protected   bool           `json:"protected,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ProviderID  string         `json:"provider_id"`
	ExternalID  string         `json:"external_id,omitempty"` // ID on the provider side
	Error       string         `json:"error,omitempty"`
}

// ─── Provider Interface ────────────────────────────────────────────────────────

// Provider is the interface all Gecko infrastructure providers must implement.
// FOSS providers include Kubernetes, Proxmox, Nomad, Gitea, etc.
type Provider interface {
	// Metadata
	Name() string
	Version() string
	SupportedTypes() []ResourceType

	// Lifecycle
	Configure(ctx context.Context, config map[string]interface{}) error
	Validate(ctx context.Context, args ResourceArgs) error

	// CRUD operations
	Create(ctx context.Context, args ResourceArgs) (*ResourceState, error)
	Read(ctx context.Context, id ResourceID, externalID string) (*ResourceState, error)
	Update(ctx context.Context, current *ResourceState, desired ResourceArgs) (*ResourceState, error)
	Delete(ctx context.Context, state *ResourceState) error

	// Import existing resource by external ID
	Import(ctx context.Context, resourceType ResourceType, externalID string) (*ResourceState, error)

	// Diff computes what would change if desired args were applied to current state
	Diff(ctx context.Context, current *ResourceState, desired ResourceArgs) (*Diff, error)
}

// ProviderFactory creates a provider instance
type ProviderFactory func() Provider

// ─── Diff ─────────────────────────────────────────────────────────────────────

// ChangeKind describes how a field will change
type ChangeKind string

const (
	ChangeAdd     ChangeKind = "add"
	ChangeUpdate  ChangeKind = "update"
	ChangeDelete  ChangeKind = "delete"
	ChangeNoOp    ChangeKind = "noop"
	ChangeReplace ChangeKind = "replace" // destroy+recreate required
)

// FieldChange describes a single field change
type FieldChange struct {
	Field    string
	Kind     ChangeKind
	OldValue interface{}
	NewValue interface{}
	Computed bool // value is not known until apply
}

// Diff represents the full set of changes for a resource
type Diff struct {
	ResourceID ResourceID
	Kind       ChangeKind // overall operation
	Changes    []FieldChange
	RequiresReplace bool
}

// HasChanges returns true if there are meaningful changes
func (d *Diff) HasChanges() bool {
	if d.Kind == ChangeNoOp {
		return false
	}
	return len(d.Changes) > 0 || d.Kind == ChangeAdd || d.Kind == ChangeDelete
}

// ─── Resource Graph ───────────────────────────────────────────────────────────

// Node is a resource in the dependency graph
type Node struct {
	ID       ResourceID
	Args     ResourceArgs
	State    *ResourceState
	Deps     []ResourceID
	Visited  bool
}

// Graph is a directed acyclic graph of resource dependencies
type Graph struct {
	mu    sync.RWMutex
	Nodes map[ResourceID]*Node
}

// NewGraph creates an empty resource graph
func NewGraph() *Graph {
	return &Graph{Nodes: make(map[ResourceID]*Node)}
}

// Add adds a resource node to the graph
func (g *Graph) Add(args ResourceArgs, state *ResourceState) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()

	id := ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name))
	node := &Node{
		ID:   id,
		Args: args,
		Deps: args.DependsOn,
	}
	if state != nil {
		node.State = state
	}
	g.Nodes[id] = node
	return node
}

// TopologicalSort returns nodes in dependency order (deps first)
func (g *Graph) TopologicalSort() ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Node
	visited := make(map[ResourceID]bool)
	inStack := make(map[ResourceID]bool)

	var visit func(id ResourceID) error
	visit = func(id ResourceID) error {
		if inStack[id] {
			return fmt.Errorf("circular dependency detected involving %s", id)
		}
		if visited[id] {
			return nil
		}
		inStack[id] = true
		node, ok := g.Nodes[id]
		if !ok {
			return fmt.Errorf("resource %s not found in graph", id)
		}
		for _, dep := range node.Deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		inStack[id] = false
		visited[id] = true
		result = append(result, node)
		return nil
	}

	// Sort keys for deterministic output
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	for _, id := range ids {
		if err := visit(ResourceID(id)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// ─── Stack ────────────────────────────────────────────────────────────────────

// Stack is a named, versioned collection of resources — the top-level Gecko unit
type Stack struct {
	Name      string
	Project   string
	Workspace string // e.g. "dev", "staging", "prod"
	Graph     *Graph
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewStack creates a new Gecko stack
func NewStack(project, name, workspace string) *Stack {
	return &Stack{
		Name:      name,
		Project:   project,
		Workspace: workspace,
		Graph:     NewGraph(),
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider to the stack
func (s *Stack) RegisterProvider(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[p.Name()] = p
}

// GetProvider returns a provider by name
func (s *Stack) GetProvider(name string) (Provider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.providers[name]
	return p, ok
}

// Resource declares a resource within this stack and returns its ID
func (s *Stack) Resource(args ResourceArgs) ResourceID {
	node := s.Graph.Add(args, nil)
	return node.ID
}

// ─── Engine ───────────────────────────────────────────────────────────────────

// PlanResult holds all diffs computed during a plan
type PlanResult struct {
	StackName string
	Workspace string
	Diffs     []*Diff
	Adds      int
	Updates   int
	Destroys  int
	NoOps     int
	CreatedAt time.Time
}

// ApplyResult holds the result of an apply operation
type ApplyResult struct {
	StackName  string
	Workspace  string
	Succeeded  []ResourceID
	Failed     []ResourceID
	Duration   time.Duration
	FinishedAt time.Time
}

// Engine orchestrates the plan and apply lifecycle
type Engine struct {
	stack        *Stack
	stateBackend StateBackend
}

// NewEngine creates a new execution engine for a stack
func NewEngine(stack *Stack, backend StateBackend) *Engine {
	return &Engine{stack: stack, stateBackend: backend}
}

// Plan computes the diff between desired and actual state
func (e *Engine) Plan(ctx context.Context) (*PlanResult, error) {
	result := &PlanResult{
		StackName: e.stack.Name,
		Workspace: e.stack.Workspace,
		CreatedAt: time.Now(),
	}

	nodes, err := e.stack.Graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	currentState, err := e.stateBackend.Load(ctx, e.stack.Name, e.stack.Workspace)
	if err != nil {
		currentState = &StackState{Resources: make(map[ResourceID]*ResourceState)}
	}

	for _, node := range nodes {
		current := currentState.Resources[node.ID]
		providerName := providerNameForType(node.Args.Type)
		p, ok := e.stack.GetProvider(providerName)
		if !ok {
			return nil, fmt.Errorf("no provider registered for %s", node.Args.Type)
		}

		diff, err := p.Diff(ctx, current, node.Args)
		if err != nil {
			return nil, fmt.Errorf("diff failed for %s: %w", node.ID, err)
		}

		result.Diffs = append(result.Diffs, diff)
		switch diff.Kind {
		case ChangeAdd:
			result.Adds++
		case ChangeUpdate, ChangeReplace:
			result.Updates++
		case ChangeDelete:
			result.Destroys++
		case ChangeNoOp:
			result.NoOps++
		}
	}

	// Also find resources in state that are NOT in graph (orphans → destroy)
	for id := range currentState.Resources {
		if _, exists := e.stack.Graph.Nodes[id]; !exists {
			result.Diffs = append(result.Diffs, &Diff{
				ResourceID: id,
				Kind:       ChangeDelete,
			})
			result.Destroys++
		}
	}

	return result, nil
}

// Apply executes all changes in plan order
func (e *Engine) Apply(ctx context.Context, plan *PlanResult, onProgress func(ResourceID, ResourceStatus)) (*ApplyResult, error) {
	start := time.Now()
	result := &ApplyResult{
		StackName: e.stack.Name,
		Workspace: e.stack.Workspace,
	}

	currentState, err := e.stateBackend.Load(ctx, e.stack.Name, e.stack.Workspace)
	if err != nil {
		currentState = &StackState{Resources: make(map[ResourceID]*ResourceState)}
	}

	for _, diff := range plan.Diffs {
		node, ok := e.stack.Graph.Nodes[diff.ResourceID]
		if !ok {
			continue
		}

		providerName := providerNameForType(node.Args.Type)
		p, ok := e.stack.GetProvider(providerName)
		if !ok {
			result.Failed = append(result.Failed, diff.ResourceID)
			continue
		}

		if onProgress != nil {
			onProgress(diff.ResourceID, StatusPending)
		}

		var newState *ResourceState
		var applyErr error

		switch diff.Kind {
		case ChangeAdd:
			if onProgress != nil {
				onProgress(diff.ResourceID, StatusCreating)
			}
			newState, applyErr = p.Create(ctx, node.Args)

		case ChangeUpdate:
			current := currentState.Resources[diff.ResourceID]
			if onProgress != nil {
				onProgress(diff.ResourceID, StatusUpdating)
			}
			newState, applyErr = p.Update(ctx, current, node.Args)

		case ChangeReplace:
			current := currentState.Resources[diff.ResourceID]
			if current != nil {
				applyErr = p.Delete(ctx, current)
			}
			if applyErr == nil {
				newState, applyErr = p.Create(ctx, node.Args)
			}

		case ChangeDelete:
			current := currentState.Resources[diff.ResourceID]
			if current != nil {
				applyErr = p.Delete(ctx, current)
			}
			delete(currentState.Resources, diff.ResourceID)
		}

		if applyErr != nil {
			result.Failed = append(result.Failed, diff.ResourceID)
			if node != nil {
				// Save partial failure state
				if currentState.Resources[diff.ResourceID] != nil {
					currentState.Resources[diff.ResourceID].Status = StatusFailed
					currentState.Resources[diff.ResourceID].Error = applyErr.Error()
				}
			}
			continue
		}

		if newState != nil {
			newState.UpdatedAt = time.Now()
			currentState.Resources[diff.ResourceID] = newState
		}
		result.Succeeded = append(result.Succeeded, diff.ResourceID)
		if onProgress != nil {
			onProgress(diff.ResourceID, StatusRunning)
		}

		// Save state after each resource for resilience
		_ = e.stateBackend.Save(ctx, e.stack.Name, e.stack.Workspace, currentState)
	}

	result.Duration = time.Since(start)
	result.FinishedAt = time.Now()
	return result, nil
}

// providerNameForType extracts the provider name from a resource type like "proxmox:vm"
func providerNameForType(t ResourceType) string {
	s := string(t)
	for i, c := range s {
		if c == ':' {
			return s[:i]
		}
	}
	return s
}

// ─── State Backend Interface ──────────────────────────────────────────────────

// StackState holds the full serialized state for a stack+workspace
type StackState struct {
	StackName string
	Workspace string
	Version   int
	Resources map[ResourceID]*ResourceState
	UpdatedAt time.Time
	Metadata  map[string]string
}

// StateBackend persists and retrieves stack state
type StateBackend interface {
	Load(ctx context.Context, stackName, workspace string) (*StackState, error)
	Save(ctx context.Context, stackName, workspace string, state *StackState) error
	Delete(ctx context.Context, stackName, workspace string) error
	List(ctx context.Context) ([]string, error) // list all stack+workspace pairs
	Lock(ctx context.Context, stackName, workspace string) (string, error)
	Unlock(ctx context.Context, stackName, workspace, lockID string) error
}
