# 🦎 Gecko — FOSS Infrastructure as Code

> *Move fast. Grip hard.*

**Alpha release: Amalosia** — named after *Amalosia*, the first genus in the family Diplodactylidae.

Gecko is a **FOSS-first Infrastructure as Code framework** for teams who believe infrastructure tooling should be open, composable, and opinionated in the best possible ways.

Built in Go. No phone-home telemetry. No proprietary providers. No cloud vendor lock-in.

---

## Command Reference

Gecko's CLI is themed around gecko anatomy and behavior. Every command maps to something a real gecko does.

| Command | Alias | What It Does |
|---|---|---|
| `gecko hatch` | `init` | 🥚 Initialize a new project |
| `gecko crawl` | `plan` | 🦶 Preview changes (dry run) |
| `gecko grip` | `apply` |  🤲 Apply infrastructure changes |
| `gecko molt` | `destroy` | 💀 Tear down all infrastructure |
| `gecko bask` | `status` | ☀️ Infrastructure status dashboard |
| `gecko tail` | `logs` | 🔍  Stream live logs |
| `gecko lick` | `inspect` | 👅 Deep-inspect a resource |
| `gecko shed` | `upgrade` | 🐍 Upgrade providers/framework |
| `gecko hide` | `secrets` | 🫙 Manage encrypted secrets |
| `gecko burrow` | `import` | 🕳️ Import existing resources |
| `gecko scale` | `resize` | 📊 Scale resources up/down |
| `gecko clutch` | `workspace` | 🥚 Manage stacks/workspaces |
| `gecko cross` | `bridge` | 🌉 Import Terraform/Pulumi state into Scute |

---

## Install

**Homebrew (macOS / Linux):**
```bash
brew install gecko-iac/gecko/gecko
```

**curl (macOS / Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/gecko-iac/gecko/main/scripts/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/gecko-iac/gecko/main/scripts/install.ps1 | iex
```

**Go:**
```bash
go install github.com/gecko-iac/gecko@latest
```

**Download a binary:** [GitHub Releases](https://github.com/gecko-iac/gecko/releases/latest)

---

## Quick Start

```bash
# Install
go install github.com/gecko-iac/gecko@latest

# Create a new project with Proxmox + Fly.io
gecko hatch my-homelab --providers proxmox,fly --workspace dev

# Preview what Gecko would do
gecko crawl

# Apply your infrastructure
gecko grip

# Watch live logs
gecko tail --resource fly:machine.web --follow

# Check status
gecko bask

# Inspect a specific resource
gecko lick proxmox:vm.web-01
```

---

## Project Structure

After `gecko hatch`, your project looks like:

```
my-homelab/
├── gecko.json              # Project config: providers, backend, workspaces
├── .geckoignore            # Files to exclude from gecko operations
├── stacks/
│   ├── dev/
│   │   └── main.scute     # Dev workspace stack declaration
│   ├── staging/
│   │   └── main.scute
│   └── prod/
│       └── main.scute
├── modules/                # Reusable Snack modules
│   ├── monitoring/
│   └── networking/
└── .gecko/
    ├── state/              # Local state files (switch to remote for teams)
    └── plans/              # Saved crawl plans
```

---

## Stack Declaration

Gecko stacks are written in **Scute** (`.scute`), Gecko's own declarative configuration language:

```scute
# stacks/dev/main.scute

territory "my-homelab"
  workspace: "dev"
end

# Provider configuration
habitat "proxmox"
  endpoint:     env("PROXMOX_ENDPOINT")
  token_id:     env("PROXMOX_TOKEN_ID")
  token_secret: env("PROXMOX_TOKEN_SECRET")
  node:         "pve"
  insecure:     true
end

habitat "fly"
  api_token: env("FLY_API_TOKEN")
  org:       "personal"
  region:    "sjc"
end

# Input variables (overridable via GECKO_VAR_* env or --var flag)
mark vm_memory number: 2048
mark base_domain string: "homelab.local"

# Computed locals
camouflage
  is_prod:   workspace == "prod"
  vm_cores:  is_prod ? 4 : 2
end

# Network bridge for VMs
spawn "proxmox:network" as "vm-bridge"
  iface:        "vmbr1"
  type:         "bridge"
  bridge_ports: "eth1"
  autostart:    true
  comments:     "VM bridge managed by gecko"
end

# Web server VM
spawn "proxmox:vm" as "web-01"
  needs:     @vm-bridge
  name:      "web-01"
  memory:    vm_memory
  cores:     vm_cores
  disk_size: "20G"
  storage:   "local-lvm"
  iso:       "local:iso/debian-12-amd64-netinst.iso"
  net0:      "virtio,bridge=vmbr1"
  start:     false
end

# Fly.io app for the public-facing API
spawn "fly:app" as "api"
  name: "my-homelab-api"
  org:  "personal"
end

spawn "fly:machine" as "api-web"
  needs:  @api
  app:    @api.name
  image:  "registry.fly.io/my-api:latest"
  size:   "shared-cpu-1x"
  region: "sjc"
  memory: 256
end

# Stack outputs
signal "vm_name"
  value:       @web-01.name
  description: "Proxmox VM name"
end

signal "api_url"
  value:       "https://my-homelab-api.fly.dev"
  description: "Public API endpoint"
end
```

---

## FOSS Provider Ecosystem

Gecko ships with providers for open-source cloud platforms:

| Provider | Resources | Notes |
|---|---|---|
| **proxmox** | VMs, LXC, storage, networks | Full Proxmox VE 8.x support |
| **fly** | Apps, machines, volumes, secrets | Fly.io Machines API |
| **openstack** | Instances, networks, subnets, security groups, volumes | OpenStack IaaS |
| **hostinger** | VPS, domains | Hostinger VPS hosting |
| **ubicloud** | VMs, firewalls, subnets | Open-source cloud platform |
| **opennebula** | VMs, vnets, images, templates | Cloud and edge computing |

---

## State Backends

State can be stored anywhere:

```json
// gecko.json
{
  "backend": {
    "type": "local"
  }
}
```

```json
// Remote: S3-compatible (MinIO works!)
{
  "backend": {
    "type": "s3",
    "config": {
      "endpoint": "https://minio.local:9000",
      "bucket": "gecko-state",
      "prefix": "my-homelab/"
    }
  }
}
```

Supported: `local`, `s3` (+ MinIO), `gcs`, `etcd`, `postgres`

---

## Secrets

```bash
# Store an encrypted secret
gecko hide set --key grafana.admin.password --value supersecret

# List all secrets
gecko hide

# Use in your stack (.scute)
secret("grafana.admin.password")
```

Secrets are encrypted with AES-256-GCM. Environment variable fallback: `GECKO_SECRET_<KEY>`.

---

## Workspaces

```bash
# List workspaces
gecko clutch

# Create a new workspace
gecko clutch new --name production

# Switch active workspace
gecko clutch select production

# Plan against a specific workspace without switching
gecko crawl --workspace staging
```

---

## Crossing from Terraform or Pulumi

Already running Terraform or Pulumi? `gecko cross` reads your existing state file and generates equivalent Scute declarations — letting you adopt Gecko incrementally without starting from scratch.

```bash
# Import a Terraform state file
gecko cross --from terraform.tfstate --out stacks/dev/main.scute

# Import a Pulumi stack state
gecko cross --from .pulumi/stacks/dev.json --out stacks/dev/main.scute

# Auto-detect format, print to stdout to preview
gecko cross --from terraform.tfstate

# Override project name and workspace
gecko cross --from prod.tfstate --project my-homelab --workspace prod --out stacks/prod/main.scute
```

Resources with known Gecko equivalents are emitted as `spawn` blocks. Resources from cloud-specific providers (e.g. `aws_s3_bucket`) have no Gecko mapping and are preserved as commented-out blocks so nothing is silently dropped.

**Supported source types:**

| Source | Covered providers |
| --- | --- |
| Terraform | `proxmox`, `fly`, `openstack`, `opennebula`, `gitea`, `minio`, `postgresql` |
| Pulumi | `openstack` |

After generating the file, review it, fill in your `habitat` credentials, then run `gecko crawl` to see the diff and `gecko grip` to take ownership.

---

## Multi-Runtime Support

**Scute is the primary and recommended language.** For teams that prefer to embed infrastructure declarations in existing codebases, Gecko also provides SDKs:

```python
# Python SDK (pip install gecko-iac)
import gecko

stack = gecko.Stack(name="my-homelab", workspace="dev")
stack.register_provider(gecko.providers.proxmox(
    endpoint="https://proxmox.local:8006",
    token_id="root@pam!gecko",
    token_secret=os.environ["PROXMOX_TOKEN_SECRET"],
    node="pve",
))

net = stack.resource("proxmox:network", name="vm-bridge", inputs={
    "iface": "vmbr1",
    "type":  "bridge",
    "autostart": True,
})

stack.resource("proxmox:vm", name="web-01", depends_on=[net], inputs={
    "name":   "web-01",
    "memory": 2048,
    "cores":  2,
    "net0":   "virtio,bridge=vmbr1",
})
```

```typescript
// TypeScript SDK (npm install @gecko-iac/gecko)
import * as gecko from "@gecko-iac/gecko";

const stack = new gecko.Stack({ name: "my-homelab", workspace: "dev" });
stack.registerProvider(new gecko.providers.fly.Provider({
  apiToken: process.env.FLY_API_TOKEN,
  org: "personal",
  region: "sjc",
}));

const app = stack.resource("fly:app", {
  name: "my-api",
  inputs: { name: "my-api", org: "personal" },
});

stack.resource("fly:machine", {
  name: "web",
  dependsOn: [app],
  inputs: { app: "my-api", image: "registry.fly.io/my-api:latest", size: "shared-cpu-1x" },
});
```

All three runtimes produce the same resource graph and share the same state format, providers, and backends.

---

## Architecture Overview

```text
┌─────────────────────────────────────────────────┐
│                  gecko CLI                       │
│  hatch  crawl  grip  molt  bask  tail  lick ...  │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│              Gecko Engine                        │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Graph   │  │  Planner │  │  Applier      │  │
│  │ (DAG)    │  │  (Diff)  │  │  (Lifecycle)  │  │
│  └──────────┘  └──────────┘  └───────────────┘  │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│              State Backend                       │
│         local  /  s3  /  etcd  /  postgres       │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│              Provider Layer                      │
│  proxmox  fly  openstack  hostinger  ubicloud  opennebula │
└─────────────────────────────────────────────────┘
```

---

## Building Providers

Implement the `core.Provider` interface:

```go
type Provider interface {
    Name() string
    Version() string
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

---

## Design Principles

1. **FOSS-first** — every provider targets open-source infrastructure
2. **Multi-runtime** — Go, Python, TypeScript, HCL are all first-class
3. **Dependency graph** — declare resources, Gecko figures out the order
4. **Drift detection** — Gecko continuously reconciles declared vs actual state
5. **State resilience** — state saved after each resource, never lost on partial failure
6. **Zero lock-in** — state format is plain JSON, providers are plugins

---

*🦎 Infrastructure that grips, scales, and never lets go.*
