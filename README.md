# 🍞 git-this-bread

> *Let's git this bread* — tools for developers who knead to understand their git repos

[![Go Version](https://img.shields.io/github/go-mod/go-version/jdevera/git-this-bread)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/jdevera/git-this-bread)](https://goreportcard.com/report/github.com/jdevera/git-this-bread)

A collection of git utilities, freshly baked in Go. Vibe-coded.

## Tools

| Tool | Description |
|------|-------------|
| [git-explain](#-git-explain) | See contribution status across repositories |
| [git-id](#-git-id) | Manage identity profiles for multi-account workflows |
| [git-as](#-git-as) | Run git commands with a specific identity |
| [gh-as](#-gh-as) | Run GitHub CLI with a specific identity |

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

**Manage git identity profiles for multi-account workflows.**

Juggling personal and work GitHub accounts? `git-id` stores identity profiles in your git config so you can switch contexts without kneading through config files.

### What it stores

Each profile can have:
- 🔑 **SSH key** — path to the private key for this identity
- 📧 **Email** — git author/committer email
- 👤 **User** — git author/committer name
- 🐙 **GitHub user** — username for `gh-as`

### Installation

```bash
go install github.com/jdevera/git-this-bread/cmd/git-id@latest
```

### Usage

```bash
# List all profiles
git-id

# Create a new profile interactively
git-id add personal

# Show profile details
git-id show personal

# Set a single field
git-id set personal email me@example.com

# Remove a profile
git-id remove personal
```

### Example output

```
$ git-id
  personal: me@example.com (gh: myuser ✓)
  work: me@company.com (gh: work-user ✓)

$ git-id show personal
Profile: personal
Source:  /Users/me/.gitconfig

  sshkey: ~/.ssh/id_personal ✓
  email:  me@example.com
  user:   My Name
  ghuser: myuser ✓ authenticated
```

---

## 🥨 git-as

**Run git commands with a specific identity.**

Use your identity profiles to run git commands with the right SSH key and email — no more pushing with the wrong account.

### Installation

```bash
go install github.com/jdevera/git-this-bread/cmd/git-as@latest
```

### Usage

```bash
# Clone with your personal identity
git-as personal clone git@github.com:user/repo.git

# Push with your work identity
git-as work push origin main

# Commit as a specific identity
git-as personal commit -m "Fix bug"
```

### How it works

`git-as` sets environment variables and execs git:
- `GIT_SSH_COMMAND` — uses the profile's SSH key
- `GIT_AUTHOR_EMAIL` / `GIT_COMMITTER_EMAIL` — uses the profile's email
- `GIT_AUTHOR_NAME` / `GIT_COMMITTER_NAME` — uses the profile's name (if set)

---

## 🥞 gh-as

**Run GitHub CLI commands with a specific identity.**

Switch between authenticated GitHub accounts for `gh` commands.

### Installation

```bash
go install github.com/jdevera/git-this-bread/cmd/gh-as@latest
```

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

## License

MIT — Do what you want, just don't blame me if your bread burns.
