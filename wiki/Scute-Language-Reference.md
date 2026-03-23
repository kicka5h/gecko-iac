# Scute Language Reference

Scute (`.scute`) is Gecko's declarative configuration language. It is whitespace-significant and uses indentation (2 spaces) for block nesting.

## File structure

A Scute file has these top-level sections, in any order:

```
territory   — stack identity and workspace
store       — state backend configuration
habitat     — provider configuration
mark        — input variables
camouflage  — computed locals
spawn       — resource declarations
signal      — stack outputs
```

---

## territory

Declares the stack name and active workspace.

```scute
territory "my-homelab"
  workspace: "dev"
end
```

The `workspace` value is also available as the built-in variable `workspace` throughout the file.

---

## store — state backend

Declares where Gecko stores state. Replaces the `backend` key in `gecko.json`.

```scute
# Local disk (default)
store "local"
end

# S3-compatible (MinIO, AWS S3, Cloudflare R2)
store "s3"
  endpoint:   "https://minio.local:9000"
  bucket:     "gecko-state"
  prefix:     "my-homelab/"
  access_key: env("MINIO_ACCESS_KEY")
  secret_key: env("MINIO_SECRET_KEY")
end

# PostgreSQL
store "postgres"
  connection_string: env("GECKO_STATE_DSN")
  table:             "gecko_state"
end
```

| Type | Description |
|---|---|
| `local` | Files at `~/.gecko/state/` |
| `s3` | S3-compatible object storage |
| `etcd` | etcd key-value store |
| `postgres` | PostgreSQL table |

---

## habitat

Configures a provider. The string after `habitat` is the provider name (`proxmox`, `fly`, etc.).

```scute
habitat "proxmox"
  endpoint:  "https://pve.local:8006"
  api_token: env("PROXMOX_API_TOKEN")
  node:      "pve1"
end

habitat "fly"
  api_token: env("FLY_API_TOKEN")
  region:    "ord"
end
```

---

## mark — input variables

Declares overridable input variables. Type annotation is optional.

```scute
mark replicas number: 1
mark base_domain string: "homelab.local"
mark debug bool: false
```

Override at runtime:

```bash
gecko crawl --var replicas=3
GECKO_VAR_BASE_DOMAIN=prod.example.com gecko grip
```

### Types

| Type | Example |
|---|---|
| `string` | `"hello"` |
| `number` | `42`, `3.14` |
| `bool` | `true`, `false` |
| `list` | `[1, 2, 3]` |
| `map` | `{key: "value"}` |

---

## camouflage — computed locals

Locals are expressions evaluated at plan time. Not overridable.

```scute
camouflage
  is_prod:      workspace == "prod"
  app_replicas: is_prod ? 3 : replicas
  full_domain:  "app.#{base_domain}"
end
```

---

## spawn — resource declaration

Declares a managed resource. The type string is `provider:resource_type`.

```scute
spawn "fly:app" as "web-app"
  name: "my-app"
end

spawn "proxmox:vm" as "web-server"
  node:   "pve1"
  cores:  2
  memory: 2048
  disk:   "32G"
end
```

### spawn! — force replace

Use `spawn!` to force destruction and recreation on any change (instead of in-place update):

```scute
spawn! "fly:secret" as "app-config"
  app:   "my-app"
  key:   "DATABASE_URL"
  value: secret("db.url")
end
```

### needs — dependency

`needs:` declares an explicit dependency. Gecko ensures the referenced resource exists before creating this one.

```scute
spawn "fly:machine" as "app"
  needs: @db
  app:   @db.app
end
```

Multiple dependencies:

```scute
  needs: [@db, @cache, @ns]
```

---

## Resource references

Reference another resource's outputs using `@name` or `@name.field`:

```scute
spawn "proxmox:vm" as "db-server"
  node:   @db.node
  memory: @db.memory
end
```

`@name` alone refers to the resource ID itself (useful in `needs:`).

---

## String interpolation

Embed expressions inside strings with `#{}`:

```scute
  name: "app-#{workspace}"
  url:  "https://#{base_domain}/api"
```

---

## Pipe fallback

Use `|` to fall back to a default if a value is empty:

```scute
  region: env("AWS_REGION") | "us-east-1"
```

---

## Conditional blocks

```scute
when is_prod
  replicas: 3
  resources:
    memory: "512Mi"
    cpu:    "500m"
  end
else
  replicas: 1
  resources:
    memory: "128Mi"
    cpu:    "100m"
  end
end
```

```scute
unless debug
  log_level: "warn"
end
```

---

## across — iteration

Iterate over a list to create multiple resources:

```scute
mark services list: ["web", "api", "worker"]

across services as item
  spawn "fly:machine" as "svc-#{item}"
    app:   "my-app"
    image: "myorg/#{item}:latest"
  end
end
```

---

## secret()

Reference an encrypted secret managed by `gecko hide`:

```scute
  password: secret("db.password")
  api_key:  secret("stripe.api_key")
```

Falls back to the environment variable `GECKO_SECRET_<KEY>` (uppercased, dots replaced with underscores).

---

## env()

Read an environment variable at plan/apply time:

```scute
  token: env("PROXMOX_TOKEN_SECRET")
  host:  env("DB_HOST") | "localhost"
```

---

## signal — stack outputs

Exports values from a stack for inspection or cross-stack reference:

```scute
signal "grafana_url"
  value:       "https://grafana.#{base_domain}"
  description: "Grafana dashboard endpoint"
end

signal "db_host"
  value: @db-svc.cluster_ip
end
```

View outputs: `gecko bask --outputs`

---

## Comments

```scute
# This is a comment
```

---

## Full example

See [`examples/fly-homelab/stacks/dev/main.scute`](https://github.com/kicka5h/gecko-iac/tree/main/examples/fly-homelab) in the repository.
