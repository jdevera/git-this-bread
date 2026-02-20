package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/identity"
)

var rootCmd = &cobra.Command{
	Use:   "gh-as <profile> [gh args...]",
	Short: "Run gh (GitHub CLI) commands with a specific identity profile",
	Long: `gh-as (a git-this-bread tool)

Run gh (GitHub CLI) commands with a specific identity profile.

The profile must have 'ghuser' configured and authenticated.
Use 'git-id' to manage profiles.`,
	Example: `  gh-as personal pr list
  gh-as work issue create
  gh-as personal repo clone owner/repo`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true, // Pass all flags to gh
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
	ghArgs := args[1:]

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
	if dir := os.Getenv("GH_CONFIG_DIR"); dir != "" {
		return dir
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gh")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh")
}
