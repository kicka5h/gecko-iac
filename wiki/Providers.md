# Providers

Gecko ships with providers for the best open-source infrastructure tools. Every provider implements the `core.Provider` interface and uses a lazy connection pattern — no network calls happen until `gecko grip` runs.

## Provider table

| Provider | Resource Types | Status | Notes |
|---|---|---|---|
| [kubernetes](Provider-Kubernetes) | 21 types | stable | Works with k8s, k3s, k0s, Talos, kubeadm |
| [kind](Provider-kind) | `kind:cluster` | stable | Local k8s clusters via Docker |
| [vault](Provider-Vault) | secret, policy, auth, mount | stable | HashiCorp Vault KV v2 + sys |
| [nomad](Provider-Nomad) | job, namespace, policy, volume | stable | HashiCorp Nomad clusters |
| [proxmox](Provider-Proxmox) | vm, lxc, storage, network | planned | Full Proxmox VE 8.x support |
| [gitea](Provider-Gitea) | repo, org, user, webhook, runner | planned | Self-hosted Git forge |
| [minio](Provider-MinIO) | bucket, policy, user | planned | S3-compatible object storage |
| [keycloak](Provider-Keycloak) | realm, client, user, role | planned | Identity and SSO |
| [wireguard](Provider-WireGuard) | network, peer | planned | VPN mesh networking |
| [nfs](Provider-NFS) | export, mount | planned | Network file systems |
| [postgresql](Provider-PostgreSQL) | database, role, extension | planned | Managed Postgres |

## Configuring providers

Providers are declared in `gecko.json` and configured in your `.scute` file via `habitat` blocks.

### gecko.json

```json
{
  "providers": {
    "k8s":   { "type": "kubernetes", "config": { "kubeconfig": "~/.kube/config" } },
    "vault": { "type": "vault",      "config": { "address": "https://vault.local:8200" } },
    "nomad": { "type": "nomad",      "config": { "address": "http://nomad.local:4646" } }
  }
}
```

### .scute habitat blocks

```scute
habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "homelab"
end

habitat "vault"
  address: "https://vault.local:8200"
  token:   env("VAULT_TOKEN")
end
```

## The lazy connect pattern

Every provider follows the same pattern:

- `Configure()` — stores config only, makes no network calls
- `connect()` — called internally on first Create/Read/Update/Delete; builds the actual client

This means `gecko crawl` can show a full plan even when the infrastructure doesn't exist yet (e.g. before `gecko grip` creates a kind cluster).

## Building a provider

See [Building a Provider](Building-a-Provider) for a full walkthrough of implementing `core.Provider`.
