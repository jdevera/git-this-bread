package identity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdevera/git-this-bread/testutil"
)

// setEnv is a test helper that sets an env var and returns a cleanup function.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	orig := os.Getenv(key)
	require.NoError(t, os.Setenv(key, value))
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestListEmpty(t *testing.T) {
	// Create a temp config file with no identity profiles
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gitconfig")
	require.NoError(t, os.WriteFile(configFile, []byte("[user]\n\tname = Test\n"), 0o600))

	// Override HOME to isolate from user's real config
	setEnv(t, "HOME", tmpDir)

	// Create empty .gitconfig
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(""), 0o600))

	names, err := List()
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")

	// Create empty config file
	require.NoError(t, os.WriteFile(configFile, []byte(""), 0o600))

	// Override HOME
	setEnv(t, "HOME", tmpDir)

	// Create a profile
	p := &Profile{
		Name:   "test",
		SSHKey: "~/.ssh/id_test",
		Email:  "test@example.com",
		User:   "Test User",
		GHUser: "testuser",
	}

	file, err := Set(p, SetOptions{Detached: true})
	require.NoError(t, err)
	assert.Equal(t, configFile, file)

	// Read it back
	got, err := Get("test")
	require.NoError(t, err)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.SSHKey, got.SSHKey)
	assert.Equal(t, p.Email, got.Email)
	assert.Equal(t, p.User, got.User)
	assert.Equal(t, p.GHUser, got.GHUser)
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")
	require.NoError(t, os.WriteFile(configFile, []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	// Create two profiles
	p1 := &Profile{Name: "personal", Email: "personal@example.com"}
	p2 := &Profile{Name: "work", Email: "work@example.com"}

	_, err := Set(p1, SetOptions{Detached: true})
	require.NoError(t, err)
	_, err = Set(p2, SetOptions{Detached: true})
	require.NoError(t, err)

	names, err := List()
	require.NoError(t, err)
	assert.Len(t, names, 2)
	assert.Contains(t, names, "personal")
	assert.Contains(t, names, "work")
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	_, err := Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")
	require.NoError(t, os.WriteFile(configFile, []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	// Create a profile
	p := &Profile{Name: "toremove", Email: "remove@example.com"}
	_, err := Set(p, SetOptions{Detached: true})
	require.NoError(t, err)

	// Verify it exists
	_, err = Get("toremove")
	require.NoError(t, err)

	// Remove it
	err = Remove("toremove")
	require.NoError(t, err)

	// Verify it's gone
	_, err = Get("toremove")
	assert.Error(t, err)
}

func TestSetField(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")
	require.NoError(t, os.WriteFile(configFile, []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	// Create a profile
	p := &Profile{Name: "fieldtest", Email: "old@example.com"}
	_, err := Set(p, SetOptions{Detached: true})
	require.NoError(t, err)

	// Update a field
	_, err = SetField("fieldtest", "email", "new@example.com", SetOptions{Detached: true})
	require.NoError(t, err)

	// Verify the update
	got, err := Get("fieldtest")
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", got.Email)
}

func TestSetFieldInvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitconfig"), []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	_, err := SetField("test", "invalid", "value", SetOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key")
}

func TestDefaultConfigFile(t *testing.T) {
	t.Run("prefers ~/.gitconfig when exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		setEnv(t, "HOME", tmpDir)

		gitconfig := filepath.Join(tmpDir, ".gitconfig")
		require.NoError(t, os.WriteFile(gitconfig, []byte(""), 0o600))

		result := DefaultConfigFile()
		assert.Equal(t, gitconfig, result)
	})

	t.Run("uses XDG when ~/.gitconfig missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		setEnv(t, "HOME", tmpDir)
		setEnv(t, "XDG_CONFIG_HOME", "")

		result := DefaultConfigFile()
		expected := filepath.Join(tmpDir, ".config", "git", "config")
		assert.Equal(t, expected, result)
	})

	t.Run("respects XDG_CONFIG_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()
		xdgDir := filepath.Join(tmpDir, "custom-config")
		setEnv(t, "HOME", tmpDir)
		setEnv(t, "XDG_CONFIG_HOME", xdgDir)

		result := DefaultConfigFile()
		expected := filepath.Join(xdgDir, "git", "config")
		assert.Equal(t, expected, result)
	})
}

func TestValidateSSHKey(t *testing.T) {
	t.Run("valid key", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyFile := filepath.Join(tmpDir, "id_test")
		require.NoError(t, os.WriteFile(keyFile, []byte("ssh key content"), 0o600))

		err := ValidateSSHKey(keyFile)
		assert.NoError(t, err)
	})

	t.Run("missing key", func(t *testing.T) {
		err := ValidateSSHKey("/nonexistent/key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := ValidateSSHKey(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})

	t.Run("expands tilde", func(t *testing.T) {
		// This test verifies ~ expansion works, even if the path doesn't exist
		err := ValidateSSHKey("~/.ssh/nonexistent_test_key")
		assert.Error(t, err)
		// Error should mention the expanded path, not contain ~
		assert.NotContains(t, err.Error(), "~")
	})
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected string
	}{
		{"~/.ssh/id_rsa", home + "/.ssh/id_rsa"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"}, // Only ~/... is expanded
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ExpandPath(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetGHAuthStatus(t *testing.T) {
	t.Run("empty username", func(t *testing.T) {
		status := GetGHAuthStatus("")
		assert.False(t, status.Authenticated)
		assert.Equal(t, "GitHub user not set", status.Message)
	})

	// Note: We can't reliably test actual gh auth status in unit tests
	// as it depends on the user's actual gh configuration
}

func TestGetSourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")
	require.NoError(t, os.WriteFile(configFile, []byte(""), 0o600))

	setEnv(t, "HOME", tmpDir)

	// Create a profile
	p := &Profile{Name: "sourcetest", Email: "source@example.com"}
	_, err := Set(p, SetOptions{Detached: true})
	require.NoError(t, err)

	// Get source file
	source, err := GetSourceFile("sourcetest")
	require.NoError(t, err)
	assert.Equal(t, configFile, source)
}

// Integration test using testutil.TestRepo
func TestProfileWithTestRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	// Set a local identity config in the repo
	repo.Git("config", "identity.local.email", "local@example.com")
	repo.Git("config", "identity.local.sshkey", "~/.ssh/local")

	// Note: This tests local config, not global. The identity package
	// primarily works with global config, but this demonstrates the
	// git config mechanics work correctly.
}
