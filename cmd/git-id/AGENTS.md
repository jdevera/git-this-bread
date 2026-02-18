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

Sets env vars and execs git:
- GIT_SSH_COMMAND with profile's SSH key
- GIT_AUTHOR_EMAIL, GIT_COMMITTER_EMAIL
- GIT_AUTHOR_NAME, GIT_COMMITTER_NAME (if set)

## gh-as

Creates temp dir with hosts.yml selecting the profile's ghuser, sets GH_CONFIG_DIR, execs gh.
