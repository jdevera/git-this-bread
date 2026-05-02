# Identity Tools

git-id, git-as, gh-as share `internal/identity` for profile management.

## Profile Storage

Profiles stored in git config as `[identity.<name>]` sections:

```
[identity "personal"]
    sshkey = ~/.ssh/id_personal
    email = me@example.com
    user = My Name
    ghuser = myusername
    ; usecustomagent = false   ; optional, see git-as below
```

## internal/identity

- `List()` — get profile names from git config
- `Get(name)` — read profile fields
- `Set(profile, opts)` — write profile, returns target file path
- `Remove(name)` — delete profile section
- `ValidateSSHKey(path)` — check file exists
- `ValidateGHUser(user)` — check gh auth status

Uses `git config --global` with `--show-origin` to detect source files.

## git-as

Sets env vars and execs git, deduping any matching keys already in the parent
env so the override actually wins after `execve`:
- GIT_SSH_COMMAND with profile's SSH key
- GIT_AUTHOR_EMAIL, GIT_COMMITTER_EMAIL
- GIT_AUTHOR_NAME, GIT_COMMITTER_NAME (if set)

By default git-as routes ssh through a per-profile sub-agent (managed via
`internal/identity/agent.go`) loaded with only the profile's key, so multiple
agent-loaded keys can't outrank the `-i` flag. The sub-agent socket lives
under `${XDG_CACHE_HOME:-~/.cache}/git-this-bread/agents/<profile>.sock`
and persists across invocations as a passphrase cache; on macOS we pass
`--apple-use-keychain` to `ssh-add` so the passphrase is also cached in
Keychain across reboots. Set `usecustomagent = false` on a profile to opt
out and use the whatever ssh-agent the shell session already has (`SSH_AUTH_SOCK`) instead.

`git-id agent list / kill <profile> / kill --all / reload <profile>` manage
the sub-agents.

## gh-as

Creates temp dir with hosts.yml selecting the profile's ghuser, sets GH_CONFIG_DIR, execs gh.
