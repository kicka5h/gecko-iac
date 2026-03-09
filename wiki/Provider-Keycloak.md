# Provider: Keycloak

> **Status: planned** — tracked in [issue #7](https://github.com/kicka5h/gecko-iac/issues/7)

The `keycloak` provider will manage realms, clients, users, and roles via the Keycloak Admin REST API using `github.com/Nerzal/gocloak/v13`.

## Planned resource types

| Type | Description |
|---|---|
| `keycloak:realm` | Keycloak realm |
| `keycloak:client` | OIDC/SAML client |
| `keycloak:user` | User account with role assignments |
| `keycloak:role` | Realm or client role |

## Planned habitat config

```scute
habitat "keycloak"
  url:           "https://keycloak.local"
  realm:         "master"
  client_id:     "admin-cli"
  client_secret: env("KEYCLOAK_CLIENT_SECRET")
end
```

## Want to help?

See [issue #7](https://github.com/kicka5h/gecko-iac/issues/7) and the [Building a Provider](Building-a-Provider) guide.
