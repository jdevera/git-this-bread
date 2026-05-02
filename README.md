# 🍞 git-this-bread

> *Let's git this bread* — tools for developers who knead to understand their git repos

[![Go Version](https://img.shields.io/github/go-mod/go-version/jdevera/git-this-bread)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/jdevera/git-this-bread)](https://goreportcard.com/report/github.com/jdevera/git-this-bread)

A collection of git utilities, freshly baked in Go. Vibe-coded.

## Tools

| Package | Tools | Description |
|---------|-------|-------------|
| **git-explain** | [git-explain](#-git-explain) | See contribution status across repositories |
| **git-as** | [git-id](#-git-id), [git-as](#-git-as), [gh-as](#-gh-as) | Identity switching for git and GitHub CLI |
| **gh-wtfork** | [gh-wtfork](#-gh-wtfork) | What the fork? Triage years of GitHub forks |

## Installation

### Homebrew (recommended)

```bash
brew install jdevera/tap/git-this-bread
```

This installs: `git-explain`, `git-id`, `git-as`, `gh-as`, `gh-wtfork`

### Go install

```bash
# Install all at once
git clone https://github.com/jdevera/git-this-bread && cd git-this-bread && go install ./cmd/...

# Or one by one
go install github.com/jdevera/git-this-bread/cmd/git-explain@latest
go install github.com/jdevera/git-this-bread/cmd/git-id@latest
go install github.com/jdevera/git-this-bread/cmd/git-as@latest
go install github.com/jdevera/git-this-bread/cmd/gh-as@latest
go install github.com/jdevera/git-this-bread/cmd/gh-wtfork@latest
```

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

# Get LLM-powered advice (requires OPENAI_API_KEY or ANTHROPIC_API_KEY)
git explain ~/projects --llm-advice

# Use Anthropic instead of OpenAI
git explain ~/projects --llm-advice --llm-provider anthropic

# Add custom personality to LLM advice
git explain ~/projects --llm-advice --llm-instructions "be encouraging and use baking puns"
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
| `--compact` | `-c` | One-line output (default for multi-repo) |
| `--table` | `-t` | Compact table view |
| `--all` | `-a` | Include non-git directories |
| `--json` | | Output as JSON |
| `--advice` | | Show actionable suggestions |
| `--llm-advice` | | Enable LLM-powered advice (requires API key) |
| `--llm-provider` | | LLM provider: `openai` (default), `anthropic` |
| `--llm-instructions` | | Custom instructions for the LLM |
| `--no-cache` | | Bypass LLM advice cache |
| `--per-repo` | | Analyze each repo individually with LLM |
| `--legend` | `-l` | Explain icons and colors |
| `--quiet` | `-q` | Suppress progress output |

---

## 🥯 git-id

**Manage git identity profiles for multi-account workflows.** Stores `[identity "<name>"]` sections in your git config so `git-as` and `gh-as` can switch contexts without manual env juggling.

```bash
git-id                  # list profiles
git-id add personal     # create one interactively
git-id show personal    # inspect
git-id set personal email me@example.com
git-id agent list       # running sub-agents (used by git-as)
```

→ Full reference: [docs/git-id.md](docs/git-id.md)

---

## 🥨 git-as

**Run git commands with a specific identity.** Picks the profile's SSH key and authorship, isolates the SSH agent so the wrong key can't get offered first, then execs `git`.

```bash
git-as personal clone git@github.com:user/repo.git
git-as work push origin main
git-as personal commit -m "Fix bug"
```

By default, `git-as` spawns a per-profile `ssh-agent` loaded with only that profile's key — your system agent is untouched, plain `git` keeps working as before, and the encrypted-key passphrase is entered once (cached in macOS Keychain, or via `SSH_ASKPASS` on Linux).

→ Full reference, mental model, troubleshooting: [docs/git-as.md](docs/git-as.md)

---

## 🥞 gh-as

**Run GitHub CLI commands with a specific identity.**

Switch between authenticated GitHub accounts for `gh` commands. Reads profiles managed by [`git-id`](docs/git-id.md).

### Requirements

The GitHub user must be authenticated with `gh auth login` before use.

### Usage

```bash
# List PRs as your personal account
gh-as personal pr list

# Create an issue as your work account
gh-as work issue create

# Clone a repo as a specific user
gh-as personal repo clone owner/repo
```

### How it works

`gh-as` creates a temporary config directory with a `hosts.yml` that selects the specified user, then execs `gh` with `GH_CONFIG_DIR` pointing to it.

---

## 🍴 gh-wtfork

**What the fork? Analyze your GitHub forks.**

You've accumulated mass amounts of repositories after years of compulsive open source contribution. You no longer know what's yours and what's not. Tell apart the projects you're actively maintaining from that fork you made in 2010 to correct a typo.

### What it shows

`gh-wtfork` categorizes your forks into three groups:

- **Maintained** — you're ahead on the default branch (keeping your own version)
- **Contributions** — not ahead, but has branches or PRs (contributing back upstream)
- **Untouched** — no changes at all (can probably delete)

For each fork, you'll see:
- How far ahead/behind upstream, and *when* (is upstream dead? is your fork stale?)
- Your branches with age and associated PR status (open, merged, or closed)
- Whether that old branch is finished business or still pending

### Usage

```bash
# Show active forks (hides untouched ones)
gh-wtfork

# Show all forks including untouched
gh-wtfork --all

# Run as a specific identity
gh-wtfork --as work

# Output as JSON
gh-wtfork --json
```

### Example output

```
● Maintained
🍴 jdevera/command-launcher
    ↑ criteo/command-launcher
    ↑ 12 ahead (3mo ago)  ↓ 45 behind (upstream: 2d ago)
    ⎇ feature-branch  2025-10-20 · 4mo ago
        🔀 merged #89 Add self-update version comparison

○ Contributions
🍴 jdevera/acme.sh
    ↑ acmesh-official/acme.sh
    ↓ 441 behind (upstream: 2d ago)
    ⎇ multideploy-yaml  2025-08-31 · 6mo ago
        🔀 merged #4521 Add multi-deploy YAML support
    ⎇ patch-1  2025-09-01 · 6mo ago
        ✖ closed #4530 Fix typo in README
```

---

## License

MIT — Do what you want, just don't blame me if your bread burns.
