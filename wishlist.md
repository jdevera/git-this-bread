# Wishlist

Items ordered by impact within each section.

## High Priority

- [ ] Add tests — foundation for CI and refactoring confidence
- [ ] Add GitHub Action for fmt, lint, build, and test — visible quality gate on every push/PR

## Medium Priority

- [ ] Add [goreleaser](https://goreleaser.com/) for cross-platform builds, checksums, changelogs, and GitHub releases — enables actual distribution to users
- [ ] Add [pre-commit](https://pre-commit.com/) hook for fmt, lint, build, and test — local enforcement before push
- [ ] [Homebrew tap](https://docs.brew.sh/Taps) for easy `brew install` — requires goreleaser first
- [ ] Add [mise](https://mise.jdx.dev/) tasks to build and install tools under XDG bin (single tool or all tools) — developer convenience

## Low Priority

- [ ] Add [release-please](https://github.com/googleapis/release-please) for automated versioning and changelogs — automation polish, manual releases work fine initially
- [ ] [Renovate](https://docs.renovatebot.com/) for automated dependency updates — more valuable for long-term maintenance

## Done

- [x] Add [golangci-lint](https://golangci-lint.run/) config (`.golangci.yml`) — fast Go linters runner with dozens of linters included
