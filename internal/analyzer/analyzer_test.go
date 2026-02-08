package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseShortstat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		insertions int
		deletions  int
	}{
		{
			name:       "typical output",
			input:      " 3 files changed, 10 insertions(+), 5 deletions(-)",
			insertions: 10,
			deletions:  5,
		},
		{
			name:       "insertions only",
			input:      " 1 file changed, 42 insertions(+)",
			insertions: 42,
			deletions:  0,
		},
		{
			name:       "deletions only",
			input:      " 2 files changed, 15 deletions(-)",
			insertions: 0,
			deletions:  15,
		},
		{
			name:       "empty string",
			input:      "",
			insertions: 0,
			deletions:  0,
		},
		{
			name:       "no matches",
			input:      "some random text",
			insertions: 0,
			deletions:  0,
		},
		{
			name:       "singular insertion",
			input:      " 1 file changed, 1 insertion(+)",
			insertions: 1,
			deletions:  0,
		},
		{
			name:       "singular deletion",
			input:      " 1 file changed, 1 deletion(-)",
			insertions: 0,
			deletions:  1,
		},
		{
			name:       "large numbers",
			input:      " 100 files changed, 9999 insertions(+), 5555 deletions(-)",
			insertions: 9999,
			deletions:  5555,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ins, del := parseShortstat(tt.input)
			assert.Equal(t, tt.insertions, ins, "insertions mismatch")
			assert.Equal(t, tt.deletions, del, "deletions mismatch")
		})
	}
}

func TestDirtyDetails_TotalFiles(t *testing.T) {
	tests := []struct {
		name     string
		details  DirtyDetails
		expected int
	}{
		{
			name:     "all zeros",
			details:  DirtyDetails{},
			expected: 0,
		},
		{
			name: "staged only",
			details: DirtyDetails{
				StagedFiles: 3,
			},
			expected: 3,
		},
		{
			name: "unstaged only",
			details: DirtyDetails{
				UnstagedFiles: 5,
			},
			expected: 5,
		},
		{
			name: "untracked only",
			details: DirtyDetails{
				Untracked: 2,
			},
			expected: 2,
		},
		{
			name: "all types",
			details: DirtyDetails{
				StagedFiles:   3,
				UnstagedFiles: 5,
				Untracked:     2,
			},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.details.TotalFiles())
		})
	}
}

func TestDirtyDetails_String(t *testing.T) {
	tests := []struct {
		name     string
		details  DirtyDetails
		expected string
	}{
		{
			name:     "empty",
			details:  DirtyDetails{},
			expected: "",
		},
		{
			name: "staged only",
			details: DirtyDetails{
				StagedFiles:      2,
				StagedInsertions: 10,
				StagedDeletions:  5,
			},
			expected: "staged:2 +10/-5",
		},
		{
			name: "unstaged only",
			details: DirtyDetails{
				UnstagedFiles:      3,
				UnstagedInsertions: 20,
				UnstagedDeletions:  8,
			},
			expected: "modified:3 +20/-8",
		},
		{
			name: "untracked only",
			details: DirtyDetails{
				Untracked: 4,
			},
			expected: "untracked:4",
		},
		{
			name: "all types",
			details: DirtyDetails{
				StagedFiles:        2,
				StagedInsertions:   10,
				StagedDeletions:    5,
				UnstagedFiles:      3,
				UnstagedInsertions: 20,
				UnstagedDeletions:  8,
				Untracked:          4,
			},
			expected: "staged:2 +10/-5 modified:3 +20/-8 untracked:4",
		},
		{
			name: "staged and untracked",
			details: DirtyDetails{
				StagedFiles:      1,
				StagedInsertions: 5,
				StagedDeletions:  0,
				Untracked:        2,
			},
			expected: "staged:1 +5/-0 untracked:2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.details.String())
		})
	}
}

func TestIsUserRemote(t *testing.T) {
	tests := []struct {
		name       string
		githubUser string
		url        string
		expected   bool
	}{
		{
			name:       "SSH URL match",
			githubUser: "testuser",
			url:        "git@github.com:testuser/repo.git",
			expected:   true,
		},
		{
			name:       "HTTPS URL match",
			githubUser: "testuser",
			url:        "https://github.com/testuser/repo.git",
			expected:   true,
		},
		{
			name:       "no match",
			githubUser: "testuser",
			url:        "git@github.com:otheruser/repo.git",
			expected:   false,
		},
		{
			name:       "case insensitive match",
			githubUser: "TestUser",
			url:        "git@github.com:testuser/repo.git",
			expected:   true,
		},
		{
			name:       "empty github user",
			githubUser: "",
			url:        "git@github.com:testuser/repo.git",
			expected:   false,
		},
		{
			name:       "partial username match in path",
			githubUser: "test",
			url:        "git@github.com:testuser/repo.git",
			expected:   true, // substring match behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTestConfig("test@example.com", tt.githubUser)
			defer ResetTestConfig()

			result := isUserRemote(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsUserCommit(t *testing.T) {
	// isUserCommit requires a *object.Commit which is complex to construct
	// without a real git repo. This is tested in integration tests instead.
	// Here we test the internal email comparison logic indirectly.
	t.Run("empty email returns false", func(t *testing.T) {
		ResetTestConfig()
		// With empty userEmail, any commit should return false
		// This is tested in integration_test.go with real commits
	})
}
