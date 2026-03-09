# 🦎 Gecko — FOSS Infrastructure as Code

> *Move fast. Grip hard.*

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

## Quick Start

```bash
# Install
go install github.com/gecko-iac/gecko@latest

# Create a new project with Kubernetes + Proxmox
gecko hatch my-homelab --providers k8s,proxmox --workspace dev

# Preview what Gecko would do
gecko crawl

# Apply your infrastructure
gecko grip

# Watch live logs
gecko tail --resource k8s:deployment.api-server --follow

# Check status
gecko bask

# Inspect a specific resource
gecko lick k8s:deployment.api-server
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
│   │   └── main.go        # Dev workspace stack declaration
│   ├── staging/
│   │   └── main.go
│   └── prod/
│       └── main.go
├── modules/                # Reusable infrastructure modules
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
habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "homelab"
end

# Input variables (overridable via GECKO_VAR_* env or --var flag)
mark replicas number: 1
mark base_domain string: "homelab.local"

# Computed locals
camouflage
  is_prod:     workspace == "prod"
  app_replicas: is_prod ? 3 : replicas
end

# Kubernetes namespace
spawn "k8s:namespace" as "monitoring-ns"
  name:   "monitoring"
  labels:
    managed-by: "gecko"
    env:        workspace
  end
end

# Prometheus — depends on the namespace via @monitoring-ns
spawn "k8s:deployment" as "prometheus"
  needs:     @monitoring-ns
  namespace: @monitoring-ns.name
  image:     "prom/prometheus:v2.50.0"
  replicas:  app_replicas
  ports:     [9090]
end

spawn "k8s:service" as "prometheus-svc"
  needs:     @prometheus
  namespace: @monitoring-ns.name
  port:      9090
end

# Grafana — references Prometheus outputs
spawn "k8s:deployment" as "grafana"
  needs:     @prometheus
  namespace: @monitoring-ns.name
  image:     "grafana/grafana:10.3.0"
  replicas:  app_replicas
  ports:     [3000]

  env:
    GF_SECURITY_ADMIN_PASSWORD: secret("grafana.admin.password")
    GF_SERVER_ROOT_URL:         "https://grafana.#{base_domain}"
  end

  when is_prod
    resources:
      memory: "512Mi"
      cpu:    "500m"
    end
  else
    resources:
      memory: "256Mi"
      cpu:    "250m"
    end
  end
end

# Stack outputs
signal "grafana_url"
  value:       "https://grafana.#{base_domain}"
  description: "Grafana dashboard endpoint"
end

signal "prometheus_url"
  value: "http://#{@prometheus-svc.cluster_ip}:9090"
end
```

---

## FOSS Provider Ecosystem

Gecko ships with providers for the best open-source infrastructure tools:

| Provider | Resources | Notes |
|---|---|---|
| **kubernetes** | 22 resource types | Works with k3s, k0s, Talos, kubeadm |
| **proxmox** | VMs, LXC, storage, networks | Full Proxmox VE 8.x support |
| **nomad** | Jobs, namespaces, volumes | HashiCorp Nomad clusters |
| **gitea** | Repos, orgs, users, webhooks, runners | Self-hosted Git forge |
| **minio** | Buckets, policies, users | S3-compatible object storage |
| **vault** | Secrets, policies, auth methods | HashiCorp Vault |
| **keycloak** | Realms, clients, users, roles | Identity and SSO |
| **wireguard** | Peers, networks | VPN mesh networking |
| **nfs** | Exports, mounts | Network file systems |
| **postgresql** | Databases, roles, extensions | Managed Postgres |

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
| Terraform | `hashicorp/kubernetes`, `hashicorp/vault`, `hashicorp/nomad`, `gitea`, `minio`, `postgresql` |
| Pulumi | `kubernetes`, `vault`, `nomad` |

After generating the file, review it, fill in your `habitat` credentials, then run `gecko crawl` to see the diff and `gecko grip` to take ownership.

---

## Multi-Runtime Support

**Scute is the primary and recommended language.** For teams that prefer to embed infrastructure declarations in existing codebases, Gecko also provides SDKs:

```python
# Python SDK (pip install gecko-iac)
import gecko

stack = gecko.Stack(name="my-homelab", workspace="dev")
stack.register_provider(gecko.providers.k8s(kubeconfig="~/.kube/config"))

ns = stack.resource("k8s:namespace", name="monitoring", inputs={
    "name": "monitoring",
    "labels": {"managed-by": "gecko"},
})

stack.resource("k8s:deployment", name="prometheus", depends_on=[ns], inputs={
    "namespace": "monitoring",
    "image":     "prom/prometheus:v2.50.0",
    "replicas":  1,
    "ports":     [9090],
})
```

```typescript
// TypeScript SDK (npm install @gecko-iac/gecko)
import * as gecko from "@gecko-iac/gecko";

const stack = new gecko.Stack({ name: "my-homelab", workspace: "dev" });
stack.registerProvider(new gecko.providers.k8s.Provider({ kubeconfig: "~/.kube/config" }));

const ns = stack.resource("k8s:namespace", {
  name: "monitoring",
  inputs: { name: "monitoring", labels: { "managed-by": "gecko" } },
});

stack.resource("k8s:deployment", {
  name: "prometheus",
  dependsOn: [ns],
  inputs: { namespace: "monitoring", image: "prom/prometheus:v2.50.0", replicas: 1 },
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
│   k8s   proxmox   nomad   gitea   minio   vault  │
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
