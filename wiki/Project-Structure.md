# Project Structure

After `gecko hatch`, your project looks like:

```
my-homelab/
├── dev/
│   └── main.scute          # Dev workspace stack declaration
├── staging/
│   └── main.scute
├── prod/
│   └── main.scute
├── modules/                # Reusable Snack modules
│   ├── monitoring/
│   │   └── main.snack
│   └── networking/
│       └── main.snack
├── .geckoignore            # Files to exclude from gecko operations
└── .gecko/                 # Gecko runtime data (gitignore this)
    ├── state/              # Local state files
    └── plans/              # Saved crawl plans
```

Gecko is **directory-name agnostic** — it finds your project by looking for `.gecko/` or any `.scute` file, regardless of where the files live. The workspace name (e.g. `dev`) determines which subdirectory is loaded, not a hardcoded `stacks/` prefix. All of the following layouts work:

```
dev/main.scute
stacks/dev/main.scute
envs/dev/main.scute
workspaces/dev/main.scute
```

## Stacks

Each workspace has its own `.scute` file in a directory named after that workspace. The workspace name is available as a built-in variable inside the stack.

Multiple `.scute` files in the same workspace directory are merged — useful for splitting large stacks:

```
prod/
├── main.scute        # territory + habitats + variables
├── monitoring.scute  # Prometheus, Grafana
└── networking.scute  # Ingress, certs, DNS
```

## Configuring Providers and Backend

Everything lives in `.scute` files — there is no separate `gecko.json`. Providers are declared with `habitat` blocks and the state backend with a `store` block:

```scute
store "local"
end

habitat "proxmox"
  endpoint:     env("PROXMOX_ENDPOINT") | "https://192.168.1.100:8006"
  token_id:     env("PROXMOX_TOKEN_ID")
  token_secret: env("PROXMOX_TOKEN_SECRET")
  node:         env("PROXMOX_NODE") | "pve"
end
```

## Modules (Snack)

Reusable infra components live in `modules/`. See [Modules](Scute-Modules).

## State

State is stored per stack + workspace. With the default local backend:

```
~/.gecko/state/<project>.<workspace>.json
```

To use a remote backend, set it in your `store` block:

```scute
store "s3"
  endpoint: "https://minio.local:9000"
  bucket:   "gecko-state"
  prefix:   "my-homelab/"
end
```

See [State Backends](State-Backends) for all options.
