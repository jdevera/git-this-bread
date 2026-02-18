// Package identity provides profile management for git/GitHub identities.
package identity

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Profile represents a git/GitHub identity profile.
type Profile struct {
	Name   string // Profile name (e.g., "personal", "work")
	SSHKey string // Path to SSH private key (required for git-as)
	Email  string // Git author/committer email (required for git-as)
	User   string // Git author/committer name (optional)
	GHUser string // GitHub username for gh-as (optional)
}

// profileKeys are the git config keys used for profile fields.
var profileKeys = []string{"sshkey", "email", "user", "ghuser"}

// List returns all profile names from git config.
func List() ([]string, error) {
	cmd := exec.Command("git", "config", "--global", "--get-regexp", `^identity\.`)
	out, err := cmd.Output()
	if err != nil {
		// No matches is not an error - just empty
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("git config failed: %w", err)
	}

	// Parse output to extract unique profile names
	// Format: identity.<name>.<key> <value>
	seen := make(map[string]bool)
	var names []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}
		key := parts[0]
		// identity.<name>.<field>
		keyParts := strings.Split(key, ".")
		if len(keyParts) >= 2 {
			name := keyParts[1]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names, nil
}

// Get reads a profile from git config.
func Get(name string) (*Profile, error) {
	p := &Profile{Name: name}

	// Read each field
	if val, err := getConfigValue(name, "sshkey"); err == nil {
		p.SSHKey = val
	}
	if val, err := getConfigValue(name, "email"); err == nil {
		p.Email = val
	}
	if val, err := getConfigValue(name, "user"); err == nil {
		p.User = val
	}
	if val, err := getConfigValue(name, "ghuser"); err == nil {
		p.GHUser = val
	}

	// Check if profile exists (has at least one field)
	if p.SSHKey == "" && p.Email == "" && p.User == "" && p.GHUser == "" {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	return p, nil
}

// getConfigValue reads a single config value.
func getConfigValue(profile, key string) (string, error) {
	configKey := fmt.Sprintf("identity.%s.%s", profile, key)
	cmd := exec.Command("git", "config", "--global", "--get", configKey)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetSourceFile returns the file where a profile is defined using --show-origin.
func GetSourceFile(name string) (string, error) {
	// Try to find any key for this profile
	for _, key := range profileKeys {
		configKey := fmt.Sprintf("identity.%s.%s", name, key)
		cmd := exec.Command("git", "config", "--global", "--show-origin", "--get", configKey)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		// Format: file:<path>\t<value>
		line := strings.TrimSpace(string(out))
		if strings.HasPrefix(line, "file:") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) >= 1 {
				return strings.TrimPrefix(parts[0], "file:"), nil
			}
		}
	}
	return "", fmt.Errorf("profile %q not found in any config file", name)
}

// GetAllSourceFiles returns all files where a profile has keys defined.
func GetAllSourceFiles(name string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)

	for _, key := range profileKeys {
		configKey := fmt.Sprintf("identity.%s.%s", name, key)
		cmd := exec.Command("git", "config", "--global", "--show-origin", "--get-all", configKey)
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "file:") {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) >= 1 {
					path := strings.TrimPrefix(parts[0], "file:")
					if !seen[path] {
						seen[path] = true
						files = append(files, path)
					}
				}
			}
		}
	}

	return files, nil
}

// SetOptions controls how Set behaves.
type SetOptions struct {
	File     string // Explicit target file (optional)
	Yes      bool   // Auto-accept multi-file conflict prompt
	Detached bool   // Skip effectiveness check
}

// Set writes a profile to git config.
func Set(p *Profile, opts SetOptions) (string, error) {
	// Determine target file
	targetFile := opts.File
	if targetFile == "" {
		// Check if profile already exists
		existingFile, err := GetSourceFile(p.Name)
		if err == nil {
			targetFile = existingFile
		} else {
			// New profile - use default config file
			targetFile = DefaultConfigFile()
		}
	}

	// Check for conflicts if no explicit file given
	if opts.File == "" {
		files, _ := GetAllSourceFiles(p.Name)
		if len(files) > 1 {
			if !opts.Yes {
				return "", fmt.Errorf("identity exists in multiple files: %s. Use --yes to proceed or --file to specify target", strings.Join(files, ", "))
			}
			// With --yes, we use the last file (git reads last)
			targetFile = files[len(files)-1]
		}
	}

	// Write each field
	if p.SSHKey != "" {
		if err := setConfigValue(targetFile, p.Name, "sshkey", p.SSHKey); err != nil {
			return targetFile, err
		}
	}
	if p.Email != "" {
		if err := setConfigValue(targetFile, p.Name, "email", p.Email); err != nil {
			return targetFile, err
		}
	}
	if p.User != "" {
		if err := setConfigValue(targetFile, p.Name, "user", p.User); err != nil {
			return targetFile, err
		}
	}
	if p.GHUser != "" {
		if err := setConfigValue(targetFile, p.Name, "ghuser", p.GHUser); err != nil {
			return targetFile, err
		}
	}

	// Verify write succeeded by reading back from the specific file
	if err := verifyWrite(targetFile, p); err != nil {
		return targetFile, err
	}

	// Verify effectiveness (unless detached)
	if !opts.Detached {
		if err := verifyEffective(p); err != nil {
			return targetFile, fmt.Errorf("%w. If you meant to write to a file outside the git config chain, use --detached", err)
		}
	}

	return targetFile, nil
}

// setConfigValue writes a single config value to a specific file.
func setConfigValue(file, profile, key, value string) error {
	configKey := fmt.Sprintf("identity.%s.%s", profile, key)
	cmd := exec.Command("git", "config", "--file", file, configKey, value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set %s: %w", configKey, err)
	}
	return nil
}

// verifyWrite checks that the values were written to the target file.
func verifyWrite(file string, p *Profile) error {
	check := func(key, expected string) error {
		if expected == "" {
			return nil
		}
		configKey := fmt.Sprintf("identity.%s.%s", p.Name, key)
		cmd := exec.Command("git", "config", "--file", file, "--get", configKey)
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("write failed: %s not found in %s", configKey, file)
		}
		if strings.TrimSpace(string(out)) != expected {
			return fmt.Errorf("write failed: %s has unexpected value", configKey)
		}
		return nil
	}

	if err := check("sshkey", p.SSHKey); err != nil {
		return err
	}
	if err := check("email", p.Email); err != nil {
		return err
	}
	if err := check("user", p.User); err != nil {
		return err
	}
	return check("ghuser", p.GHUser)
}

// verifyEffective checks that git's merged config returns our values.
func verifyEffective(p *Profile) error {
	check := func(key, expected string) error {
		if expected == "" {
			return nil
		}
		val, err := getConfigValue(p.Name, key)
		if err != nil || val != expected {
			return fmt.Errorf("write succeeded, but another config file is overriding identity.%s.%s", p.Name, key)
		}
		return nil
	}

	if err := check("sshkey", p.SSHKey); err != nil {
		return err
	}
	if err := check("email", p.Email); err != nil {
		return err
	}
	if err := check("user", p.User); err != nil {
		return err
	}
	return check("ghuser", p.GHUser)
}

// Remove deletes a profile from its source file.
func Remove(name string) error {
	// Find which file contains the profile
	file, err := GetSourceFile(name)
	if err != nil {
		return err
	}

	section := fmt.Sprintf("identity.%s", name)
	cmd := exec.Command("git", "config", "--file", file, "--remove-section", section)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove profile %q: %w", name, err)
	}
	return nil
}

// DefaultConfigFile returns the default git config file to use.
// Prefers ~/.gitconfig if it exists, otherwise uses XDG path.
func DefaultConfigFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check if ~/.gitconfig exists
	gitconfig := filepath.Join(home, ".gitconfig")
	if _, err := os.Stat(gitconfig); err == nil {
		return gitconfig
	}

	// Use XDG path
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfig, "git", "config")
}

// SetField sets a single field on an existing profile.
func SetField(name, key, value string, opts SetOptions) (string, error) {
	// Validate key
	validKeys := map[string]bool{"sshkey": true, "email": true, "user": true, "ghuser": true}
	if !validKeys[key] {
		return "", fmt.Errorf("invalid key %q, must be one of: sshkey, email, user, ghuser", key)
	}

	// Determine target file
	targetFile := opts.File
	if targetFile == "" {
		existingFile, err := GetSourceFile(name)
		if err != nil {
			return "", fmt.Errorf("profile %q not found", name)
		}
		targetFile = existingFile
	}

	// Write the value
	if err := setConfigValue(targetFile, name, key, value); err != nil {
		return targetFile, err
	}

	// Verify write
	configKey := fmt.Sprintf("identity.%s.%s", name, key)
	cmd := exec.Command("git", "config", "--file", targetFile, "--get", configKey)
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != value {
		return targetFile, fmt.Errorf("write failed")
	}

	// Verify effectiveness
	if !opts.Detached {
		val, err := getConfigValue(name, key)
		if err != nil || val != value {
			return targetFile, fmt.Errorf("write succeeded, but another config file is overriding this value. Use --detached to skip this check")
		}
	}

	return targetFile, nil
}
