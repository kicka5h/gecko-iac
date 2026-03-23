# Provider: Hostinger

> **Status: alpha** — API client implementation in progress

The `hostinger` provider manages VPS instances and domains via the Hostinger API.

## Resource types

| Type | Description |
|---|---|
| `hostinger:vps` | Virtual private server |
| `hostinger:domain` | Domain name registration and DNS |

## Habitat config

```scute
habitat "hostinger"
  api_token: env("HOSTINGER_API_TOKEN")
end
```

## Example

```scute
spawn "hostinger:vps" as "app-server"
  name:     "app-server"
  plan:     "kvm-2"
  os:       "ubuntu-22.04"
  region:   "eu-west"
  ssh_keys: ["deploy-key"]
end
```

## Building

See the [Building a Provider](Building-a-Provider) guide.
