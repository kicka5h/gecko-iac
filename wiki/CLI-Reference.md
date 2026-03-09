# CLI Reference

Gecko's commands are named after gecko anatomy and behavior.

## Commands at a glance

| Command | Alias | What It Does |
|---|---|---|
| `gecko hatch` | `init` | Initialize a new project |
| `gecko crawl` | `plan` | Preview changes (dry run) |
| `gecko grip` | `apply` | Apply infrastructure changes |
| `gecko molt` | `destroy` | Tear down all infrastructure |
| `gecko bask` | `status` | Infrastructure status dashboard |
| `gecko tail` | `logs` | Stream live logs |
| `gecko lick` | `inspect` | Deep-inspect a resource |
| `gecko shed` | `upgrade` | Upgrade providers/framework |
| `gecko hide` | `secrets` | Manage encrypted secrets |
| `gecko burrow` | `import` | Import existing resources |
| `gecko scale` | `resize` | Scale resources up/down |
| `gecko clutch` | `workspace` | Manage stacks/workspaces |
| `gecko cross` | `bridge` | Import Terraform/Pulumi state |
| `gecko check` | `validate` | Validate .scute files |
| `gecko fmt` | — | Format .scute files |
| `gecko run` | — | Evaluate a .scute file (dry eval) |
| `gecko snack` | — | Manage Snack modules |
| `gecko version` | — | Show version information |

## Global flags

```
--help, -h      Show help
--version, -v   Show version
```

## gecko hatch

Initialize a new Gecko project.

```bash
gecko hatch <project-name> [flags]

Flags:
  --providers, -p   Comma-separated providers to configure (e.g. k8s,vault)
  --workspace, -w   Initial workspace name (default: dev)
  --backend         State backend type: local, s3, etcd, postgres (default: local)
```

## gecko crawl

Preview what Gecko would do — no changes made.

```bash
gecko crawl [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --target, -t      Limit to a specific resource (e.g. k8s:deployment.nginx)
  --dir, -d         Project directory (default: .)
  --out             Save plan to file for use with gecko grip --plan
  --json            Output plan as JSON
```

## gecko grip

Apply infrastructure changes.

```bash
gecko grip [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --target, -t      Apply only a specific resource
  --plan            Path to a saved crawl plan
  --auto-approve    Skip confirmation prompt
  --dir, -d         Project directory (default: .)
```

## gecko molt

Destroy all managed infrastructure.

```bash
gecko molt [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --target, -t      Destroy only a specific resource
  --auto-approve    Skip confirmation prompt (dangerous!)
  --dir, -d         Project directory (default: .)
```

## gecko bask

Show infrastructure status dashboard.

```bash
gecko bask [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --dir, -d         Project directory (default: .)
  --outputs         Show stack signal outputs
  --json            Output status as JSON
```

## gecko tail

Stream live logs from a resource.

```bash
gecko tail [flags]

Flags:
  --resource, -r    Resource to stream logs from (e.g. k8s:deployment.nginx)
  --workspace, -w   Target workspace (default: dev)
  --follow, -f      Keep streaming (default: false)
  --lines, -n       Number of past lines to show (default: 50)
```

## gecko lick

Deep-inspect a resource (show full state, attributes, drift).

```bash
gecko lick <resource-id> [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --dir, -d         Project directory (default: .)
```

## gecko hide

Manage encrypted secrets.

```bash
# Set a secret
gecko hide set --key grafana.admin.password --value supersecret

# List all secrets
gecko hide

# Delete a secret
gecko hide delete --key grafana.admin.password
```

Secrets are encrypted with AES-256-GCM and stored in `.gecko/secrets.enc`. Environment variable fallback: `GECKO_SECRET_<KEY>` (uppercase, dots become underscores).

## gecko clutch

Manage workspaces.

```bash
gecko clutch                           # list workspaces
gecko clutch new --name production     # create workspace
gecko clutch select production         # switch active workspace
gecko crawl --workspace staging        # plan against specific workspace
```

## gecko cross

Import existing Terraform or Pulumi state into Scute.

```bash
# From Terraform state
gecko cross --from terraform.tfstate --out stacks/dev/main.scute

# From Pulumi stack state
gecko cross --from .pulumi/stacks/dev.json --out stacks/dev/main.scute

# Preview to stdout
gecko cross --from terraform.tfstate
```

## gecko check

Validate `.scute` files (parse + type check) without touching infrastructure.

```bash
gecko check [flags]

Flags:
  --workspace, -w   Target workspace (default: dev)
  --dir, -d         Project directory (default: .)
```

## gecko fmt

Format `.scute` files in canonical style.

```bash
gecko fmt [file...]         # format specific files
gecko fmt --dir .           # format all .scute files in project
gecko fmt --check           # check formatting without writing
```

## gecko version

```bash
gecko version
```

Shows release codename, version, commit, build date, and license.
