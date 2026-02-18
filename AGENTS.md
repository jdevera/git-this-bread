# git-this-bread

Go monorepo with git utilities. Tools in `cmd/`, shared code in `internal/`.

## Commands

```
mise run build    # build all to dist/
mise run test     # unit tests
mise run lint     # golangci-lint
```

## Packages

- git-explain — repo status analyzer (see cmd/git-explain/AGENTS.md)
- git-as — identity tools bundle: git-id, git-as, gh-as (see cmd/git-id/AGENTS.md)

## Releases

GoReleaser builds binaries and Homebrew formulas on tag push:

```
git tag v1.0.0 && git push --tags
```

Formulas pushed to `jdevera/homebrew-tap`. Install with:

```
brew install jdevera/tap/git-explain
brew install jdevera/tap/git-as
```

## Test Helpers

`testutil.NewTestRepo(t)` creates temp git repos for tests.
