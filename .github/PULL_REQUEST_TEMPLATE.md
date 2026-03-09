## What does this PR do?

<!-- A clear, concise description of the change. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] New provider
- [ ] Provider improvement
- [ ] Scute language change
- [ ] Refactor
- [ ] Docs / tests only

## Related issues

Closes #

## Provider checklist (if adding/changing a provider)

- [ ] Implements all `core.Provider` interface methods
- [ ] Uses lazy `connect()` — no client built during `Configure()`
- [ ] `Diff()` returns `ChangeAdd` when `current == nil` (no cluster required for `crawl`)
- [ ] `Create`, `Read`, `Update`, `Delete`, `Import` call `connect()` at the top
- [ ] Registered in `crawl.go`, `grip.go`, and `observe.go` (molt/bask)
- [ ] Example `.scute` file added under `examples/`

## Testing

<!-- How did you test this? Include commands run and relevant output. -->

```bash
gecko crawl
gecko grip
```

## Screenshots / output (if applicable)
