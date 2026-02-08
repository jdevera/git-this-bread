package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdevera/git-this-bread/internal/analyzer"
	"github.com/jdevera/git-this-bread/testutil"
)

func TestGetAdvice(t *testing.T) {
	tests := []struct {
		name     string
		info     *analyzer.RepoInfo
		expected []string
	}{
		{
			name: "no contributions no changes",
			info: &analyzer.RepoInfo{
				IsGitRepo:     true,
				HasUserRemote: false,
			},
			expected: []string{"No contributions - consider removing if not needed"},
		},
		{
			name: "no contributions with uncommitted changes",
			info: &analyzer.RepoInfo{
				IsGitRepo:             true,
				HasUserRemote:         false,
				HasUncommittedChanges: true,
			},
			expected: []string{"Has local changes but no remote - set up your fork or commit upstream"},
		},
		{
			name: "no contributions with stash",
			info: &analyzer.RepoInfo{
				IsGitRepo:     true,
				HasUserRemote: false,
				StashCount:    1,
			},
			expected: []string{
				"Has local changes but no remote - set up your fork or commit upstream",
				"Review 1 stash(es) - apply or drop",
			},
		},
		{
			name: "forked but no commits",
			info: &analyzer.RepoInfo{
				IsGitRepo:        true,
				HasUserRemote:    true,
				TotalUserCommits: 0,
			},
			expected: []string{"Forked but no commits yet - start contributing or remove"},
		},
		{
			name: "has unpushed commits",
			info: &analyzer.RepoInfo{
				IsGitRepo:        true,
				HasUserRemote:    true,
				TotalUserCommits: 5,
				Ahead:            3,
			},
			expected: []string{"Push your 3 unpushed commit(s)"},
		},
		{
			name: "staged changes ready",
			info: &analyzer.RepoInfo{
				IsGitRepo:             true,
				HasUserRemote:         true,
				TotalUserCommits:      1,
				HasUncommittedChanges: true,
				DirtyDetails: &analyzer.DirtyDetails{
					StagedFiles: 2,
				},
			},
			expected: []string{"Staged changes ready - commit 2 file(s)"},
		},
		{
			name: "many untracked files",
			info: &analyzer.RepoInfo{
				IsGitRepo:             true,
				HasUserRemote:         true,
				TotalUserCommits:      1,
				HasUncommittedChanges: true,
				DirtyDetails: &analyzer.DirtyDetails{
					Untracked: 10,
				},
			},
			expected: []string{"10 untracked files - add to .gitignore or stage"},
		},
		{
			name: "has stashes",
			info: &analyzer.RepoInfo{
				IsGitRepo:        true,
				HasUserRemote:    true,
				TotalUserCommits: 1,
				StashCount:       3,
			},
			expected: []string{"Review 3 stash(es) - apply or drop"},
		},
		{
			name: "healthy repo no advice",
			info: &analyzer.RepoInfo{
				IsGitRepo:        true,
				HasUserRemote:    true,
				TotalUserCommits: 10,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			advice := GetAdvice(tt.info)

			if tt.expected == nil {
				assert.Empty(t, advice)
			} else {
				assert.Equal(t, len(tt.expected), len(advice), "advice count mismatch")
				for _, exp := range tt.expected {
					assert.Contains(t, advice, exp)
				}
			}
		})
	}
}

func TestToMap(t *testing.T) {
	t.Run("non-git repo", func(t *testing.T) {
		info := &analyzer.RepoInfo{
			Name:      "test-repo",
			Path:      "/path/to/repo",
			IsGitRepo: false,
		}

		m := toMap(info)

		assert.Equal(t, "test-repo", m["name"])
		assert.Equal(t, "/path/to/repo", m["path"])
		assert.Equal(t, false, m["is_git_repo"])
		// Should not have git-specific fields
		_, hasCommits := m["commits"]
		assert.False(t, hasCommits)
	})

	t.Run("repo with error", func(t *testing.T) {
		info := &analyzer.RepoInfo{
			Name:      "test-repo",
			Path:      "/path/to/repo",
			IsGitRepo: true,
			Error:     "some error",
		}

		m := toMap(info)

		assert.Equal(t, "some error", m["error"])
	})

	t.Run("full git repo", func(t *testing.T) {
		info := &analyzer.RepoInfo{
			Name:               "test-repo",
			Path:               "/path/to/repo",
			IsGitRepo:          true,
			CurrentBranch:      "main",
			DefaultBranch:      "main",
			IsFork:             true,
			TotalUserCommits:   42,
			LastCommitDate:     "2024-01-15",
			LastRepoCommitDate: "2024-01-20",
			Ahead:              3,
			StashCount:         1,
			DirtyDetails: &analyzer.DirtyDetails{
				StagedFiles:   2,
				UnstagedFiles: 3,
				Untracked:     1,
			},
			AllRemotes: []analyzer.RemoteInfo{
				{Name: "origin", URL: "git@github.com:user/repo.git", IsMine: true},
				{Name: "upstream", URL: "git@github.com:original/repo.git", IsMine: false},
			},
		}

		m := toMap(info)

		assert.Equal(t, "main", m["current_branch"])
		assert.Equal(t, "main", m["default_branch"])
		assert.Equal(t, true, m["is_fork"])
		assert.Equal(t, 3, m["ahead"])
		assert.Equal(t, 1, m["stash_count"])

		commits := m["commits"].(map[string]interface{})
		assert.Equal(t, 42, commits["user_total"])
		assert.Equal(t, "2024-01-15", commits["last_user_commit"])
		assert.Equal(t, "2024-01-20", commits["last_repo_commit"])

		dirty := m["dirty"].(map[string]interface{})
		assert.Equal(t, 2, dirty["staged"])
		assert.Equal(t, 3, dirty["unstaged"])
		assert.Equal(t, 1, dirty["untracked"])

		remotes := m["remotes"].([]map[string]interface{})
		assert.Len(t, remotes, 2)
		assert.Equal(t, "origin", remotes[0]["name"])
		assert.Equal(t, true, remotes[0]["is_mine"])
	})

	t.Run("no dirty details when clean", func(t *testing.T) {
		info := &analyzer.RepoInfo{
			Name:          "test-repo",
			Path:          "/path/to/repo",
			IsGitRepo:     true,
			CurrentBranch: "main",
			DirtyDetails:  nil,
		}

		m := toMap(info)

		_, hasDirty := m["dirty"]
		assert.False(t, hasDirty)
	})
}

func TestRenderJSON(t *testing.T) {
	repos := []analyzer.RepoInfo{
		{
			Name:             "repo1",
			Path:             "/path/to/repo1",
			IsGitRepo:        true,
			CurrentBranch:    "main",
			TotalUserCommits: 10,
		},
		{
			Name:      "repo2",
			Path:      "/path/to/repo2",
			IsGitRepo: false,
		},
	}

	output := testutil.CaptureStdout(func() {
		RenderJSON(repos)
	})

	// Verify it's valid JSON
	var parsed []map[string]interface{}
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)

	assert.Len(t, parsed, 2)
	assert.Equal(t, "repo1", parsed[0]["name"])
	assert.Equal(t, true, parsed[0]["is_git_repo"])
	assert.Equal(t, "repo2", parsed[1]["name"])
	assert.Equal(t, false, parsed[1]["is_git_repo"])
}

func TestRenderRepo_JSON(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:             "test-repo",
		Path:             "/path/to/test-repo",
		IsGitRepo:        true,
		CurrentBranch:    "feature",
		TotalUserCommits: 5,
	}

	output := testutil.CaptureStdout(func() {
		RenderRepo(info, Options{UseJSON: true})
	})

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "test-repo", parsed["name"])
	assert.Equal(t, "feature", parsed["current_branch"])
}

func TestRenderRepo_Compact(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:             "test-repo",
		Path:             "/path/to/test-repo",
		IsGitRepo:        true,
		CurrentBranch:    "main",
		HasUserRemote:    true,
		UserRemotes:      []string{"origin"},
		TotalUserCommits: 5,
	}

	output := testutil.CaptureStdout(func() {
		RenderRepo(info, Options{Verbose: false})
	})

	// Should be a single line containing repo info
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 1)
	assert.Contains(t, output, "test-repo")
	assert.Contains(t, output, "main")
}

func TestRenderRepo_NotGitRepo(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:      "not-a-repo",
		Path:      "/path/to/not-a-repo",
		IsGitRepo: false,
	}

	output := testutil.CaptureStdout(func() {
		RenderRepo(info, Options{})
	})

	assert.Contains(t, output, "not-a-repo")
	assert.Contains(t, output, "not a git repo")
}

func TestRenderRepo_WithError(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:      "error-repo",
		Path:      "/path/to/error-repo",
		IsGitRepo: true,
		Error:     "failed to read repo",
	}

	output := testutil.CaptureStdout(func() {
		RenderRepo(info, Options{})
	})

	assert.Contains(t, output, "error-repo")
	assert.Contains(t, output, "failed to read repo")
}

func TestRenderRepo_WithAdvice(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:             "needs-advice",
		Path:             "/path/to/needs-advice",
		IsGitRepo:        true,
		CurrentBranch:    "main",
		HasUserRemote:    true,
		TotalUserCommits: 1,
		Ahead:            2,
	}

	output := testutil.CaptureStdout(func() {
		RenderRepo(info, Options{ShowAdvice: true})
	})

	assert.Contains(t, output, "Push your 2 unpushed commit(s)")
}
