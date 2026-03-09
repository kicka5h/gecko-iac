# Provider: kind

The `kind` provider creates and manages local Kubernetes clusters using [kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker). It is primarily used for local development and CI environments.

## Prerequisites

- Docker Desktop or Docker Engine
- `kind` CLI: `brew install kind`

## Configuration

### gecko.json

```json
{
  "providers": {
    "kind": { "type": "kind" }
  }
}
```

### habitat

```scute
habitat "kind"
end
```

No configuration fields are required. The provider verifies that `kind` is in `PATH` on first use.

## Supported resource types

| Type | Description |
|---|---|
| `kind:cluster` | A local Kubernetes cluster managed by kind |

## Resource: kind:cluster

| Field | Type | Default | Description |
|---|---|---|---|
| `name` | string | required | Cluster name (also becomes the kubectl context `kind-<name>`) |
| `config` | string | — | Path to a kind config YAML file (for custom node counts, port mappings, etc.) |

## Example

```scute
habitat "kind"
end

habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "kind-gecko-test"
end

spawn "kind:cluster" as "cluster"
  name: "gecko-test"
end

spawn "k8s:namespace" as "app-ns"
  needs: @cluster
  name:  "my-app"
end
```

## Behavior

- **Create**: runs `kind create cluster --name <name> --wait 60s`. If a cluster with that name already exists, it is adopted into state without recreation.
- **Delete**: runs `kind delete cluster --name <name>`.
- **Drift detection**: checks `kind get clusters` to see if the cluster exists. If it was deleted externally, Gecko reports drift on next `gecko crawl`.

## Tips

- The kubectl context created by kind is always `kind-<cluster-name>`. Set your `k8s` habitat `context:` to match.
- kind clusters are ephemeral — `docker desktop restart` may destroy them. Run `gecko grip` again to recreate.
- For CI, kind is ideal. For persistent homelab use, prefer a real cluster (k3s, k0s, Talos) with the [Kubernetes provider](Provider-Kubernetes).
