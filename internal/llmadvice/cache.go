package llmadvice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jdevera/git-this-bread/internal/analyzer"
)

// CacheEntry represents a cached LLM advice response
type CacheEntry struct {
	StateHash string    `json:"state_hash"`
	CreatedAt time.Time `json:"created_at"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Advice    []string  `json:"advice"`
}

// CacheKey represents the fields used to compute the cache hash
type CacheKey struct {
	Path          string
	CurrentBranch string
	Ahead         int
	Behind        int
	StagedFiles   int
	UnstagedFiles int
	Untracked     int
	StashCount    int
	IsFork        bool
	TotalCommits  int
	Instructions  string // Custom LLM instructions affect output
}

// getCacheDir returns the XDG-compliant cache directory
func getCacheDir() (string, error) {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheHome, "git-this-bread", "git-explain", "llm-advice"), nil
}

// computeStateHash computes a hash of the repo state that affects advice
func computeStateHash(info *analyzer.RepoInfo, instructions string) string {
	key := CacheKey{
		Path:          info.Path,
		CurrentBranch: info.CurrentBranch,
		Ahead:         info.Ahead,
		Behind:        info.Behind,
		StashCount:    info.StashCount,
		IsFork:        info.IsFork,
		TotalCommits:  info.TotalUserCommits,
		Instructions:  instructions,
	}

	if info.DirtyDetails != nil {
		key.StagedFiles = info.DirtyDetails.StagedFiles
		key.UnstagedFiles = info.DirtyDetails.UnstagedFiles
		key.Untracked = info.DirtyDetails.Untracked
	}

	data, _ := json.Marshal(key)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// computeMultiRepoStateHash computes a hash for multiple repos
func computeMultiRepoStateHash(repos []*analyzer.RepoInfo, instructions string) string {
	var hashes []string
	for _, repo := range repos {
		hashes = append(hashes, computeStateHash(repo, instructions))
	}
	data, _ := json.Marshal(hashes)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// getCacheFilePath returns the cache file path for a given hash
func getCacheFilePath(stateHash string) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, stateHash+".json"), nil
}

// ReadCache attempts to read cached advice for the given repo state
func ReadCache(info *analyzer.RepoInfo, instructions string) (*CacheEntry, error) {
	stateHash := computeStateHash(info, instructions)
	return readCacheByHash(stateHash)
}

// ReadMultiCache attempts to read cached advice for multiple repos
func ReadMultiCache(repos []*analyzer.RepoInfo, instructions string) (*CacheEntry, error) {
	stateHash := computeMultiRepoStateHash(repos, instructions)
	return readCacheByHash(stateHash)
}

func readCacheByHash(stateHash string) (*CacheEntry, error) {
	cachePath, err := getCacheFilePath(stateHash)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cachePath) //nolint:gosec // cachePath is constructed from hash, not user input
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// Verify hash matches (state-based validation)
	if entry.StateHash != stateHash {
		return nil, fmt.Errorf("cache hash mismatch")
	}

	return &entry, nil
}

// WriteCache writes advice to the cache
func WriteCache(info *analyzer.RepoInfo, instructions, provider, model string, advice []string) error {
	stateHash := computeStateHash(info, instructions)
	return writeCacheByHash(stateHash, provider, model, advice)
}

// WriteMultiCache writes advice for multiple repos to the cache
func WriteMultiCache(repos []*analyzer.RepoInfo, instructions, provider, model string, advice []string) error {
	stateHash := computeMultiRepoStateHash(repos, instructions)
	return writeCacheByHash(stateHash, provider, model, advice)
}

func writeCacheByHash(stateHash, provider, model string, advice []string) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return err
	}

	entry := CacheEntry{
		StateHash: stateHash,
		CreatedAt: time.Now(),
		Provider:  provider,
		Model:     model,
		Advice:    advice,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	cachePath, err := getCacheFilePath(stateHash)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0o600)
}
