# Provider: MinIO

> **Status: planned** — tracked in [issue #6](https://github.com/kicka5h/gecko-iac/issues/6)

The `minio` provider will manage S3-compatible object storage via the MinIO Go SDK (`minio-go`) and admin API (`madmin-go`).

## Planned resource types

| Type | Description |
|---|---|
| `minio:bucket` | S3 bucket with versioning and lifecycle |
| `minio:policy` | IAM-style bucket policy |
| `minio:user` | MinIO user with policy assignment |

## Planned habitat config

```scute
habitat "minio"
  endpoint:   "minio.local:9000"
  access_key: env("MINIO_ACCESS_KEY")
  secret_key: env("MINIO_SECRET_KEY")
  secure:     false
end
```

## Want to help?

See [issue #6](https://github.com/kicka5h/gecko-iac/issues/6) and the [Building a Provider](Building-a-Provider) guide.
