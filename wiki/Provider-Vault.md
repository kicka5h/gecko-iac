# Provider: Vault

The `vault` provider manages HashiCorp Vault resources — KV secrets, policies, auth methods, and secret engine mounts — via the official Vault Go client.

## Configuration

### gecko.json

```json
{
  "providers": {
    "vault": {
      "type": "vault",
      "config": {
        "address": "https://vault.local:8200"
      }
    }
  }
}
```

### habitat

```scute
habitat "vault"
  address:   "https://vault.local:8200"
  token:     env("VAULT_TOKEN")
  namespace: ""        # Vault Enterprise namespace (optional)
  insecure:  false     # skip TLS verification (dev only)
end
```

| Field | Default | Description |
|---|---|---|
| `address` | `http://127.0.0.1:8200` | Vault server address |
| `token` | `$VAULT_TOKEN` | Auth token |
| `namespace` | — | Vault Enterprise namespace |
| `insecure` | `false` | Skip TLS certificate verification |

## Supported resource types

| Type | Description |
|---|---|
| `vault:secret` | KV v2 secret — write, read, delete |
| `vault:policy` | HCL policy — create, update, delete |
| `vault:auth` | Auth method — enable, configure, disable |
| `vault:mount` | Secret engine mount — enable, configure, disable |

## Resource: vault:secret

Writes a KV v2 secret.

| Field | Type | Description |
|---|---|---|
| `mount` | string | KV mount path (default: `secret`) |
| `path` | string | Secret path within the mount |
| `data` | map | Key-value pairs to store |

```scute
spawn "vault:secret" as "db-creds"
  mount: "secret"
  path:  "myapp/database"
  data:
    username: "myapp"
    password: secret("db.password")
  end
end
```

## Resource: vault:policy

| Field | Type | Description |
|---|---|---|
| `name` | string | Policy name |
| `policy` | string | HCL policy document |

```scute
spawn "vault:policy" as "app-policy"
  name: "myapp-read"
  policy: |
    path "secret/data/myapp/*" {
      capabilities = ["read", "list"]
    }
end
```

## Resource: vault:auth

Enables and configures an auth method.

| Field | Type | Description |
|---|---|---|
| `type` | string | Auth method type (`approle`, `kubernetes`, `github`, `ldap`, etc.) |
| `path` | string | Mount path (defaults to the type name) |
| `description` | string | Human-readable description |

```scute
spawn "vault:auth" as "approle-auth"
  type:        "approle"
  path:        "approle"
  description: "AppRole auth for services"
end
```

## Resource: vault:mount

Enables and configures a secret engine.

| Field | Type | Description |
|---|---|---|
| `type` | string | Engine type (`kv`, `pki`, `transit`, `aws`, etc.) |
| `path` | string | Mount path |
| `description` | string | Human-readable description |
| `options` | map | Engine-specific options (e.g. `version: "2"` for KV v2) |

```scute
spawn "vault:mount" as "kv-store"
  type:        "kv"
  path:        "secret"
  description: "KV v2 secret store"
  options:
    version: "2"
  end
end

spawn "vault:mount" as "pki-engine"
  type:        "pki"
  path:        "pki"
  description: "Internal PKI"
end
```

## Full example

See [`examples/vault-nomad/`](https://github.com/kicka5h/gecko-iac/tree/main/examples/vault-nomad) in the repository.
