package core

import (
	"context"
	"testing"
	"time"
)

// stubProvider implements Provider with a controllable Read.
type stubProvider struct {
	name string
	read func(ctx context.Context, id ResourceID, externalID string) (*ResourceState, error)
}

func (s *stubProvider) Name() string                    { return s.name }
func (s *stubProvider) Version() string                 { return "test" }
func (s *stubProvider) SupportedTypes() []ResourceType  { return nil }
func (s *stubProvider) Configure(context.Context, map[string]interface{}) error { return nil }
func (s *stubProvider) Validate(context.Context, ResourceArgs) error            { return nil }
func (s *stubProvider) Create(context.Context, ResourceArgs) (*ResourceState, error) {
	return nil, nil
}
func (s *stubProvider) Read(ctx context.Context, id ResourceID, externalID string) (*ResourceState, error) {
	return s.read(ctx, id, externalID)
}
func (s *stubProvider) Update(context.Context, *ResourceState, ResourceArgs) (*ResourceState, error) {
	return nil, nil
}
func (s *stubProvider) Delete(context.Context, *ResourceState) error { return nil }
func (s *stubProvider) Import(context.Context, ResourceType, string) (*ResourceState, error) {
	return nil, nil
}
func (s *stubProvider) Diff(context.Context, *ResourceState, ResourceArgs) (*Diff, error) {
	return &Diff{Kind: ChangeNoOp}, nil
}

// memBackend is an in-memory StateBackend.
type memBackend struct {
	state *StackState
	saved int
}

func (m *memBackend) Load(context.Context, string, string) (*StackState, error) {
	return m.state, nil
}
func (m *memBackend) Save(_ context.Context, _, _ string, s *StackState) error {
	m.state = s
	m.saved++
	return nil
}
func (m *memBackend) Delete(context.Context, string, string) error { return nil }
func (m *memBackend) List(context.Context) ([]string, error)       { return nil, nil }
func (m *memBackend) Lock(context.Context, string, string) (string, error) {
	return "lock", nil
}
func (m *memBackend) Unlock(context.Context, string, string, string) error { return nil }

func refreshFixture(read func(ctx context.Context, id ResourceID, externalID string) (*ResourceState, error)) (*Engine, *memBackend) {
	stack := NewStack("proj", "teststack", "dev")
	stack.RegisterProvider(&stubProvider{name: "proxmox", read: read})
	backend := &memBackend{
		state: &StackState{
			StackName: "teststack",
			Workspace: "dev",
			Resources: map[ResourceID]*ResourceState{
				"proxmox:vm::web-01": {
					ID:         "proxmox:vm::web-01",
					Type:       "proxmox:vm",
					Name:       "web-01",
					Status:     StatusRunning,
					ExternalID: "100",
					Inputs:     Inputs{"memory": 2048, "cores": 2},
					UpdatedAt:  time.Now(),
				},
			},
		},
	}
	return NewEngine(stack, backend), backend
}

func TestRefresh_NoDrift_KeepsStatus(t *testing.T) {
	engine, backend := refreshFixture(func(_ context.Context, id ResourceID, _ string) (*ResourceState, error) {
		return &ResourceState{ID: id, Inputs: Inputs{"memory": 2048, "cores": 2, "extra": "ignored"}}, nil
	})

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Checked != 1 {
		t.Errorf("want 1 checked, got %d", result.Checked)
	}
	if len(result.Drifted) != 0 {
		t.Errorf("want no drift, got %+v", result.Drifted)
	}
	if backend.state.Resources["proxmox:vm::web-01"].Status != StatusRunning {
		t.Errorf("status should stay running, got %v", backend.state.Resources["proxmox:vm::web-01"].Status)
	}
}

func TestRefresh_ChangedField_MarksDrifted(t *testing.T) {
	engine, backend := refreshFixture(func(_ context.Context, id ResourceID, _ string) (*ResourceState, error) {
		return &ResourceState{ID: id, Inputs: Inputs{"memory": 4096, "cores": 2}}, nil
	})

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Drifted) != 1 {
		t.Fatalf("want 1 drifted, got %d", len(result.Drifted))
	}
	drift := result.Drifted[0]
	if drift.Missing {
		t.Error("resource should not be marked missing")
	}
	if len(drift.Changes) != 1 || drift.Changes[0].Field != "memory" {
		t.Fatalf("want memory change, got %+v", drift.Changes)
	}
	if backend.state.Resources["proxmox:vm::web-01"].Status != StatusDrifted {
		t.Errorf("want StatusDrifted persisted, got %v", backend.state.Resources["proxmox:vm::web-01"].Status)
	}
	if backend.saved == 0 {
		t.Error("refreshed state was not saved")
	}
}

func TestRefresh_MissingResource_MarksDrifted(t *testing.T) {
	engine, backend := refreshFixture(func(context.Context, ResourceID, string) (*ResourceState, error) {
		return nil, nil // gone at the provider
	})

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Drifted) != 1 || !result.Drifted[0].Missing {
		t.Fatalf("want 1 missing drift, got %+v", result.Drifted)
	}
	rs := backend.state.Resources["proxmox:vm::web-01"]
	if rs.Status != StatusDrifted {
		t.Errorf("want StatusDrifted, got %v", rs.Status)
	}
	if rs.Error == "" {
		t.Error("want error message recorded on missing resource")
	}
}

func TestRefresh_DriftResolved_RestoresRunning(t *testing.T) {
	engine, backend := refreshFixture(func(_ context.Context, id ResourceID, _ string) (*ResourceState, error) {
		return &ResourceState{ID: id, Inputs: Inputs{"memory": 2048, "cores": 2}}, nil
	})
	backend.state.Resources["proxmox:vm::web-01"].Status = StatusDrifted

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Drifted) != 0 {
		t.Fatalf("want no drift, got %+v", result.Drifted)
	}
	if backend.state.Resources["proxmox:vm::web-01"].Status != StatusRunning {
		t.Errorf("want status restored to running, got %v", backend.state.Resources["proxmox:vm::web-01"].Status)
	}
}

func TestRefresh_TypeMismatch_ComparesLoosely(t *testing.T) {
	// JSON round-trips turn ints into float64; that must not read as drift.
	engine, _ := refreshFixture(func(_ context.Context, id ResourceID, _ string) (*ResourceState, error) {
		return &ResourceState{ID: id, Inputs: Inputs{"memory": float64(2048), "cores": float64(2)}}, nil
	})

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Drifted) != 0 {
		t.Errorf("float64/int equivalence should not drift, got %+v", result.Drifted)
	}
}

func TestRefresh_NoProvider_RecordsError(t *testing.T) {
	engine, _ := refreshFixture(func(_ context.Context, id ResourceID, _ string) (*ResourceState, error) {
		return &ResourceState{ID: id, Inputs: Inputs{}}, nil
	})
	// Sneak in a resource whose provider is not registered.
	backend := engine.stateBackend.(*memBackend)
	backend.state.Resources["unknown:thing::x"] = &ResourceState{
		ID: "unknown:thing::x", Type: "unknown:thing", Inputs: Inputs{},
	}

	result, err := engine.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("want 1 error for unregistered provider, got %+v", result.Errors)
	}
}
