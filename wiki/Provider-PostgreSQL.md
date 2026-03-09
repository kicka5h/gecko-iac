# Provider: PostgreSQL

> **Status: planned** — tracked in [issue #10](https://github.com/kicka5h/gecko-iac/issues/10)

The `postgresql` provider will manage databases, roles, and extensions via `github.com/jackc/pgx/v5`.

## Planned resource types

| Type | Description |
|---|---|
| `postgresql:database` | Database with encoding and owner |
| `postgresql:role` | Role/user with password and login privileges |
| `postgresql:extension` | Extension (`CREATE EXTENSION IF NOT EXISTS`) |

## Planned habitat config

```scute
habitat "postgresql"
  host:     "postgres.local"
  port:     5432
  username: env("PGUSER")
  password: env("PGPASSWORD")
  sslmode:  "disable"
end
```

## Want to help?

See [issue #10](https://github.com/kicka5h/gecko-iac/issues/10) and the [Building a Provider](Building-a-Provider) guide.
