package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/identity"
)

var rootCmd = &cobra.Command{
	Use:   "git-as <profile> [git args...]",
	Short: "Run git commands with a specific identity profile",
	Long: `git-as (a git-this-bread tool)

Run git commands with a specific identity profile.

The profile must have 'sshkey' and 'email' configured.
Use 'git-id' to manage profiles.`,
	Example: `  git-as personal status
  git-as work push origin main
  git-as personal commit -m 'Fix bug'`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true, // Pass all flags to git
	RunE:               run,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Check for help flags manually since we disabled flag parsing
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		return cmd.Help()
	}

	if len(args) < 1 {
		return fmt.Errorf("missing profile argument")
	}

	profileName := args[0]
	gitArgs := args[1:]

	// Load the profile
	profile, err := identity.Get(profileName)
	if err != nil {
		return fmt.Errorf("%w\nUse 'git-id list' to see available profiles", err)
	}

	// Validate required fields
	if profile.SSHKey == "" {
		return fmt.Errorf("profile '%s' has no SSH key configured.\nUse: git-id set %s sshkey <path>", profileName, profileName)
	}

	if profile.Email == "" {
		return fmt.Errorf("profile '%s' has no email configured.\nUse: git-id set %s email <email>", profileName, profileName)
	}

	// Validate SSH key exists
	expandedKey := identity.ExpandPath(profile.SSHKey)
	if err := identity.ValidateSSHKey(profile.SSHKey); err != nil {
		return err
	}

	// Resolve the agent socket path: empty means "use whatever the user has
	// configured outside git-this-bread" (no IdentityAgent override). Non-empty
	// means git-as will route ssh through a per-profile sub-agent that holds
	// only the profile's key.
	var agentSocket string
	if profile.UseCustomAgent {
		sa, err := identity.Ensure(profile)
		if err != nil {
			return fmt.Errorf("preparing sub-agent for profile %q: %w", profileName, err)
		}
		agentSocket = sa.Socket
	}

	env := buildEnv(os.Environ(), profile, expandedKey, agentSocket)

	// Find git executable
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Build args for exec (argv[0] should be the command name)
	execArgs := append([]string{"git"}, gitArgs...)

	// Replace this process with git
	if err := syscall.Exec(gitPath, execArgs, env); err != nil {
		return fmt.Errorf("failed to exec git: %w", err)
	}

	return nil // unreachable
}

// buildEnv returns the env slice for the exec'd git, with profile overrides
// applied. Any colliding key in the parent env is removed before the override
// is appended, because syscall.Exec on macOS keeps the *first* occurrence of a
// duplicated key — appending alone silently loses the override.
//
// agentSocket, if non-empty, is added as `IdentityAgent=<path>` so ssh routes
// through the per-profile sub-agent and ignores SSH_AUTH_SOCK.
func buildEnv(parentEnv []string, profile *identity.Profile, expandedKey, agentSocket string) []string {
	overrides := map[string]string{
		"GIT_SSH_COMMAND":     sshCommand(expandedKey, agentSocket),
		"GIT_AUTHOR_EMAIL":    profile.Email,
		"GIT_COMMITTER_EMAIL": profile.Email,
	}
	if commitName := profile.CommitName(); commitName != "" {
		overrides["GIT_AUTHOR_NAME"] = commitName
		overrides["GIT_COMMITTER_NAME"] = commitName
	}

	env := make([]string, 0, len(parentEnv)+len(overrides))
	for _, kv := range parentEnv {
		name, _, _ := strings.Cut(kv, "=")
		if _, replaced := overrides[name]; !replaced {
			env = append(env, kv)
		}
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// sshCommand builds the GIT_SSH_COMMAND value. With a sub-agent socket,
// IdentityAgent overrides the ambient SSH_AUTH_SOCK so only the profile's key
// is offered. Without one, ssh falls back to the user's ambient agent (or
// no agent), with IdentitiesOnly still pinning to the -i key.
func sshCommand(expandedKey, agentSocket string) string {
	if agentSocket != "" {
		return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o IdentityAgent=%s", expandedKey, agentSocket)
	}
	return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", expandedKey)
}
