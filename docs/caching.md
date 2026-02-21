# Caching Strategy

git-this-bread tools use a common caching strategy for expensive operations.

## Cache Location

All caches live under the XDG cache directory:

```
~/.cache/git-this-bread/           # or $XDG_CACHE_HOME/git-this-bread/
├── git-explain/
│   └── llm-advice/                # LLM advice responses
│       └── {state_hash}.json
└── gh-wtfork/
    └── prs/                       # Merged/closed PR data
        └── {owner}_{repo}.json
```

## Cache Behavior

### Bypass without invalidation

All tools support `--no-cache` which:
- **Skips reading** from cache (fetches fresh data)
- **Still writes** to cache (refreshes it for next time)

This ensures running with `--no-cache` doesn't leave you with stale data on the next normal run.

### Per-tool strategies

#### git-explain (LLM advice)

- **What's cached**: LLM responses for repo analysis
- **Cache key**: Hash of repo state (branch, ahead/behind, dirty files, etc.)
- **Invalidation**: Automatic - cache key changes when repo state changes
- **TTL**: None (state-based invalidation)

#### gh-wtfork (PR data)

- **What's cached**: Merged and closed PRs only
- **Cache key**: Upstream repo name (`owner/repo`)
- **Invalidation**: Never - merged/closed PRs don't change
- **TTL**: None (permanent for merged/closed)

Open PRs are always fetched fresh. The cache provides:
- Faster access to historical PR data
- Fallback if API is rate-limited
- Accumulates PR history over time

## Implementation

Tools should use the common cache directory pattern:

```go
func getCacheDir(tool string) (string, error) {
    cacheHome := os.Getenv("XDG_CACHE_HOME")
    if cacheHome == "" {
        home, err := os.UserHomeDir()
        if err != nil {
            return "", err
        }
        cacheHome = filepath.Join(home, ".cache")
    }
    return filepath.Join(cacheHome, "git-this-bread", tool), nil
}
```

## Clearing the cache

To clear all caches:

```bash
rm -rf ~/.cache/git-this-bread/
```

To clear a specific tool's cache:

```bash
rm -rf ~/.cache/git-this-bread/gh-wtfork/
rm -rf ~/.cache/git-this-bread/git-explain/
```
