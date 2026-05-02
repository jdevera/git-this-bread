# git-as

Run git commands with a specific identity profile.

```bash
git-as personal clone git@github.com:user/repo.git
git-as work push origin main
git-as personal commit -m "Fix bug"
```

`git-as` is the runtime side of the identity tools: it loads a profile (managed by [`git-id`](git-id.md)), wires the right SSH key and authorship into the environment, and execs `git`. The current process is replaced — `git-as` itself doesn't sit in the call chain after exec.

---

## Quick start

```bash
# 1. Set up a profile (interactive)
git-id add personal

# 2. Use it
git-as personal status
git-as personal push -u origin some-branch

# 3. Inspect what's running
git-id agent list
```

Profile names are arbitrary; `personal`/`work` is just a common pattern.

## Mental model: identity isolation

The hard problem `git-as` solves is *picking a specific SSH key when the system `ssh-agent` has multiple keys loaded*. Stock OpenSSH does not provide a "use the agent but only key X" directive: agent-loaded keys outrank `-i` regardless of `IdentitiesOnly=yes`. Without active isolation, the wrong key is offered first, and the remote authenticates as the wrong account — usually surfacing as a confusing `Permission denied to <other-user>` error.

`git-as` defaults to a per-profile **managed sub-agent**: a separate `ssh-agent` process whose only loaded identity is the profile's key. The system agent stays untouched (plain `git push`, plain `ssh github.com`, every other tool: same as before), but anything routed through `git-as` is force-fed the profile's key.

```
parent shell                                        ssh-agent (system)
   │                                                 ├── work-key
   │                                                 └── personal-key
   ▼
   git as personal push
   │
   ├── identity.Ensure ───── ssh-agent ───────────► sub-agent /tmp/gtb-501/personal.sock
   │                          (only personal-key)
   │
   └── exec git, with:
       GIT_SSH_COMMAND = ssh -i ~/.ssh/personal -o IdentitiesOnly=yes \
                            -o IdentityAgent=/tmp/gtb-501/personal.sock
```

The sub-agent persists across `git-as` invocations on purpose — that's the passphrase cache. First invocation with an encrypted key prompts once; subsequent invocations are silent until the agent is killed or the machine reboots.

## The sub-agent in detail

Each profile gets its own sub-agent:

| File | Role |
| --- | --- |
| `<dir>/<profile>.sock` | UNIX socket the sub-agent listens on |
| `<dir>/<profile>.pid`  | sub-agent's PID, written when spawned |
| `<dir>/<profile>.lock` | advisory `flock` for spawn-and-load critical section |

`<dir>` is normally `${XDG_CACHE_HOME:-~/.cache}/git-this-bread/agents/`. If that path would push the socket past the ~104-byte `sun_path` limit (mostly a problem on macOS with deep `$TMPDIR`), `git-as` falls back to `/tmp/gtb-<uid>/`.

### Lifecycle on each invocation

1. **Probe** the socket via `ssh-add -l`. Alive → check whether the profile's pubkey is loaded.
2. **Lock** `<profile>.lock` if anything needs to change, to keep concurrent invocations from racing.
3. **Spawn** a new `ssh-agent` if the probe failed. **Load** the profile's key with `ssh-add` if it isn't already loaded.
4. Set `GIT_SSH_COMMAND` with `IdentityAgent=<sock>` and exec `git`.

Steady state — agent already running with key loaded — costs one `ssh-add -l` call (sub-millisecond).

### Passphrase ergonomics

| Platform | Behavior |
| --- | --- |
| **macOS** | `ssh-add --apple-use-keychain` is passed automatically. First-ever invocation prompts; passphrase is cached in Keychain; *every* subsequent invocation is silent — including across reboots. |
| **Linux with gnome-keyring / ksshaskpass** | We inherit the user's `SSH_ASKPASS` chain, so the passphrase is cached by the same daemon that handles the system agent. Silent across reboots once configured. |
| **Linux without a keystore** | One passphrase prompt per agent restart. The sub-agent persists across `git-as` invocations, so within a session it's "prompt once, cached forever." Reboot → prompt once again. |

If your encrypted key prompts more often than expected, see [Troubleshooting](#troubleshooting) below.

## Disabling the sub-agent

Set `usecustomagent = false` on a profile:

```bash
git-id set personal usecustomagent false
```

In that mode, `git-as` does not spawn or talk to a sub-agent. It sets `GIT_SSH_COMMAND="ssh -i <key> -o IdentitiesOnly=yes"` and lets whatever ssh-agent your shell session already has — `SSH_AUTH_SOCK`, whether that's the OS's launchd/systemd ssh-agent, 1Password, KeePassXC, gpg-agent, or one you started by hand — take over.

Use this when:

- Your system agent is itself per-key isolated (1Password, KeePassXC, gpg-agent with destination constraints) — wrapping it in a sub-agent is redundant.
- You're debugging an agent-related issue and want to bypass `git-as`'s management entirely.

When you opt out, the original SSH-agent ordering caveats apply: an agent-backed key registered on a different account on the same host can still authenticate as the wrong account. The whole point of the default sub-agent is to make that class of failure impossible by construction.

## Environment

`git-as` sets these variables on the exec'd `git` (any colliding values from the parent env are dropped first, so a parent-set `GIT_SSH_COMMAND` can't silently survive `execve`):

| Variable | Value |
| --- | --- |
| `GIT_SSH_COMMAND`     | `ssh -i <key> -o IdentitiesOnly=yes -o IdentityAgent=<sock>` (or without `IdentityAgent` when `usecustomagent=false`) |
| `GIT_AUTHOR_EMAIL`    | profile `email` |
| `GIT_COMMITTER_EMAIL` | profile `email` |
| `GIT_AUTHOR_NAME`     | profile `name` (display name) or `user` (fallback), if set |
| `GIT_COMMITTER_NAME`  | same |

## Managing sub-agents

```bash
git-id agent list           # show running sub-agents (alive/dead, PID, key count)
git-id agent kill <profile> # terminate a profile's sub-agent
git-id agent kill --all     # terminate every sub-agent under the cache dir
git-id agent reload <profile>  # re-run ssh-add against the existing sub-agent
```

Killing is non-destructive — the next `git-as` invocation will respawn. Useful when:

- You rotated the profile's SSH key and want the agent to pick up the new one.
- Some other tool is misbehaving and you want a clean slate.
- You're worried about agent state crossing some trust boundary.

## Troubleshooting

### `Permission denied to <other-user>` on `git-as personal push`

Run `git-id agent list` and `git-id show personal`. If the sub-agent has the right key but you're still getting the wrong account, double-check that `usecustomagent` isn't accidentally set to false:

```bash
git config --get-all identity.personal.usecustomagent
```

Empty output means the default (`true`) is in effect. If it prints `false`, that's why isolation isn't happening.

### "I keep getting prompted for my passphrase"

Check `git-id agent list`. If the agent dies between invocations, every fresh spawn re-prompts. Likely causes:
- Your shell or session manager is killing background processes on exit.
- `git-id agent kill` is being run somewhere unexpectedly.
- The socket file got cleaned up by a `/tmp` reaper (only relevant under the `/tmp/gtb-<uid>/` fallback).

On macOS, `--apple-use-keychain` should make this irrelevant — the prompt should appear only once ever. If it doesn't, your key may not actually be encrypted, or `ssh-add` may be using a different keychain entry than expected. `security find-internet-password -s "ssh-agent" -a <key path>` can confirm.

### `unix_listener: path … too long for Unix domain socket`

Should not happen with `git-as` — the `/tmp/gtb-<uid>/` fallback handles long cache paths. If you see this anyway, your `/tmp` is somehow unwritable; set `XDG_CACHE_HOME` to a short path and try again.

### "Where did my profile go?"

Profiles live in your git config (default: `~/.gitconfig`) under `[identity "<name>"]` sections. They follow git's normal `[include]` semantics — a profile in an included file is fully usable. `git-id show <name>` prints the source file.

### Plain `git push` (no `git-as`) authenticates as the wrong account

That's working as intended: `git-as` only affects invocations that go through it. Plain `git` continues to use whatever the system agent offers first. If you want a specific repo to use a profile's identity automatically without typing `git as`, that's a separate feature (see ["Just-works in this repo" wishlist](../wishlist.md), if/when it lands) — `git-as` is for explicit, per-command identity selection.

## Related

- [`git-id`](git-id.md) — manage profiles (the configuration side)
