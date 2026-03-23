# Provider: OpenStack

> **Status: alpha** — API client implementation in progress

The `openstack` provider manages instances, networks, subnets, security groups, and volumes via the OpenStack REST APIs.

## Resource types

| Type | Description |
|---|---|
| `openstack:instance` | Compute instance (Nova) |
| `openstack:network` | Virtual network (Neutron) |
| `openstack:subnet` | Subnet within a network |
| `openstack:security_group` | Security group with ingress/egress rules |
| `openstack:volume` | Block storage volume (Cinder) |

## Habitat config

```scute
habitat "openstack"
  auth_url:    "https://cloud.example.com:5000/v3"
  username:    env("OS_USERNAME")
  password:    env("OS_PASSWORD")
  tenant_name: "my-project"
  region:      "RegionOne"
  domain_name: "Default"
end
```

## Example

```scute
spawn "openstack:network" as "app-net"
  name: "app-network"
end

spawn "openstack:subnet" as "app-subnet"
  needs:   @app-net
  network: @app-net.id
  name:    "app-subnet"
  cidr:    "10.0.1.0/24"
  gateway: "10.0.1.1"
end

spawn "openstack:instance" as "web-server"
  needs:    @app-subnet
  name:     "web-server"
  image:    "ubuntu-22.04"
  flavor:   "m1.small"
  network:  @app-net.id
  key_name: "deploy-key"
end
```

## Building

See the [Building a Provider](Building-a-Provider) guide.
