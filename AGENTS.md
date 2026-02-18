# git-this-bread

Go monorepo with git utilities. Tools in `cmd/`, shared code in `internal/`.

## Commands

```
mise run build    # build all to dist/
mise run test     # unit tests
mise run lint     # golangci-lint
```

## Tools

- cmd/git-explain — repo status analyzer (see cmd/git-explain/CLAUDE.md)
- cmd/git-id, cmd/git-as, cmd/gh-as — identity switching (see cmd/git-id/CLAUDE.md)

## Test Helpers

`testutil.NewTestRepo(t)` creates temp git repos for tests.
