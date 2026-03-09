# Provider: Nomad

The `nomad` provider manages HashiCorp Nomad resources — jobs, namespaces, ACL policies, and CSI volumes — via the official Nomad Go client.

## Configuration

### gecko.json

```json
{
  "providers": {
    "nomad": {
      "type": "nomad",
      "config": {
        "address": "http://nomad.local:4646"
      }
    }
  }
}
```

### habitat

```scute
habitat "nomad"
  address:   "http://nomad.local:4646"
  token:     env("NOMAD_TOKEN")
  region:    "global"
  namespace: "default"
end
```

| Field | Default | Description |
|---|---|---|
| `address` | `http://127.0.0.1:4646` | Nomad server address |
| `token` | `$NOMAD_TOKEN` | ACL token |
| `region` | `global` | Nomad region |
| `namespace` | `default` | Default namespace |

## Supported resource types

| Type | Description |
|---|---|
| `nomad:job` | Nomad job — register, update, stop, purge |
| `nomad:namespace` | Nomad namespace — create, update, delete |
| `nomad:policy` | ACL policy — create, update, delete |
| `nomad:volume` | CSI volume — register, deregister |

## Resource: nomad:job

Registers a Nomad service job.

| Field | Type | Description |
|---|---|---|
| `name` | string | Job ID |
| `namespace` | string | Target namespace (default: `default`) |
| `datacenters` | list | Datacenters to run in (default: `["dc1"]`) |
| `image` | string | Docker image for the main task |
| `count` | number | Instance count (default: 1) |
| `cpu` | number | CPU MHz (default: 100) |
| `memory` | number | Memory in MB (default: 128) |
| `port` | number | Port to expose |
| `env` | map | Environment variables |

```scute
spawn "nomad:job" as "nginx-job"
  needs:        @app-ns
  name:         "nginx"
  namespace:    @app-ns.name
  datacenters:  ["dc1"]
  image:        "nginx:latest"
  count:        2
  cpu:          200
  memory:       256
  port:         80
  env:
    NGINX_HOST: "#{base_domain}"
  end
end
```

## Resource: nomad:namespace

| Field | Type | Description |
|---|---|---|
| `name` | string | Namespace name |
| `description` | string | Human-readable description |

```scute
spawn "nomad:namespace" as "apps-ns"
  name:        "apps"
  description: "Application workloads"
end
```

## Resource: nomad:policy

| Field | Type | Description |
|---|---|---|
| `name` | string | Policy name |
| `description` | string | Human-readable description |
| `rules` | string | HCL policy rules |

```scute
spawn "nomad:policy" as "app-operator"
  name:        "app-operator"
  description: "Manage app namespace jobs"
  rules: |
    namespace "apps" {
      policy = "write"
    }
    agent { policy = "read" }
    node  { policy = "read" }
end
```

## Resource: nomad:volume

Registers a CSI volume.

| Field | Type | Description |
|---|---|---|
| `name` | string | Volume ID |
| `namespace` | string | Target namespace |
| `plugin_id` | string | CSI plugin to use |
| `capacity_min` | string | Minimum size (e.g. `10GiB`) |
| `capacity_max` | string | Maximum size |
| `access_mode` | string | `single-node-writer`, `multi-node-reader-only`, etc. |
| `attachment_mode` | string | `file-system` or `block-device` |

```scute
spawn "nomad:volume" as "data-vol"
  name:            "data-vol"
  namespace:       "apps"
  plugin_id:       "csi-hostpath"
  capacity_min:    "10GiB"
  capacity_max:    "20GiB"
  access_mode:     "single-node-writer"
  attachment_mode: "file-system"
end
```

## Full example

See [`examples/vault-nomad/`](https://github.com/kicka5h/gecko-iac/tree/main/examples/vault-nomad) in the repository.
