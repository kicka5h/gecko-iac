# Providers

Gecko ships with providers for the best open-source infrastructure tools. Every provider implements the `core.Provider` interface and uses a lazy connection pattern — no network calls happen until `gecko grip` runs.

## Provider table

| Provider | Resource Types | Status | Notes |
|---|---|---|---|
| [proxmox](Provider-Proxmox) | vm, lxc, storage, network | stable | Full Proxmox VE 8.x support |
| [fly](Provider-Fly) | app, machine, volume, secret | alpha | Fly.io Machines API |
| [openstack](Provider-OpenStack) | instance, network, subnet, security_group, volume | alpha | OpenStack IaaS |
| [hostinger](Provider-Hostinger) | vps, domain | alpha | Hostinger VPS hosting |
| [ubicloud](Provider-Ubicloud) | vm, firewall, subnet | alpha | Open-source cloud platform |
| [opennebula](Provider-OpenNebula) | vm, vnet, image, template | alpha | Cloud and edge computing |

## Configuring providers

Providers are declared in `gecko.json` and configured in your `.scute` file via `habitat` blocks.

### gecko.json

```json
{
  "providers": {
    "proxmox": { "type": "proxmox", "config": { "endpoint": "https://pve.local:8006", "token_id": "gecko@pam!iac", "token_secret": "env:PROXMOX_TOKEN" } },
    "fly":     { "type": "fly",     "config": { "org": "personal", "token": "env:FLY_API_TOKEN" } }
  }
}
```

### .scute habitat blocks

```scute
habitat "proxmox"
  endpoint:     "https://pve.local:8006"
  token_id:     "gecko@pam!iac"
  token_secret: env("PROXMOX_TOKEN")
end

habitat "fly"
  org:   "personal"
  token: env("FLY_API_TOKEN")
end
```

## The lazy connect pattern

Every provider follows the same pattern:

- `Configure()` — stores config only, makes no network calls
- `connect()` — called internally on first Create/Read/Update/Delete; builds the actual client

This means `gecko crawl` can show a full plan even when the infrastructure doesn't exist yet (e.g. before `gecko grip` provisions a Proxmox VM).

## Building a provider

See [Building a Provider](Building-a-Provider) for a full walkthrough of implementing `core.Provider`.
