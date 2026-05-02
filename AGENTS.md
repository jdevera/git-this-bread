# git-this-bread

Go monorepo with git utilities. Tools in `cmd/`, shared code in `internal/`.

## Commands

```
mise run build    # build all to dist/
mise run test     # unit tests
mise run lint     # golangci-lint
```

## Pre-commit hooks

Install once per clone (requires the [pre-commit](https://pre-commit.com/) framework):

```
pre-commit install
```

Runs `golangci-lint` (pinned to the same version as CI) and `go test` before
each commit. Skipping with `--no-verify` is discouraged — fix the underlying
issue rather than bypassing.

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

## Transient docs

`/.local-docs/` is gitignored and is the home for working notes that should
not be committed: bug investigations, design drafts, scratch analyses,
anything written for the moment rather than for the repo. Put files there by
default; only promote to `docs/` (or a per-tool README) when the content is
intended to ship to other readers.
