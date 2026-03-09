# Scute Language Reference

Scute (`.scute`) is Gecko's declarative configuration language. It is whitespace-significant and uses indentation (2 spaces) for block nesting.

## File structure

A Scute file has four top-level sections, in any order:

```
territory   — stack identity and workspace
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

## habitat

Configures a provider. The string after `habitat` is the provider name, which must match a key in `gecko.json`.

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
spawn "k8s:deployment" as "nginx"
  namespace: "default"
  image:     "nginx:1.25"
  replicas:  2
  ports:     [80, 443]
end
```

### spawn! — force replace

Use `spawn!` to force destruction and recreation on any change (instead of in-place update):

```scute
spawn! "k8s:configmap" as "app-config"
  name: "app-config"
  data:
    key: "value"
  end
end
```

### needs — dependency

`needs:` declares an explicit dependency. Gecko ensures the referenced resource exists before creating this one.

```scute
spawn "k8s:deployment" as "app"
  needs:     @db
  namespace: @db.namespace
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
spawn "k8s:service" as "db-svc"
  namespace: @db.namespace
  port:      @db.port
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
mark namespaces list: ["monitoring", "logging", "ingress"]

across namespaces as item
  spawn "k8s:namespace" as "ns-#{item}"
    name: item
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
  token: env("VAULT_TOKEN")
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

See [`examples/k8s-homelab/stacks/dev/main.scute`](https://github.com/kicka5h/gecko-iac/tree/main/examples/k8s-homelab) in the repository.
