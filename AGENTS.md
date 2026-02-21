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
- gh-wtfork — GitHub fork analyzer

## Caching

Tools cache expensive operations in `~/.cache/git-this-bread/`. See `docs/caching.md` for strategy.

## Releases

GoReleaser builds binaries and Homebrew formula on tag push:

```
git tag v1.0.0 && git push --tags
```

Formula pushed to `jdevera/homebrew-tap`. Install with:

```
brew install jdevera/tap/git-this-bread
```

## Test Helpers

`testutil.NewTestRepo(t)` creates temp git repos for tests.
