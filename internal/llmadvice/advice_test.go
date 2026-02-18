package llmadvice

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jdevera/git-this-bread/internal/analyzer"
)

// mockProvider implements Provider for testing
type mockProvider struct {
	name   string
	model  string
	advice []string
	err    error
	called bool
	prompt string
}

// Ensure mockProvider implements Provider interface.
var _ Provider = (*mockProvider)(nil)

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Model() string {
	return m.model
}

func (m *mockProvider) GenerateAdvice(ctx context.Context, prompt string) ([]string, error) {
	m.called = true
	m.prompt = prompt
	if m.err != nil {
		return nil, m.err
	}
	return m.advice, nil
}

func TestComputeStateHash(t *testing.T) {
	info1 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "main",
		Ahead:         2,
		StashCount:    1,
	}

	info2 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "main",
		Ahead:         2,
		StashCount:    1,
	}

	info3 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "develop", // Different branch
		Ahead:         2,
		StashCount:    1,
	}

	// Same state should produce same hash
	hash1 := computeStateHash(info1, "")
	hash2 := computeStateHash(info2, "")
	assert.Equal(t, hash1, hash2, "Same state should produce same hash")

	// Different state should produce different hash
	hash3 := computeStateHash(info3, "")
	assert.NotEqual(t, hash1, hash3, "Different state should produce different hash")

	// Hash should be deterministic
	hash1Again := computeStateHash(info1, "")
	assert.Equal(t, hash1, hash1Again, "Hash should be deterministic")

	// Different instructions should produce different hash
	hash1WithInstructions := computeStateHash(info1, "be Eeyore")
	assert.NotEqual(t, hash1, hash1WithInstructions, "Different instructions should produce different hash")
}

func TestComputeStateHashWithDirtyDetails(t *testing.T) {
	info1 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "main",
		DirtyDetails: &analyzer.DirtyDetails{
			StagedFiles:   2,
			UnstagedFiles: 3,
			Untracked:     5,
		},
	}

	info2 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "main",
		DirtyDetails: &analyzer.DirtyDetails{
			StagedFiles:   2,
			UnstagedFiles: 3,
			Untracked:     5,
		},
	}

	info3 := &analyzer.RepoInfo{
		Path:          "/path/to/repo",
		CurrentBranch: "main",
		DirtyDetails: &analyzer.DirtyDetails{
			StagedFiles:   1, // Different
			UnstagedFiles: 3,
			Untracked:     5,
		},
	}

	hash1 := computeStateHash(info1, "")
	hash2 := computeStateHash(info2, "")
	hash3 := computeStateHash(info3, "")

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
}

func TestCacheReadWrite(t *testing.T) {
	// Use a temp directory for cache
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	info := &analyzer.RepoInfo{
		Path:          "/test/repo",
		CurrentBranch: "main",
		Ahead:         1,
	}

	advice := []string{"Push your changes", "Review stashes"}
	instructions := ""

	// Write to cache
	err := WriteCache(info, instructions, "openai", "gpt-4o-mini", advice)
	require.NoError(t, err)

	// Read from cache
	entry, err := ReadCache(info, instructions)
	require.NoError(t, err)
	assert.Equal(t, "openai", entry.Provider)
	assert.Equal(t, "gpt-4o-mini", entry.Model)
	assert.Equal(t, advice, entry.Advice)

	// Change repo state - should not find cache
	info.Ahead = 2
	_, err = ReadCache(info, instructions)
	assert.Error(t, err)

	// Different instructions should not find cache
	info.Ahead = 1 // Reset
	_, err = ReadCache(info, "be Eeyore")
	assert.Error(t, err)
}

func TestCacheDir(t *testing.T) {
	// Test with XDG_CACHE_HOME set
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	dir, err := getCacheDir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/cache/git-this-bread/llm-advice", dir)

	// Test with XDG_CACHE_HOME unset (falls back to ~/.cache)
	t.Setenv("XDG_CACHE_HOME", "")
	dir, err = getCacheDir()
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".cache", "git-this-bread", "llm-advice")
	assert.Equal(t, expected, dir)
}

func TestParseAdviceResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name: "numbered list",
			response: `1. Push your 4 unpushed commits
2. Review your 2 stashes
3. Commit staged changes`,
			expected: []string{
				"Push your 4 unpushed commits",
				"Review your 2 stashes",
				"Commit staged changes",
			},
		},
		{
			name: "bulleted list",
			response: `- Push your changes
- Review stashes
- Clean up untracked files`,
			expected: []string{
				"Push your changes",
				"Review stashes",
				"Clean up untracked files",
			},
		},
		{
			name: "mixed format with empty lines",
			response: `1. Push commits

2) Review stashes

* Clean up`,
			expected: []string{
				"Push commits",
				"Review stashes",
				"Clean up",
			},
		},
		{
			name:     "plain text",
			response: "Everything looks good!",
			expected: []string{"Everything looks good!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAdviceResponse(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSingleRepoPrompt(t *testing.T) {
	info := &analyzer.RepoInfo{
		Name:                  "my-project",
		Path:                  "/home/user/my-project",
		CurrentBranch:         "feature-branch",
		DefaultBranch:         "main",
		Ahead:                 3,
		TotalUserCommits:      42,
		LastCommitDate:        "2025-02-01",
		StashCount:            2,
		HasUncommittedChanges: true,
		DirtyDetails: &analyzer.DirtyDetails{
			StagedFiles:      1,
			StagedInsertions: 10,
			StagedDeletions:  5,
			UnstagedFiles:    2,
			Untracked:        3,
		},
	}

	basicAdvice := []string{"Push your commits", "Review stashes"}
	prompt := FormatSingleRepoPrompt(info, basicAdvice, "")

	// Check that key information is included
	assert.Contains(t, prompt, "my-project")
	assert.Contains(t, prompt, "feature-branch")
	assert.Contains(t, prompt, "(feature branch)") // Shows it's not the default branch
	assert.Contains(t, prompt, "Unpushed Commits: 3")
	assert.Contains(t, prompt, "Stashes (2):")
	assert.Contains(t, prompt, "Staged (1 files")
	assert.Contains(t, prompt, "Untracked (3 files)")
	// Check basic advice is included
	assert.Contains(t, prompt, "Basic Advice")
	assert.Contains(t, prompt, "Push your commits")
}

func TestFormatMultiRepoPrompt(t *testing.T) {
	repos := []*analyzer.RepoInfo{
		{
			Name:          "repo1",
			Path:          "/path/repo1",
			CurrentBranch: "main",
			Ahead:         2,
		},
		{
			Name:          "repo2",
			Path:          "/path/repo2",
			CurrentBranch: "develop",
			StashCount:    1,
			Stashes: []analyzer.StashInfo{
				{Index: 0, Message: "WIP", Date: "2 days ago"},
			},
		},
	}

	basicAdvice := map[string][]string{
		"repo1": {"Push your commits"},
		"repo2": {"Review stashes"},
	}
	prompt := FormatMultiRepoPrompt(repos, basicAdvice, "")

	assert.Contains(t, prompt, "Multiple Repository States")
	assert.Contains(t, prompt, "Repository 1: repo1")
	assert.Contains(t, prompt, "Repository 2: repo2")
	assert.Contains(t, prompt, "Unpushed Commits: 2")
	assert.Contains(t, prompt, "Stashes (1):")
	assert.Contains(t, prompt, "WIP")
	assert.Contains(t, prompt, "Push your commits")
	assert.Contains(t, prompt, "Review stashes")
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, ProviderOpenAI, opts.Provider)
	assert.False(t, opts.NoCache)
	assert.False(t, opts.PerRepo)
}

func TestProviderType(t *testing.T) {
	assert.Equal(t, ProviderType("openai"), ProviderOpenAI)
	assert.Equal(t, ProviderType("anthropic"), ProviderAnthropic)
}
