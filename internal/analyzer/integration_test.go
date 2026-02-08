//go:build integration

package analyzer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdevera/git-this-bread/testutil"
)

func TestAnalyzeRepo_EmptyRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.IsGitRepo)
	assert.Equal(t, 0, info.TotalUserCommits)
	assert.False(t, info.HasUncommittedChanges)
	assert.Equal(t, 0, info.StashCount)
}

func TestAnalyzeRepo_WithUserCommits(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Create commits
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("First commit")

	repo.WriteFile("file2.txt", "content2")
	repo.Commit("Second commit")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.IsGitRepo)
	assert.Equal(t, 2, info.TotalUserCommits)
	assert.NotEmpty(t, info.LastCommitDate)
}

func TestAnalyzeRepo_WithMixedCommits(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// User commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("User commit")

	// Other user's commit
	repo.WriteFile("file2.txt", "content2")
	repo.CommitAs("Other commit", "other@example.com", "Other User")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.Equal(t, 1, info.TotalUserCommits)
}

func TestAnalyzeRepo_DirtyWorkingDirectory(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Initial commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("Initial commit")

	// Create dirty state
	repo.WriteFile("file1.txt", "modified content")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.HasUncommittedChanges)
	require.NotNil(t, info.DirtyDetails)
	assert.Equal(t, 1, info.DirtyDetails.UnstagedFiles)
}

func TestAnalyzeRepo_StagedChanges(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Initial commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("Initial commit")

	// Staged change
	repo.WriteFile("file2.txt", "new file")
	repo.Stage("file2.txt")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.HasUncommittedChanges)
	require.NotNil(t, info.DirtyDetails)
	assert.Equal(t, 1, info.DirtyDetails.StagedFiles)
	assert.Equal(t, 0, info.DirtyDetails.UnstagedFiles)
}

func TestAnalyzeRepo_UntrackedFiles(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Initial commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("Initial commit")

	// Untracked file
	repo.WriteFile("untracked.txt", "untracked content")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.HasUncommittedChanges)
	require.NotNil(t, info.DirtyDetails)
	assert.Equal(t, 1, info.DirtyDetails.Untracked)
}

func TestAnalyzeRepo_WithRemotes(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Add user's remote
	repo.AddRemote("origin", "git@github.com:testuser/repo.git")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.HasUserRemote)
	assert.Contains(t, info.UserRemotes, "origin")
	assert.Len(t, info.AllRemotes, 1)
	assert.True(t, info.AllRemotes[0].IsMine)
}

func TestAnalyzeRepo_Fork(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Fork setup: user remote + upstream
	repo.AddRemote("origin", "git@github.com:testuser/repo.git")
	repo.AddRemote("upstream", "git@github.com:original/repo.git")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.True(t, info.IsFork)
	assert.True(t, info.HasUserRemote)
	assert.NotEmpty(t, info.UpstreamURL)
}

func TestAnalyzeRepo_StashCount(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Initial commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("Initial commit")

	// Create stash
	repo.WriteFile("file1.txt", "modified")
	repo.Stash()

	info := AnalyzeRepo(repo.Path, Options{})

	assert.Equal(t, 1, info.StashCount)
}

func TestAnalyzeRepo_MultipleStashes(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Initial commit
	repo.WriteFile("file1.txt", "content1")
	repo.Commit("Initial commit")

	// Create multiple stashes
	repo.WriteFile("file1.txt", "modified1")
	repo.Stash()

	repo.WriteFile("file1.txt", "modified2")
	repo.Stash()

	info := AnalyzeRepo(repo.Path, Options{})

	assert.Equal(t, 2, info.StashCount)
}

func TestAnalyzeRepo_NotGitRepo(t *testing.T) {
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Create a plain directory without git init
	dir, err := os.MkdirTemp("", "non-git-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	info := AnalyzeRepo(dir, Options{})

	assert.False(t, info.IsGitRepo)
	assert.Equal(t, 0, info.TotalUserCommits)
}

func TestAnalyzeRepo_CurrentBranch(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	SetTestConfig("test@example.com", "testuser")
	defer ResetTestConfig()

	// Need a commit to create branches
	repo.WriteFile("file1.txt", "content")
	repo.Commit("Initial commit")

	// Create and checkout new branch
	repo.CreateBranch("feature")
	repo.Checkout("feature")

	info := AnalyzeRepo(repo.Path, Options{})

	assert.Equal(t, "feature", info.CurrentBranch)
}

func TestGetDirtyDetails(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(r *testutil.TestRepo)
		expected *DirtyDetails
	}{
		{
			name: "clean repo",
			setup: func(r *testutil.TestRepo) {
				r.WriteFile("file.txt", "content")
				r.Commit("Initial")
			},
			expected: nil,
		},
		{
			name: "modified file",
			setup: func(r *testutil.TestRepo) {
				r.WriteFile("file.txt", "content")
				r.Commit("Initial")
				r.WriteFile("file.txt", "modified")
			},
			expected: &DirtyDetails{
				UnstagedFiles: 1,
			},
		},
		{
			name: "staged file",
			setup: func(r *testutil.TestRepo) {
				r.WriteFile("file.txt", "content")
				r.Commit("Initial")
				r.WriteFile("new.txt", "new content")
				r.Stage("new.txt")
			},
			expected: &DirtyDetails{
				StagedFiles: 1,
			},
		},
		{
			name: "untracked file",
			setup: func(r *testutil.TestRepo) {
				r.WriteFile("file.txt", "content")
				r.Commit("Initial")
				r.WriteFile("untracked.txt", "untracked")
			},
			expected: &DirtyDetails{
				Untracked: 1,
			},
		},
		{
			name: "mixed state",
			setup: func(r *testutil.TestRepo) {
				r.WriteFile("file.txt", "content")
				r.Commit("Initial")
				r.WriteFile("file.txt", "modified")      // unstaged
				r.WriteFile("new.txt", "new")            // will stage
				r.Stage("new.txt")                       // staged
				r.WriteFile("untracked.txt", "untracked") // untracked
			},
			expected: &DirtyDetails{
				StagedFiles:   1,
				UnstagedFiles: 1,
				Untracked:     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewTestRepo(t)
			tt.setup(repo)

			dirty, details := getDirtyDetails(repo.Path)

			if tt.expected == nil {
				assert.False(t, dirty)
				assert.Nil(t, details)
			} else {
				assert.True(t, dirty)
				require.NotNil(t, details)
				assert.Equal(t, tt.expected.StagedFiles, details.StagedFiles, "StagedFiles")
				assert.Equal(t, tt.expected.UnstagedFiles, details.UnstagedFiles, "UnstagedFiles")
				assert.Equal(t, tt.expected.Untracked, details.Untracked, "Untracked")
			}
		})
	}
}

func TestIsUserCommit_Integration(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	// Commit as test user
	repo.WriteFile("file.txt", "content")
	repo.Commit("Test commit")

	t.Run("matches user email", func(t *testing.T) {
		SetTestConfig("test@example.com", "testuser")
		defer ResetTestConfig()

		info := AnalyzeRepo(repo.Path, Options{})
		assert.Equal(t, 1, info.TotalUserCommits)
	})

	t.Run("does not match different email", func(t *testing.T) {
		SetTestConfig("other@example.com", "testuser")
		defer ResetTestConfig()

		info := AnalyzeRepo(repo.Path, Options{})
		assert.Equal(t, 0, info.TotalUserCommits)
	})
}
