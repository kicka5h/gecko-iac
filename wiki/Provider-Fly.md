# Provider: Fly.io

> **Status: alpha** — API client implementation in progress

The `fly` provider manages Fly.io applications, machines, volumes, and secrets via the Fly Machines API.

## Resource types

| Type | Description |
|---|---|
| `fly:app` | Fly application |
| `fly:machine` | Fly Machine (microVM) |
| `fly:volume` | Persistent volume |
| `fly:secret` | Application secret |

## Habitat config

```scute
habitat "fly"
  api_token: env("FLY_API_TOKEN")
  org:       "personal"
  region:    "sjc"
end
```

## Example

```scute
spawn "fly:app" as "my-api"
  name: "my-api"
  org:  "personal"
end

spawn "fly:machine" as "web"
  needs: @my-api
  app:    @my-api.name
  image:  "registry.fly.io/my-api:latest"
  size:   "shared-cpu-1x"
  region: "sjc"
  memory: 256
end
```

## Building

See the [Building a Provider](Building-a-Provider) guide.
