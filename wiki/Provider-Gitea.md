# Provider: Gitea

> **Status: planned** — tracked in [issue #5](https://github.com/kicka5h/gecko-iac/issues/5)

The `gitea` provider will manage repositories, organizations, users, webhooks, and runners via the Gitea REST API using the official `code.gitea.io/sdk/gitea` Go client.

## Planned resource types

| Type | Description |
|---|---|
| `gitea:repo` | Repository |
| `gitea:org` | Organization |
| `gitea:user` | User account |
| `gitea:webhook` | Repo or org webhook |
| `gitea:runner` | Runner registration |

## Planned habitat config

```scute
habitat "gitea"
  url:   "https://gitea.local"
  token: env("GITEA_TOKEN")
end
```

## Want to help?

See [issue #5](https://github.com/kicka5h/gecko-iac/issues/5) and the [Building a Provider](Building-a-Provider) guide.
