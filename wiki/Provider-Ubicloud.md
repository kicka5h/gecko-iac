# Provider: Ubicloud

> **Status: alpha** — API client implementation in progress

The `ubicloud` provider manages virtual machines, firewalls, and subnets via the Ubicloud API.

## Resource types

| Type | Description |
|---|---|
| `ubicloud:vm` | Virtual machine |
| `ubicloud:firewall` | Firewall with ingress/egress rules |
| `ubicloud:subnet` | Private subnet |

## Habitat config

```scute
habitat "ubicloud"
  api_token:  env("UBICLOUD_API_TOKEN")
  project_id: "prj-abc123"
end
```

## Example

```scute
spawn "ubicloud:firewall" as "web-fw"
  name: "web-firewall"
  rules: [
    { direction: "inbound", protocol: "tcp", port: 443 },
    { direction: "inbound", protocol: "tcp", port: 80 }
  ]
end

spawn "ubicloud:vm" as "web-node"
  needs:    @web-fw
  name:     "web-node"
  size:     "standard-2"
  image:    "ubuntu-22.04"
  location: "eu-central-h1"
  firewall: @web-fw.id
  ssh_key:  "deploy-key"
end
```

## Building

See the [Building a Provider](Building-a-Provider) guide.
