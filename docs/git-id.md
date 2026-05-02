# git-id

Manage identity profiles for git and GitHub workflows.

```bash
git-id                       # list all profiles
git-id show personal         # inspect one
git-id add personal          # create interactively
git-id set personal email me@example.com
git-id remove personal
git-id agent list            # see running sub-agents (used by git-as)
```

`git-id` is the configuration side of the identity tools. Profiles live in your git config under `[identity "<name>"]` sections; runtime tools — [`git-as`](git-as.md), `gh-as` — read those profiles to know which key/email/account to use.

---

## Profile schema

Each profile is a set of fields under `identity.<name>` in git config:

| Field | Required for | Purpose |
| --- | --- | --- |
| `sshkey` | `git-as` | Path to the private SSH key. `~/...` is expanded. |
| `email`  | `git-as` | Author/committer email. |
| `name`   | optional | Display name for git commits. Overrides `user`. |
| `user`   | optional | Fallback display name. |
| `ghuser` | `gh-as`  | GitHub username; must be authenticated via `gh auth login`. |
| `usecustomagent` | `git-as` | Default `true`. Set to `false` to opt out of the per-profile sub-agent. See [git-as docs](git-as.md#disabling-the-sub-agent). |

The on-disk form looks like:

```ini
[identity "personal"]
    sshkey = ~/.ssh/id_personal
    email = me@example.com
    name = My Name
    ghuser = myusername
    ; usecustomagent = false   ; optional, defaults to true
```

`git-id` follows git's `[include]` semantics: a profile defined in an included file is fully usable. `git-id show <name>` prints the actual source file.

## Subcommands

### `git-id` / `git-id list`

List all profiles with their email and gh-auth status:

```
$ git-id
  personal: me@example.com (gh: myuser ✓)
  work: me@company.com (gh: work-user ✓)
```

### `git-id show <profile>`

Print full details for a profile:

```
$ git-id show personal
Profile: personal
Source:  /Users/me/.gitconfig

  name:   My Name
  sshkey: ~/.ssh/id_personal ✓
  email:  me@example.com
  user:   (not set)
  ghuser: myuser ✓ authenticated
```

The ✓ marks pass at the validation layer:

- `sshkey ✓` — the file exists and is readable. (It does *not* verify the key authenticates as the expected GitHub account.)
- `ghuser ✓ authenticated` — `gh auth status` lists this user as logged in.

If `usecustomagent` is set to `false`, that's surfaced with a ⚠. The default-true case isn't shown, since it's the default.

### `git-id add <profile>`

Interactive profile creation. Prompts for SSH key, email, optional display name and GitHub username. Validates the SSH key exists before saving.

```
$ git-id add personal
Creating profile: personal

SSH key path (required): ~/.ssh/id_personal
Email (required): me@example.com
Display name for commits (optional): My Name
User name (optional):
GitHub username (optional): myuser

Profile 'personal' saved to /Users/me/.gitconfig
```

### `git-id set <profile> <key> <value>`

Set a single field on an existing profile:

```bash
git-id set personal email new@example.com
git-id set work sshkey ~/.ssh/id_work
git-id set personal usecustomagent false
```

Valid keys: `name`, `sshkey`, `email`, `user`, `ghuser`, `usecustomagent`.

### `git-id remove <profile>`

Delete the profile section from its source file. Other identity profiles in the same file are untouched.

### `git-id agent <subcommand>`

Manage the per-profile ssh sub-agents that `git-as` spawns. See [git-as docs](git-as.md#the-sub-agent-in-detail) for the full mental model.

```bash
git-id agent list                # show running sub-agents
git-id agent kill <profile>      # terminate a profile's sub-agent
git-id agent kill --all          # terminate every sub-agent
git-id agent reload <profile>    # re-run ssh-add against the existing sub-agent
```

`agent list` columns: profile name, alive/dead, PID, loaded key count, socket path.

```
$ git-id agent list
  personal	alive	pid=21239	keys=1	socket=/Users/me/.cache/git-this-bread/agents/personal.sock
  work	alive	pid=21412	keys=1	socket=/Users/me/.cache/git-this-bread/agents/work.sock
```

## Storage and conflicts

By default, `git-id add`/`set` writes to the file where the profile already exists; for new profiles, it writes to `~/.gitconfig` (or `${XDG_CONFIG_HOME}/git/config` if `~/.gitconfig` doesn't exist).

Three flags control the write target:

| Flag | Effect |
| --- | --- |
| `--file <path>` | Write to a specific file. |
| `--yes`         | Auto-accept when the same profile name exists in multiple included files (writes to the last one — git's read-precedence rule). |
| `--detached`    | Skip the post-write effectiveness check. Use when writing to a file outside git's normal config chain. |

Without `--file`, multi-file conflicts produce an error so you don't silently shadow one definition with another:

```
identity exists in multiple files: /Users/me/.gitconfig, /Users/me/.git-identities. Use --yes to proceed or --file to specify target
```

Effectiveness check: after writing, `git-id` reads the value back through git's *merged* config. If something else in the chain overrides the value, the write is reported as failed even though the file on disk is correct. `--detached` skips this when you actually want to write to a file that isn't in the chain.

## Common workflows

### Adding a personal identity to a work machine

```bash
git-id add jdevera
# SSH key path: ~/.ssh/id_personal_github
# Email: me@personal.example.com
# Display name: Real Name
# GitHub username: jdevera
```

Then commit through `git-as`:

```bash
git-as jdevera commit -m "fix typo"
git-as jdevera push
```

The first push prompts once for the SSH key passphrase (cached in Keychain on macOS, or wherever your `SSH_ASKPASS` chain stores it on Linux); subsequent pushes are silent.

### Migrating profiles between machines

Profiles live in your git config, so they sync however your dotfiles do. Store them in a separate file and reference it from `~/.gitconfig`:

```ini
; ~/.gitconfig
[include]
    path = ~/.git-identities
```

```ini
; ~/.git-identities (in your dotfiles repo)
[identity "personal"]
    sshkey = ~/.ssh/id_personal
    email  = me@personal.example.com
[identity "work"]
    sshkey = ~/.ssh/id_work
    email  = me@work.example.com
```

`git-id` reads through `[include]`s transparently. Writes default to the file the profile lives in.

### Rotating a profile's SSH key

```bash
git-id set personal sshkey ~/.ssh/id_personal_new
git-id agent reload personal     # load the new key into the existing sub-agent
```

Or kill the agent and let the next `git-as` invocation respawn it:

```bash
git-id agent kill personal
git-as personal status           # respawns, prompts once if encrypted
```

## Related

- [`git-as`](git-as.md) — run git through a profile (the runtime side)
