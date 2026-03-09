# Quick Start

This guide gets you from zero to a running Kubernetes deployment in about 5 minutes using [kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker).

## Prerequisites

- Docker Desktop (or Docker Engine on Linux)
- `kind` installed: `brew install kind`
- Gecko installed: see [Installation](Installation)

## 1. Create a project

```bash
gecko hatch my-homelab --providers k8s,kind --workspace dev
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

habitat "kind"
end

habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "kind-my-homelab"
end

spawn "kind:cluster" as "cluster"
  name: "my-homelab"
end

spawn "k8s:namespace" as "app-ns"
  needs: @cluster
  name:  "my-app"
end

spawn "k8s:deployment" as "nginx"
  needs:     @app-ns
  namespace: @app-ns.name
  image:     "nginx:latest"
  replicas:  1
  ports:     [80]
end

spawn "k8s:service" as "nginx-svc"
  needs:     @nginx
  namespace: @app-ns.name
  port:      80
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

Gecko creates the kind cluster, then the namespace, deployment, and service — in dependency order.

## 5. Check status

```bash
gecko bask
```

## 6. Stream logs

```bash
gecko tail --resource k8s:deployment.nginx --follow
```

## 7. Clean up

```bash
gecko molt
```

Destroys all resources in reverse order.

---

Next: [Scute Language Reference](Scute-Language-Reference) to learn the full DSL.
