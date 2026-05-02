//go:build unix

package identity

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// SubAgent is a per-profile ssh-agent process whose only loaded key is the
// profile's `sshkey`. The system ssh-agent is untouched; only ssh invocations
// pointed at SubAgent.Socket via IdentityAgent see this agent.
//
// Sub-agents persist across `git as` invocations on purpose — that's the
// passphrase cache. They're cleaned up by `git-id agent kill`, by reboot, or
// by an explicit signal.
type SubAgent struct {
	Profile  *Profile
	Socket   string // <agentDir>/<profile>.sock
	PIDFile  string // <agentDir>/<profile>.pid
	LockFile string // <agentDir>/<profile>.lock
}

// Ensure returns a ready-to-use sub-agent for the profile. It probes the
// existing socket; spawns a new agent if needed; loads the profile's key if
// the agent doesn't have it. The first call for an encrypted key prompts for
// the passphrase; subsequent calls (within the agent's lifetime, or across
// agent restarts when the OS keychain has the passphrase cached) are silent.
func Ensure(p *Profile) (*SubAgent, error) {
	s, err := newSubAgent(p)
	if err != nil {
		return nil, err
	}

	// The fast path is an alive agent with the right key already loaded — just
	// the IsAlive probe + a HasKey probe, no lock.
	if s.IsAlive() {
		has, err := s.HasKey()
		if err != nil {
			return nil, fmt.Errorf("checking sub-agent for profile %q: %w", p.Name, err)
		}
		if has {
			return s, nil
		}
	}

	// Either the agent is dead or the key isn't loaded. Both fixes mutate
	// shared state under the lock to keep concurrent `git as` invocations
	// from racing on spawn or duplicate ssh-add prompts.
	unlock, err := flockExclusive(s.LockFile)
	if err != nil {
		return nil, fmt.Errorf("locking sub-agent for profile %q: %w", p.Name, err)
	}
	defer unlock()

	// Re-check liveness with the lock held — a concurrent caller may have
	// spawned the agent between our probe and our acquire.
	if !s.IsAlive() {
		if err := s.spawn(); err != nil {
			return nil, fmt.Errorf("spawning sub-agent for profile %q: %w", p.Name, err)
		}
	}

	has, err := s.HasKey()
	if err != nil {
		return nil, fmt.Errorf("checking sub-agent for profile %q: %w", p.Name, err)
	}
	if !has {
		if err := s.LoadKey(); err != nil {
			return nil, fmt.Errorf("loading key into sub-agent for profile %q: %w", p.Name, err)
		}
	}
	return s, nil
}

// List returns sub-agents for all profiles that have files under the agents
// cache dir. Live and dead agents are both included; callers can filter via
// IsAlive.
func ListAgents() ([]*SubAgent, error) {
	dir, err := agentDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var agents []*SubAgent
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".sock"), ".pid"), ".lock")
		if base == name || seen[base] {
			continue
		}
		seen[base] = true
		// We don't need the full profile here — caller asks Get if they care.
		s := &SubAgent{
			Profile:  &Profile{Name: base},
			Socket:   filepath.Join(dir, base+".sock"),
			PIDFile:  filepath.Join(dir, base+".pid"),
			LockFile: filepath.Join(dir, base+".lock"),
		}
		agents = append(agents, s)
	}
	return agents, nil
}

// IsAlive returns true when ssh-add can talk to the socket. Exit 0 (keys
// listed) and exit 1 (no keys loaded) both indicate an alive agent; anything
// else means the socket is dead or never existed.
func (s *SubAgent) IsAlive() bool {
	cmd := exec.Command("ssh-add", "-l")
	cmd.Env = appendEnv(os.Environ(), "SSH_AUTH_SOCK="+s.Socket)
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true
	}
	return false
}

// HasKey returns true when the sub-agent currently holds the profile's key.
// Returns (false, nil) when the agent is alive but empty.
func (s *SubAgent) HasKey() (bool, error) {
	target, err := pubkeyForPrivate(ExpandPath(s.Profile.SSHKey))
	if err != nil {
		return false, err
	}
	loaded, err := agentPubkeys(s.Socket)
	if err != nil {
		return false, err
	}
	for _, l := range loaded {
		if l == target {
			return true, nil
		}
	}
	return false, nil
}

// LoadKey runs `ssh-add` against the sub-agent. On macOS the key's passphrase
// is stored in Keychain on first add (if encrypted), so subsequent reloads
// after agent restart are silent. On Linux we inherit the user's SSH_ASKPASS
// chain (gnome-keyring, ksshaskpass, etc.) — no extra wiring required.
func (s *SubAgent) LoadKey() error {
	keyPath := ExpandPath(s.Profile.SSHKey)
	args := []string{}
	if runtime.GOOS == "darwin" {
		args = append(args, "--apple-use-keychain")
	}
	args = append(args, keyPath)

	cmd := exec.Command("ssh-add", args...)
	cmd.Env = appendEnv(os.Environ(), "SSH_AUTH_SOCK="+s.Socket)
	// Connect stdio so the user can see and answer the passphrase prompt.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-add %s: %w", keyPath, err)
	}
	return nil
}

// Kill terminates the sub-agent process and removes its files. If the agent
// is already dead, the files are still cleaned up.
func (s *SubAgent) Kill() error {
	if pid, err := s.readPID(); err == nil && pid > 0 {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
	_ = os.Remove(s.Socket)
	_ = os.Remove(s.PIDFile)
	_ = os.Remove(s.LockFile)
	return nil
}

// PID reads the recorded ssh-agent PID, or 0 if unavailable.
func (s *SubAgent) PID() int {
	pid, err := s.readPID()
	if err != nil {
		return 0
	}
	return pid
}

// LoadedKeys returns the public keys currently loaded in the sub-agent in
// `<algo> <base64>` form. Empty slice means "agent alive but empty"; an error
// means the socket couldn't be queried at all.
func (s *SubAgent) LoadedKeys() ([]string, error) {
	return agentPubkeys(s.Socket)
}

// newSubAgent prepares the file paths for a profile's sub-agent. It does not
// spawn anything.
func newSubAgent(p *Profile) (*SubAgent, error) {
	if p.SSHKey == "" {
		return nil, fmt.Errorf("profile %q has no sshkey", p.Name)
	}
	dir, err := agentDir()
	if err != nil {
		return nil, err
	}
	return &SubAgent{
		Profile:  p,
		Socket:   filepath.Join(dir, p.Name+".sock"),
		PIDFile:  filepath.Join(dir, p.Name+".pid"),
		LockFile: filepath.Join(dir, p.Name+".lock"),
	}, nil
}

// spawn starts a fresh ssh-agent listening on s.Socket and records its PID.
// Caller must hold the lock.
func (s *SubAgent) spawn() error {
	// Stale socket file from a previous agent that died: ssh-agent refuses to
	// bind on top of an existing path, so we clear it first.
	_ = os.Remove(s.Socket)

	cmd := exec.Command("ssh-agent", "-a", s.Socket)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-agent -a %s: %w", s.Socket, err)
	}
	pid := parseAgentPID(stdout.String())
	if pid == 0 {
		return fmt.Errorf("ssh-agent did not print SSH_AGENT_PID")
	}
	return os.WriteFile(s.PIDFile, []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

// readPID reads the agent's PID from the recorded file.
func (s *SubAgent) readPID() (int, error) {
	b, err := os.ReadFile(s.PIDFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// sunPathLimit is the conservative max for sockaddr_un.sun_path that works on
// both macOS (104) and Linux (108). Includes the trailing NUL.
const sunPathLimit = 100

// agentDir returns the directory for sub-agent files, creating it if needed.
// Prefers `${XDG_CACHE_HOME:-~/.cache}/git-this-bread/agents/`; falls back to
// `/tmp/gtb-<uid>/` if the cache path would push a socket past sunPathLimit
// (sub-agents would otherwise refuse to bind).
func agentDir() (string, error) {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	primary := filepath.Join(cacheHome, "git-this-bread", "agents")

	// Reserve room for "<profile>.sock". Profile names are user-controlled but
	// typically short; reserving 32 bytes leaves headroom for typical names
	// while still letting unusually-long ones trigger the fallback explicitly.
	if len(primary)+1+32 < sunPathLimit {
		if err := os.MkdirAll(primary, 0o700); err == nil { //nolint:gosec // primary is derived from XDG_CACHE_HOME / $HOME, both user-owned
			return primary, nil
		}
	}
	// Fallback: a per-user dir directly under /tmp keeps the path short on
	// systems with deep HOME or $TMPDIR (notably macOS, where $TMPDIR can be
	// >50 chars on its own).
	fallback := fmt.Sprintf("/tmp/gtb-%d", os.Getuid())
	if err := os.MkdirAll(fallback, 0o700); err != nil {
		return "", err
	}
	return fallback, nil
}

// pubkeyForPrivate returns the canonicalized "<algo> <base64>" form of the
// public key matching the private key at path. Prefers the conventional
// `<path>.pub` sidecar file (always unencrypted) over `ssh-keygen -y`, which
// prompts for the passphrase on encrypted keys — wasteful and broken under
// non-TTY callers like our HasKey probe.
func pubkeyForPrivate(path string) (string, error) {
	if data, err := os.ReadFile(path + ".pub"); err == nil { //nolint:gosec // path is the user's own configured ssh key
		if k := canonPubkey(string(data)); k != "" {
			return k, nil
		}
	}
	cmd := exec.Command("ssh-keygen", "-y", "-f", path)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("derive pubkey for %s (no %s.pub sidecar): %w (%s)",
			path, path, err, strings.TrimSpace(errBuf.String()))
	}
	return canonPubkey(out.String()), nil
}

// agentPubkeys queries `ssh-add -L` against the given socket and returns each
// loaded key in canonical "<algo> <base64>" form. Exit 1 (empty agent) is not
// an error.
func agentPubkeys(socket string) ([]string, error) {
	cmd := exec.Command("ssh-add", "-L")
	cmd.Env = appendEnv(os.Environ(), "SSH_AUTH_SOCK="+socket)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ssh-add -L: %w", err)
	}
	var keys []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		if k := canonPubkey(scanner.Text()); k != "" {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// canonPubkey returns "<algo> <base64>" — the first two whitespace-separated
// fields of an OpenSSH public key line. Comment field (third field, if any) is
// dropped so equality compares key material only.
func canonPubkey(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 {
		return ""
	}
	return fields[0] + " " + fields[1]
}

// parseAgentPID extracts the integer PID from `ssh-agent -a` output, which
// looks like:
//
//	SSH_AUTH_SOCK=/tmp/...; export SSH_AUTH_SOCK;
//	SSH_AGENT_PID=12345; export SSH_AGENT_PID;
//	echo Agent pid 12345;
func parseAgentPID(out string) int {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "SSH_AGENT_PID="
		i := strings.Index(line, prefix)
		if i < 0 {
			continue
		}
		rest := line[i+len(prefix):]
		if j := strings.IndexAny(rest, "; \t"); j >= 0 {
			rest = rest[:j]
		}
		if pid, err := strconv.Atoi(rest); err == nil {
			return pid
		}
	}
	return 0
}

// flockExclusive acquires an exclusive advisory lock on path. Returned func
// releases the lock and closes the fd.
func flockExclusive(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // path is the agent lock file under our cache dir
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // file descriptors fit in int on every supported platform
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // see above
		_ = f.Close()
	}, nil
}

// appendEnv replaces or appends a "K=V" entry, honoring the dedup invariant
// that build-env enforces on the git exec path. Used here for ssh-add /
// ssh-add -L invocations whose env we control.
func appendEnv(env []string, kv string) []string {
	name, _, _ := strings.Cut(kv, "=")
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if k, _, _ := strings.Cut(e, "="); k == name {
			continue
		}
		out = append(out, e)
	}
	return append(out, kv)
}
