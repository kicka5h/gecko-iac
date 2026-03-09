# Provider: Proxmox

> **Status: planned** — tracked in [issue #4](https://github.com/kicka5h/gecko-iac/issues/4)

The `proxmox` provider will manage VMs, LXC containers, storage, and networks via the Proxmox VE REST API (`/api2/json/`).

## Planned resource types

| Type | Description |
|---|---|
| `proxmox:vm` | QEMU virtual machine |
| `proxmox:lxc` | LXC container |
| `proxmox:storage` | Storage pool definition |
| `proxmox:network` | Linux bridge or VLAN interface |

## Planned habitat config

```scute
habitat "proxmox"
  endpoint:     "https://proxmox.local:8006"
  token_id:     env("PROXMOX_TOKEN_ID")
  token_secret: env("PROXMOX_TOKEN_SECRET")
  node:         "pve"
  insecure:     false
end
```

## Want to help?

See [issue #4](https://github.com/kicka5h/gecko-iac/issues/4) and the [Building a Provider](Building-a-Provider) guide.
