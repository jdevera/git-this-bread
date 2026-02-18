package identity

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ValidateSSHKey checks that the SSH key file exists and is readable.
func ValidateSSHKey(path string) error {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand ~: %w", err)
		}
		path = home + path[1:]
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("SSH key not found: %s", path)
		}
		return fmt.Errorf("cannot access SSH key: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("SSH key path is a directory: %s", path)
	}

	// Check readable
	f, err := os.Open(path) //nolint:gosec // path validated above
	if err != nil {
		return fmt.Errorf("SSH key not readable: %w", err)
	}
	_ = f.Close()

	return nil
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}

// ValidateGHUser checks that the GitHub user is authenticated with gh CLI.
func ValidateGHUser(username string) error {
	cmd := exec.Command("gh", "auth", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh auth failed: %w", err)
	}

	// Parse output to find the user
	// gh auth status output format includes "Logged in to github.com account <username>"
	output := string(out)
	if !strings.Contains(output, username) {
		return fmt.Errorf("GitHub user %q not authenticated. Run: gh auth login", username)
	}

	return nil
}

// CheckGHUserStatus returns detailed auth status for a GitHub user.
type GHAuthStatus struct {
	Authenticated bool
	Message       string
}

// GetGHAuthStatus checks the authentication status for a GitHub user.
func GetGHAuthStatus(username string) GHAuthStatus {
	if username == "" {
		return GHAuthStatus{
			Authenticated: false,
			Message:       "GitHub user not set",
		}
	}

	cmd := exec.Command("gh", "auth", "status")
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if strings.Contains(output, username) {
		return GHAuthStatus{
			Authenticated: true,
			Message:       "authenticated",
		}
	}

	return GHAuthStatus{
		Authenticated: false,
		Message:       "not authenticated. Run: gh auth login",
	}
}
