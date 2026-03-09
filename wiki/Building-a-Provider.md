# Building a Provider

This guide walks through implementing a new Gecko provider from scratch.

## The core.Provider interface

Every provider implements this interface (`internal/core/provider.go`):

```go
type Provider interface {
    Name()           string
    Version()        string
    SupportedTypes() []ResourceType

    Configure(ctx context.Context, config map[string]interface{}) error
    Validate(ctx context.Context, args ResourceArgs) error

    Create(ctx context.Context, args ResourceArgs) (*ResourceState, error)
    Read(ctx context.Context, id ResourceID, externalID string) (*ResourceState, error)
    Update(ctx context.Context, current *ResourceState, desired ResourceArgs) (*ResourceState, error)
    Delete(ctx context.Context, state *ResourceState) error

    Import(ctx context.Context, resourceType ResourceType, externalID string) (*ResourceState, error)
    Diff(ctx context.Context, current *ResourceState, desired ResourceArgs) (*Diff, error)
}
```

## Lazy connect pattern

**Never build the API client in `Configure()`**. Instead, store config in `Configure` and build the client lazily in a `connect()` method. This allows `gecko crawl` to show a full plan even when the remote system doesn't exist yet.

```go
type MyProvider struct {
    endpoint string
    token    string
    client   *myapi.Client // nil until connect() is called
}

func (p *MyProvider) Configure(ctx context.Context, config map[string]interface{}) error {
    if v, ok := config["endpoint"].(string); ok {
        p.endpoint = v
    }
    if v, ok := config["token"].(string); ok {
        p.token = v
    }
    return nil // no network calls here
}

func (p *MyProvider) connect() error {
    if p.client != nil {
        return nil // already connected
    }
    c, err := myapi.NewClient(p.endpoint, p.token)
    if err != nil {
        return err
    }
    p.client = c
    return nil
}
```

## Diff implementation

`Diff()` is called during `gecko crawl`. It must handle the case where `current == nil` (resource doesn't exist yet):

```go
func (p *MyProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
    if current == nil {
        return &core.Diff{ChangeType: core.ChangeAdd}, nil
    }
    // Compare current outputs/inputs with desired inputs
    if needsUpdate(current, desired) {
        return &core.Diff{ChangeType: core.ChangeUpdate}, nil
    }
    return &core.Diff{ChangeType: core.ChangeNoOp}, nil
}
```

## ResourceState

`Create`, `Read`, and `Update` all return a `*core.ResourceState`:

```go
return &core.ResourceState{
    ID:         core.ResourceID(fmt.Sprintf("myprovider:thing::%s", name)),
    Type:       args.Type,
    Provider:   "myprovider",
    ExternalID: name,
    Inputs:     args.Inputs,
    Outputs: core.Outputs{
        "id":         id,
        "name":       name,
        "created_at": time.Now().Format(time.RFC3339),
    },
}, nil
```

`ExternalID` is the identifier used by `Read`, `Update`, `Delete`, and `Import` — usually the remote resource's name or ID.

## Full skeleton

```go
package mypkg

import (
    "context"
    "fmt"

    "github.com/gecko-iac/gecko/internal/core"
)

type MyProvider struct {
    endpoint string
    client   *myapi.Client
}

func NewMyProvider(config map[string]interface{}) *MyProvider {
    p := &MyProvider{}
    if config != nil {
        if v, ok := config["endpoint"].(string); ok {
            p.endpoint = v
        }
    }
    return p
}

func (p *MyProvider) Name()    string { return "myprovider" }
func (p *MyProvider) Version() string { return "0.1.0" }
func (p *MyProvider) SupportedTypes() []core.ResourceType {
    return []core.ResourceType{"myprovider:thing"}
}

func (p *MyProvider) Configure(ctx context.Context, config map[string]interface{}) error {
    if config != nil {
        if v, ok := config["endpoint"].(string); ok { p.endpoint = v }
    }
    return nil
}

func (p *MyProvider) connect() error {
    if p.client != nil { return nil }
    c, err := myapi.NewClient(p.endpoint)
    if err != nil { return fmt.Errorf("myprovider connect: %w", err) }
    p.client = c
    return nil
}

func (p *MyProvider) Validate(ctx context.Context, args core.ResourceArgs) error {
    if args.Inputs["name"] == nil {
        return fmt.Errorf("myprovider:%s requires 'name'", args.Type)
    }
    return nil
}

func (p *MyProvider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
    if err := p.connect(); err != nil { return nil, err }
    name := args.Inputs["name"].(string)
    // ... create the resource
    return &core.ResourceState{
        ID:         core.ResourceID(fmt.Sprintf("myprovider:thing::%s", name)),
        Type:       args.Type,
        Provider:   "myprovider",
        ExternalID: name,
        Inputs:     args.Inputs,
        Outputs:    core.Outputs{"name": name},
    }, nil
}

func (p *MyProvider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
    if err := p.connect(); err != nil { return nil, err }
    // ... fetch by externalID
    return nil, nil // return nil, nil if not found (triggers ChangeAdd on next plan)
}

func (p *MyProvider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
    if err := p.connect(); err != nil { return nil, err }
    // ... update
    return current, nil
}

func (p *MyProvider) Delete(ctx context.Context, state *core.ResourceState) error {
    if err := p.connect(); err != nil { return err }
    // ... delete by state.ExternalID
    return nil
}

func (p *MyProvider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
    if err := p.connect(); err != nil { return nil, err }
    return p.Read(ctx, "", externalID)
}

func (p *MyProvider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
    if current == nil {
        return &core.Diff{ChangeType: core.ChangeAdd}, nil
    }
    return &core.Diff{ChangeType: core.ChangeNoOp}, nil
}
```

## Registering the provider

After implementing, register it in three files:

**`cmd/crawl.go`**, **`cmd/grip.go`**, **`cmd/observe.go`** — add a case to the provider switch:

```go
case "myprovider":
    p := mypkg.NewMyProvider(hint.Config)
    if err := p.Configure(ctx, hint.Config); err != nil {
        ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
    }
    eng.RegisterProvider(p)
```

## PR checklist

See the [provider checklist](Contributing#provider-checklist) before opening a PR.
