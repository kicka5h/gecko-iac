# Provider: NFS

> **Status: planned** — tracked in [issue #9](https://github.com/kicka5h/gecko-iac/issues/9)

The `nfs` provider will manage NFS exports and mounts by managing `/etc/exports` and calling `exportfs`/`mount`.

## Planned resource types

| Type | Description |
|---|---|
| `nfs:export` | NFS export entry — manages `/etc/exports` |
| `nfs:mount` | NFS mount point — manages `/etc/fstab` |

## Planned habitat config

```scute
habitat "nfs"
  exports_file: "/etc/exports"
  host:         "nfs.local"
end
```

## Want to help?

See [issue #9](https://github.com/kicka5h/gecko-iac/issues/9) and the [Building a Provider](Building-a-Provider) guide.
