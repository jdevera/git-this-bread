// Package testutil provides test utilities for git-this-bread tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRepo is a helper for creating temporary git repositories in tests.
type TestRepo struct {
	t    testing.TB
	Path string
}

// NewTestRepo creates a new temporary git repository and registers cleanup.
func NewTestRepo(t testing.TB) *TestRepo {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	repo := &TestRepo{
		t:    t,
		Path: dir,
	}

	// Initialize git repo
	repo.Git("init")
	repo.Git("config", "user.email", "test@example.com")
	repo.Git("config", "user.name", "Test User")

	return repo
}

// Git runs a git command in the repository directory.
func (r *TestRepo) Git(args ...string) string {
	r.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

// GitMayFail runs a git command that may fail, returning output and error.
func (r *TestRepo) GitMayFail(args ...string) (string, error) {
	r.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// WriteFile creates a file with the given content in the repository.
func (r *TestRepo) WriteFile(name, content string) {
	r.t.Helper()

	path := filepath.Join(r.Path, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // test helper needs standard perms
		r.t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		r.t.Fatalf("failed to write file %s: %v", name, err)
	}
}

// Commit stages all changes and creates a commit with the given message.
func (r *TestRepo) Commit(message string) {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-m", message)
}

// CommitAs stages all changes and creates a commit as the specified author.
func (r *TestRepo) CommitAs(message, email, name string) {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-m", message, "--author", name+" <"+email+">")
}

// AddRemote adds a remote with the given name and URL.
func (r *TestRepo) AddRemote(name, url string) {
	r.t.Helper()
	r.Git("remote", "add", name, url)
}

// Stash creates a stash entry. There must be staged or unstaged changes.
func (r *TestRepo) Stash() {
	r.t.Helper()
	r.Git("stash", "push", "-m", "test stash")
}

// CreateBranch creates a new branch.
func (r *TestRepo) CreateBranch(name string) {
	r.t.Helper()
	r.Git("branch", name)
}

// Checkout switches to the given branch.
func (r *TestRepo) Checkout(name string) {
	r.t.Helper()
	r.Git("checkout", name)
}

// Stage stages a file.
func (r *TestRepo) Stage(name string) {
	r.t.Helper()
	r.Git("add", name)
}
