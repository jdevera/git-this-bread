# 🍞 git-this-bread

> *Let's git this bread* — tools for developers who knead to understand their git repos

[![Go Version](https://img.shields.io/github/go-mod/go-version/jdevera/git-this-bread)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/jdevera/git-this-bread)](https://goreportcard.com/report/github.com/jdevera/git-this-bread)

A collection of git utilities, freshly baked in Go. Vibe-coded.

---

## 🥖 git-explain

**See your contribution status across repositories at a glance.**

Ever wonder which repos in a folder are yours, which are forks, and which are just clones you grabbed and forgot about? `git-explain` rises to the occasion.

### What it shows

- 🔍 **Your commits** — how many commits you've made (by matching your `user.email`)
- 🍴 **Fork detection** — identifies repos where you have an upstream remote
- ☁️ **Your remotes** — highlights remotes containing your GitHub username
- 📝 **Dirty status** — staged, modified, untracked files with line counts
- ⬆️ **Unpushed commits** — don't leave your dough unproofed
- 📦 **Stashes** — forgotten stashes you should deal with

### Installation

```bash
go install github.com/jdevera/git-this-bread/cmd/git-explain@latest
```

Then use it as a git subcommand:

```bash
git explain ~/projects
```

### Requirements

Set your git identity so git-explain knows who you are:

```bash
git config --global user.email "you@example.com"
git config --global github.user "yourusername"
```

### Usage

```bash
# Analyze all repos in a directory
git explain ~/projects

# Analyze a single repo with verbose output
git explain ~/projects/my-repo -v

# Show as a table
git explain ~/projects -t

# Output as JSON
git explain ~/projects --json

# Get advice on what to do
git explain ~/projects --advice
```

### Example output

```
 chezmoi   master   origin   3   2025-11-13   modified:1 +21/-0 untracked:3  fork
 command-launcher   main   origin   12   2025-10-20   modified:1 +2/-0 untracked:3   4 unpushed   1 stash  fork
 ddns-updater   json_api   origin   3   2026-01-06   untracked:1   1 stash  fork
 ebookatty   explicit_cli_output_format   origin   2   2026-01-04  fork
 grc   master   origin   1   2015-02-03   modified:52 +130/-146   1 unpushed  fork
 homepage   size_formatter   origin   4   2024-08-26  fork
 mirror-to-gitea   skip_forks   origin   5   2024-07-20   untracked:1  fork
```

### Verbose output

```
 command-launcher
     main
     Remotes:
        origin → git@github.com:jdevera/command-launcher.git (mine)
        upstream → git@github.com:criteo/command-launcher.git
     12 commits by you
     Last commit: 2025-10-20
     modified:1 +2/-0 untracked:3
     4 unpushed
     1 stash

    Branches with your commits:
        ● main                            10 commits  (2025-10-20)
        ○ self_updater_version_compare    7 commits  (2025-08-02)
        ○ docs_linting                    6 commits  (2025-02-20)
        ○ command_name_in_env             5 commits  (2024-08-26)
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Detailed multi-line output with branches |
| `--table` | `-t` | Compact table view |
| `--all` | `-a` | Include non-git directories |
| `--json` | | Output as JSON |
| `--advice` | | Show actionable suggestions |
| `--legend` | `-l` | Explain icons and colors |
| `--quiet` | `-q` | Suppress progress output |

---

## 🥐 More tools coming

This is a monorepo. More freshly baked git tools may appear here in the future.

---

## License

MIT — Do what you want, just don't blame me if your bread burns.
