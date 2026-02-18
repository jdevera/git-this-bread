package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/jdevera/git-this-bread/internal/identity"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gh-as <profile> [gh args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run gh (GitHub CLI) commands with a specific identity profile.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The profile must have 'ghuser' configured and authenticated.")
		fmt.Fprintln(os.Stderr, "Use 'git-id' to manage profiles.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gh-as personal pr list")
		fmt.Fprintln(os.Stderr, "  gh-as work issue create")
		fmt.Fprintln(os.Stderr, "  gh-as personal repo clone owner/repo")
		return fmt.Errorf("missing profile argument")
	}

	profileName := os.Args[1]
	ghArgs := os.Args[2:]

	// Load the profile
	profile, err := identity.Get(profileName)
	if err != nil {
		return fmt.Errorf("%w\nUse 'git-id list' to see available profiles", err)
	}

	// Validate GHUser is set
	if profile.GHUser == "" {
		return fmt.Errorf("profile '%s' has no GitHub user configured.\nUse: git-id set %s ghuser <username>", profileName, profileName)
	}

	// Validate user is authenticated
	if err := identity.ValidateGHUser(profile.GHUser); err != nil {
		return err
	}

	// Find the real gh config directory
	realConfigDir := getGHConfigDir()

	// Create temp directory for our modified config
	// Note: This temp dir is intentionally not cleaned up with defer because
	// syscall.Exec replaces the process. The temp dir will be cleaned up by
	// the OS eventually, or we could use a fixed location in the future.
	tmpDir, err := os.MkdirTemp("", "gh-as-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Symlink config.yml from real config dir if it exists
	realConfig := filepath.Join(realConfigDir, "config.yml")
	if _, err := os.Stat(realConfig); err == nil {
		tmpConfig := filepath.Join(tmpDir, "config.yml")
		if err := os.Symlink(realConfig, tmpConfig); err != nil {
			_ = os.RemoveAll(tmpDir)
			return fmt.Errorf("failed to symlink config: %w", err)
		}
	}

	// Write minimal hosts.yml that selects our user
	hostsContent := fmt.Sprintf(`github.com:
    git_protocol: ssh
    users:
        %s:
    user: %s
`, profile.GHUser, profile.GHUser)

	hostsFile := filepath.Join(tmpDir, "hosts.yml")
	if err := os.WriteFile(hostsFile, []byte(hostsContent), 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to write hosts.yml: %w", err)
	}

	// Find gh executable
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("gh not found in PATH")
	}

	// Build environment with GH_CONFIG_DIR override
	env := append(os.Environ(), fmt.Sprintf("GH_CONFIG_DIR=%s", tmpDir))

	// Build args for exec
	execArgs := append([]string{"gh"}, ghArgs...)

	// Replace this process with gh
	// Note: If this succeeds, it never returns. If it fails, we clean up.
	if err := syscall.Exec(ghPath, execArgs, env); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to exec gh: %w", err)
	}

	return nil // unreachable
}

// getGHConfigDir returns the gh CLI config directory.
func getGHConfigDir() string {
	// Check GH_CONFIG_DIR first
	if dir := os.Getenv("GH_CONFIG_DIR"); dir != "" {
		return dir
	}

	// Check XDG_CONFIG_HOME
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gh")
	}

	// Default to ~/.config/gh
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh")
}
