package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/jdevera/git-this-bread/internal/identity"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: git-as <profile> [git args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run git commands with a specific identity profile.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The profile must have 'sshkey' and 'email' configured.")
		fmt.Fprintln(os.Stderr, "Use 'git-id' to manage profiles.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  git-as personal status")
		fmt.Fprintln(os.Stderr, "  git-as work push origin main")
		fmt.Fprintln(os.Stderr, "  git-as personal commit -m 'Fix bug'")
		os.Exit(1)
	}

	profileName := os.Args[1]
	gitArgs := os.Args[2:]

	// Load the profile
	profile, err := identity.Get(profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Use 'git-id list' to see available profiles.\n")
		os.Exit(1)
	}

	// Validate required fields
	if profile.SSHKey == "" {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' has no SSH key configured.\n", profileName)
		fmt.Fprintf(os.Stderr, "Use: git-id set %s sshkey <path>\n", profileName)
		os.Exit(1)
	}

	if profile.Email == "" {
		fmt.Fprintf(os.Stderr, "Error: profile '%s' has no email configured.\n", profileName)
		fmt.Fprintf(os.Stderr, "Use: git-id set %s email <email>\n", profileName)
		os.Exit(1)
	}

	// Validate SSH key exists
	expandedKey := identity.ExpandPath(profile.SSHKey)
	if err := identity.ValidateSSHKey(profile.SSHKey); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error: git not found in PATH\n")
		os.Exit(1)
	}

	// Build args for exec (argv[0] should be the command name)
	execArgs := append([]string{"git"}, gitArgs...)

	// Replace this process with git
	if err := syscall.Exec(gitPath, execArgs, env); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to exec git: %v\n", err)
		os.Exit(1)
	}
}
