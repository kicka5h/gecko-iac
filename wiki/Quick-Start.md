# Quick Start

This guide gets you from zero to a running Fly.io app in about 5 minutes. Fly.io is the simplest provider to start with — all you need is an API token.

## Prerequisites

- A [Fly.io](https://fly.io) account
- A Fly.io API token: `fly tokens create org`
- Gecko installed: see [Installation](Installation)

## 1. Create a project

```bash
gecko hatch my-homelab --providers fly --workspace dev
cd my-homelab
```

This creates:

```
my-homelab/
├── gecko.json          # project config
├── stacks/
│   └── dev/
│       └── main.scute  # your stack declaration
└── .gecko/             # state and plans (gitignored)
```

## 2. Write your stack

Edit `stacks/dev/main.scute`:

```scute
territory "my-homelab"
  workspace: "dev"
end

habitat "fly"
  api_token: env("FLY_API_TOKEN")
  region:    "ord"
end

spawn "fly:app" as "web-app"
  name: "my-homelab-web"
end

spawn "fly:machine" as "web"
  needs: @web-app
  app:   @web-app.name
  image: "nginx:latest"
  cpus:  1
  memory: 256
  ports: [80, 443]
end
```

## 3. Preview the plan

```bash
gecko crawl
```

Gecko shows every resource it will create, with no changes made yet.

## 4. Apply

```bash
gecko grip
```

Gecko creates the Fly app, then the machine — in dependency order.

## 5. Check status

```bash
gecko bask
```

## 6. Stream logs

```bash
gecko tail --resource fly:machine.web --follow
```

## 7. Clean up

```bash
gecko molt
```

Destroys all resources in reverse order.

---

Next: [Scute Language Reference](Scute-Language-Reference) to learn the full DSL.
