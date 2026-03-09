# Provider: Kubernetes

The `kubernetes` provider manages Kubernetes resources via the dynamic client (`k8s.io/client-go`). It works with any conformant cluster: k3s, k0s, Talos, kubeadm, EKS, GKE, etc.

## Configuration

### gecko.json

```json
{
  "providers": {
    "k8s": {
      "type": "kubernetes",
      "config": {
        "kubeconfig": "~/.kube/config"
      }
    }
  }
}
```

### habitat

```scute
habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "homelab"       # optional: override the active context
end
```

| Field | Default | Description |
|---|---|---|
| `kubeconfig` | `~/.kube/config` | Path to kubeconfig file |
| `context` | current context | kubectl context to use |

The provider expands `~` and also falls back to the `$KUBECONFIG` environment variable if no `kubeconfig` is set.

## Supported resource types

| Type | API Group | Description |
|---|---|---|
| `k8s:namespace` | core/v1 | Namespace |
| `k8s:deployment` | apps/v1 | Deployment |
| `k8s:statefulset` | apps/v1 | StatefulSet |
| `k8s:daemonset` | apps/v1 | DaemonSet |
| `k8s:service` | core/v1 | Service |
| `k8s:configmap` | core/v1 | ConfigMap |
| `k8s:secret` | core/v1 | Secret |
| `k8s:persistentvolumeclaim` | core/v1 | PersistentVolumeClaim |
| `k8s:persistentvolume` | core/v1 | PersistentVolume |
| `k8s:storageclass` | storage.k8s.io/v1 | StorageClass |
| `k8s:serviceaccount` | core/v1 | ServiceAccount |
| `k8s:ingress` | networking.k8s.io/v1 | Ingress |
| `k8s:networkpolicy` | networking.k8s.io/v1 | NetworkPolicy |
| `k8s:clusterrole` | rbac.authorization.k8s.io/v1 | ClusterRole |
| `k8s:clusterrolebinding` | rbac.authorization.k8s.io/v1 | ClusterRoleBinding |
| `k8s:role` | rbac.authorization.k8s.io/v1 | Role |
| `k8s:rolebinding` | rbac.authorization.k8s.io/v1 | RoleBinding |
| `k8s:horizontalpodautoscaler` | autoscaling/v2 | HorizontalPodAutoscaler |
| `k8s:poddisruptionbudget` | policy/v1 | PodDisruptionBudget |
| `k8s:job` | batch/v1 | Job |
| `k8s:cronjob` | batch/v1 | CronJob |

## Example

```scute
habitat "k8s"
  kubeconfig: "~/.kube/config"
  context:    "homelab"
end

spawn "k8s:namespace" as "monitoring-ns"
  name: "monitoring"
  labels:
    managed-by: "gecko"
    env:        workspace
  end
end

spawn "k8s:deployment" as "prometheus"
  needs:     @monitoring-ns
  namespace: @monitoring-ns.name
  image:     "prom/prometheus:v2.50.0"
  replicas:  1
  ports:     [9090]
end

spawn "k8s:service" as "prometheus-svc"
  needs:     @prometheus
  namespace: @monitoring-ns.name
  port:      9090
end

spawn "k8s:deployment" as "grafana"
  needs:     @prometheus
  namespace: @monitoring-ns.name
  image:     "grafana/grafana:10.3.0"
  replicas:  1
  ports:     [3000]
  env:
    GF_SECURITY_ADMIN_PASSWORD: secret("grafana.admin.password")
    GF_SERVER_ROOT_URL:         "https://grafana.#{base_domain}"
  end
end
```

## Common patterns

### Namespace-scoped vs cluster-scoped

Resources like `k8s:namespace`, `k8s:persistentvolume`, `k8s:storageclass`, `k8s:clusterrole`, and `k8s:clusterrolebinding` are cluster-scoped — they don't take a `namespace:` field.

All other types are namespace-scoped and require `namespace:`.

### Depending on a kind cluster

If you're creating the cluster with the [kind provider](Provider-kind), add `needs: @cluster` to your first namespace:

```scute
spawn "k8s:namespace" as "app-ns"
  needs: @cluster
  name:  "my-app"
end
```

### Import an existing resource

```bash
gecko burrow k8s:deployment --id nginx --namespace default
```
