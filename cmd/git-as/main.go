package main

import (
	"fmt"
	"os"
	"os/exec"
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

	// Build environment with identity overrides
	env := append(os.Environ(),
		fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o IdentitiesOnly=yes", expandedKey),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", profile.Email),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", profile.Email),
	)

	if profile.User != "" {
		env = append(env,
			fmt.Sprintf("GIT_AUTHOR_NAME=%s", profile.User),
			fmt.Sprintf("GIT_COMMITTER_NAME=%s", profile.User),
		)
	}

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
