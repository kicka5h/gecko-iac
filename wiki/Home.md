# 🦎 Gecko Wiki

> *Move fast. Grip hard.*

Welcome to the Gecko documentation. Gecko is a **FOSS-first Infrastructure as Code framework** built in Go, with its own declarative language **Scute** (`.scute`).

**Alpha release: Amalosia** — named after *Amalosia*, the first genus in the family Diplodactylidae.

---

## Getting Started

- [Installation](Installation) — Homebrew, curl, Go, Windows
- [Quick Start](Quick-Start) — Your first Gecko project in 5 minutes
- [Project Structure](Project-Structure) — How a Gecko project is laid out

## The Scute Language

- [Scute Language Reference](Scute-Language-Reference) — Full syntax and keyword reference
- [Variables and Locals](Scute-Variables-and-Locals) — `mark`, `camouflage`, interpolation
- [Resource References](Scute-Resource-References) — `@resource`, `@resource.field`, `needs:`
- [Secrets](Scute-Secrets) — `secret()`, `gecko hide`
- [Modules (Snack)](Scute-Modules) — Reusable infra building blocks

## CLI Reference

- [CLI Overview](CLI-Reference) — All commands at a glance
- [gecko hatch](CLI-hatch) — Initialize a new project
- [gecko crawl](CLI-crawl) — Preview changes (plan)
- [gecko grip](CLI-grip) — Apply infrastructure
- [gecko molt](CLI-molt) — Destroy infrastructure
- [gecko bask](CLI-bask) — Status dashboard
- [gecko tail](CLI-tail) — Stream live logs
- [gecko lick](CLI-lick) — Inspect a resource
- [gecko hide](CLI-hide) — Manage secrets
- [gecko clutch](CLI-clutch) — Manage workspaces
- [gecko cross](CLI-cross) — Import Terraform/Pulumi state
- [gecko check](CLI-check) — Validate .scute files
- [gecko fmt](CLI-fmt) — Format .scute files

## Providers

- [Provider Overview](Providers) — All supported FOSS providers
- [Proxmox](Provider-Proxmox) — VMs, LXC, storage, networks
- [Fly](Provider-Fly) — Apps, machines, volumes, secrets
- [OpenStack](Provider-OpenStack) — Instances, networks, subnets, security groups, volumes
- [Hostinger](Provider-Hostinger) — VPS, domains
- [Ubicloud](Provider-Ubicloud) — VMs, firewalls, subnets
- [OpenNebula](Provider-OpenNebula) — VMs, vnets, images, templates
- [Building a Provider](Building-a-Provider) — Implement `core.Provider`

## State & Backends

- [State Backends](State-Backends) — local, S3/MinIO, etcd, postgres

## Contributing

- [Contributing Guide](Contributing) — How to contribute to Gecko
- [Branch Naming](Contributing#branch-naming) — `v0.1.0/Amalosia-Core`
- [Provider Checklist](Contributing#provider-checklist) — What every provider needs

---

*This wiki is managed from source in the [`wiki/`](https://github.com/kicka5h/gecko-iac/tree/main/wiki) directory of the main repository. Edit pages there — changes sync automatically on merge to `main`.*
