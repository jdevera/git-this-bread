# git-explain

CLI tool to show contribution status across git repositories.

## Architecture

```
main.go (CLI/cobra)
    ↓
analyzer.AnalyzeRepo() → RepoInfo struct
    ↓
render.RenderRepo() → stdout (compact/verbose/table/JSON)
```

`RepoInfo` is the intermediate data structure between analysis and rendering.

## Key Dependencies

- `github.com/go-git/go-git/v5` — git operations
- `github.com/charmbracelet/lipgloss` — terminal styling
- `github.com/spf13/cobra` — CLI framework

## Testing

```bash
go test ./...                        # unit tests
go test -tags=integration ./...      # include integration tests
```

Integration tests use `testutil.NewTestRepo(t)` to create temp git repos.

## Required Git Config

The tool identifies user commits/remotes via:
- `git config user.email` — matches commit authors
- `git config github.user` — matches remote URLs
