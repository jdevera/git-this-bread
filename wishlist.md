# Wishlist

## Build & Development

- [ ] Add [mise](https://mise.jdx.dev/) tasks to build and install tools under XDG bin (single tool or all tools) — polyglot dev tool manager and task runner
- [ ] Add tests
- [ ] Add [pre-commit](https://pre-commit.com/) hook for fmt, lint, build, and test — git hook framework to run checks before commits
- [x] Add [golangci-lint](https://golangci-lint.run/) config (`.golangci.yml`) — fast Go linters runner with dozens of linters included

## CI/CD

- [ ] Add GitHub Action for fmt, lint, build, and test
- [ ] Add [goreleaser](https://goreleaser.com/) for cross-platform builds, checksums, changelogs, and GitHub releases — release automation for Go projects
- [ ] Add [release-please](https://github.com/googleapis/release-please) for automated versioning and changelogs — creates a Release PR that accumulates conventional commits, merging triggers a release
- [ ] [Renovate](https://docs.renovatebot.com/) for automated dependency updates (maybe) — keeps dependencies up to date via PRs

## Distribution

- [ ] [Homebrew tap](https://docs.brew.sh/Taps) for easy `brew install` — custom Homebrew repository for distributing the tools
