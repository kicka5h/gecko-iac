# Contributing

Thank you for contributing to Gecko! This guide covers everything you need to know.

## Development setup

```bash
git clone https://github.com/kicka5h/gecko-iac.git
cd gecko-iac
./scripts/bootstrap.sh   # fetch Go deps and build
gecko --version
```

Requires Go 1.22+.

## Branch naming

All branches must follow this pattern:

```
v<semver>/<Codename>-<Milestone>
```

Examples:

```
v0.1.0/Amalosia-Core
v0.2.0/Amalosia-Providers
v1.0.0/Belanotus-Networking
```

Exceptions: `main` and `hotfix/*` branches are exempt.

The CI workflow `branch-name.yml` enforces this on every PR.

## Commit style

Prefix commits with a type:

| Prefix | Use for |
|---|---|
| `feat:` | New feature or resource type |
| `fix:` | Bug fix |
| `provider:` | Provider implementation work |
| `docs:` | Documentation only |
| `test:` | Tests only |
| `ci:` | CI/CD changes |
| `refactor:` | Refactoring (no behavior change) |

Example: `feat: add keycloak:realm resource type`

## Running tests

```bash
go test ./...
go test -race ./...        # with race detector
go vet ./...
```

## Provider checklist

Every new or updated provider must satisfy:

- [ ] Implements all `core.Provider` interface methods
- [ ] Uses lazy `connect()` — no client built during `Configure()`
- [ ] `Diff()` returns `ChangeAdd` when `current == nil` (no live cluster required for `gecko crawl`)
- [ ] `Create`, `Read`, `Update`, `Delete`, `Import` call `connect()` at the top
- [ ] Registered in `cmd/crawl.go`, `cmd/grip.go`, and `cmd/observe.go`
- [ ] Example `.scute` file added under `examples/<provider>/`
- [ ] Provider documented in the wiki (`wiki/Provider-<Name>.md`)

## Scute language changes

Changes to the Scute DSL (new keywords, syntax changes) require:

- [ ] Parser update in `internal/lang/`
- [ ] Updated grammar in `editors/vscode-scute/syntaxes/scute.tmLanguage.json`
- [ ] Updated [Scute Language Reference](Scute-Language-Reference) wiki page
- [ ] Example updated

## Opening a PR

1. Create a branch following the naming convention above
2. Write your code
3. Run `go test ./...` and `go vet ./...`
4. Open a PR — the template will guide you through the checklist
5. All PRs target `main`

## Milestones

Issues are grouped by milestone. The current milestone is **Core** (Amalosia alpha). See [GitHub Milestones](https://github.com/kicka5h/gecko-iac/milestones) for what's planned.

## Code of conduct

Be kind. Review code, not people. If something is wrong, say so constructively with a suggested fix.
