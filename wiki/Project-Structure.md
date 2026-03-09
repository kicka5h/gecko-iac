# Project Structure

After `gecko hatch`, your project looks like:

```
my-homelab/
├── gecko.json              # Project config: providers, backend, workspaces
├── .geckoignore            # Files to exclude from gecko operations
├── stacks/
│   ├── dev/
│   │   └── main.scute     # Dev workspace stack declaration
│   ├── staging/
│   │   └── main.scute
│   └── prod/
│       └── main.scute
├── modules/                # Reusable Snack modules
│   ├── monitoring/
│   │   └── main.snack
│   └── networking/
│       └── main.snack
└── .gecko/                 # Gecko runtime data (gitignore this)
    ├── state/              # Local state files
    └── plans/              # Saved crawl plans
```

## gecko.json

The project manifest. Configures providers and the state backend:

```json
{
  "name": "my-homelab",
  "workspaces": ["dev", "staging", "prod"],
  "providers": {
    "kind": { "type": "kind" },
    "k8s": {
      "type": "kubernetes",
      "config": {
        "kubeconfig": "~/.kube/config"
      }
    },
    "vault": {
      "type": "vault",
      "config": {
        "address": "https://vault.local:8200"
      }
    }
  },
  "backend": {
    "type": "local"
  }
}
```

## Stacks

Each workspace has its own `.scute` file under `stacks/<workspace>/`. The workspace name is available as a built-in variable inside the stack.

Multiple `.scute` files in the same workspace directory are merged — useful for splitting large stacks:

```
stacks/prod/
├── main.scute        # territory + habitats + variables
├── monitoring.scute  # Prometheus, Grafana
└── networking.scute  # Ingress, certs, DNS
```

## Modules (Snack)

Reusable infra components live in `modules/`. See [Modules](Scute-Modules).

## State

State is stored per stack + workspace. With the default local backend:

```
~/.gecko/state/<project>.<workspace>.json
```

To use a remote backend (required for teams), configure it in `gecko.json`:

```json
{
  "backend": {
    "type": "s3",
    "config": {
      "endpoint": "https://minio.local:9000",
      "bucket":   "gecko-state",
      "prefix":   "my-homelab/"
    }
  }
}
```

See [State Backends](State-Backends) for all options.
