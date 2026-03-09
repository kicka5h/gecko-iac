# State Backends

Gecko stores state per stack+workspace so it can detect drift and plan incremental changes. Configure the backend in `gecko.json`.

## local (default)

State stored as JSON files on disk. Good for solo use and local development.

```json
{
  "backend": {
    "type": "local"
  }
}
```

State files are written to `~/.gecko/state/<project>.<workspace>.json`.

## s3 (+ MinIO)

S3-compatible object storage. Works with AWS S3, MinIO, Cloudflare R2, and any S3-compatible endpoint.

```json
{
  "backend": {
    "type": "s3",
    "config": {
      "endpoint":          "https://minio.local:9000",
      "bucket":            "gecko-state",
      "prefix":            "my-homelab/",
      "access_key_id":     "REPLACE_ME",
      "secret_access_key": "REPLACE_ME",
      "region":            "us-east-1",
      "force_path_style":  true
    }
  }
}
```

Set credentials via environment variables instead of hardcoding:

```bash
export GECKO_S3_ACCESS_KEY_ID=...
export GECKO_S3_SECRET_ACCESS_KEY=...
```

## etcd

Distributed key-value store. Good for teams running etcd alongside Kubernetes.

```json
{
  "backend": {
    "type": "etcd",
    "config": {
      "endpoints": ["https://etcd.local:2379"],
      "prefix":    "/gecko/my-homelab/",
      "ca_cert":   "/etc/ssl/etcd/ca.crt",
      "cert":      "/etc/ssl/etcd/client.crt",
      "key":       "/etc/ssl/etcd/client.key"
    }
  }
}
```

## postgres

PostgreSQL table for state. Useful if you're already running Postgres.

```json
{
  "backend": {
    "type": "postgres",
    "config": {
      "connection_string": "postgres://gecko:password@postgres.local:5432/gecko_state?sslmode=disable",
      "table":             "gecko_state"
    }
  }
}
```

Gecko creates the table automatically if it doesn't exist.

## State format

State is plain JSON — no proprietary binary format. You can read, inspect, and migrate it manually. Each state file contains:

```json
{
  "version": 1,
  "project": "my-homelab",
  "workspace": "dev",
  "resources": {
    "k8s:namespace::monitoring": {
      "id":          "k8s:namespace::monitoring",
      "type":        "k8s:namespace",
      "provider":    "k8s",
      "external_id": "monitoring",
      "inputs":      { "name": "monitoring" },
      "outputs":     { "uid": "abc-123" },
      "created_at":  "2026-03-01T12:00:00Z",
      "updated_at":  "2026-03-01T12:00:00Z"
    }
  }
}
```

## State resilience

Gecko writes state after **each individual resource** succeeds. If `gecko grip` is interrupted halfway through, the next run picks up from where it left off — no partial state loss.
