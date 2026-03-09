# Provider: WireGuard

> **Status: planned** — tracked in [issue #8](https://github.com/kicka5h/gecko-iac/issues/8)

The `wireguard` provider will manage VPN interfaces and peers by writing WireGuard config files and calling `wg`/`wg-quick`.

## Planned resource types

| Type | Description |
|---|---|
| `wireguard:network` | WireGuard interface with keypair and address |
| `wireguard:peer` | Peer entry with allowed IPs and endpoint |

## Planned habitat config

```scute
habitat "wireguard"
  config_dir: "/etc/wireguard"
  interface:  "wg0"
end
```

## Want to help?

See [issue #8](https://github.com/kicka5h/gecko-iac/issues/8) and the [Building a Provider](Building-a-Provider) guide.
