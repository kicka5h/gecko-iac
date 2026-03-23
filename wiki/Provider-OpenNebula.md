# Provider: OpenNebula

> **Status: alpha** — API client implementation in progress

The `opennebula` provider manages virtual machines, virtual networks, images, and templates via the OpenNebula XML-RPC API.

## Resource types

| Type | Description |
|---|---|
| `opennebula:vm` | Virtual machine |
| `opennebula:vnet` | Virtual network |
| `opennebula:image` | Disk image in the datastore |
| `opennebula:template` | VM template |

## Habitat config

```scute
habitat "opennebula"
  endpoint: "https://nebula.example.com:2633/RPC2"
  username: env("ONE_USERNAME")
  password: env("ONE_PASSWORD")
end
```

## Example

```scute
spawn "opennebula:vnet" as "app-vnet"
  name:   "app-network"
  bridge: "br0"
  ar: {
    type:     "IP4"
    ip:       "10.0.10.1"
    size:     254
  }
end

spawn "opennebula:image" as "base-disk"
  name:      "ubuntu-2204"
  type:      "OS"
  source:    "https://marketplace.opennebula.io/appliance/ubuntu-2204"
  datastore: "default"
end

spawn "opennebula:vm" as "worker"
  needs: [@app-vnet, @base-disk]
  name:    "worker-01"
  cpu:     2
  memory:  2048
  disk:    @base-disk.id
  network: @app-vnet.id
end
```

## Building

See the [Building a Provider](Building-a-Provider) guide.
