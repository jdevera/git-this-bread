package llmadvice

import (
	"context"
	"time"

	"github.com/jdevera/git-this-bread/internal/analyzer"
)

// Options configures the LLM advice behavior
type Options struct {
	Provider     ProviderType
	NoCache      bool
	PerRepo      bool   // For multi-repo: analyze each repo individually
	Instructions string // Custom user instructions for the LLM
}

// DefaultOptions returns the default options
func DefaultOptions() Options {
	return Options{
		Provider: ProviderOpenAI,
		NoCache:  false,
		PerRepo:  false,
	}
}

// GetLLMAdvice returns LLM-powered advice for a single repo
// basicAdvice is the rule-based advice that the LLM can improve upon
// Falls back to nil (no advice) on error
func GetLLMAdvice(info *analyzer.RepoInfo, basicAdvice []string, opts Options) ([]string, error) {
	// Check cache first
	if !opts.NoCache {
		if cached, err := ReadCache(info, opts.Instructions); err == nil {
			return cached.Advice, nil
		}
	}

	// Create provider
	provider, err := NewProvider(opts.Provider)
	if err != nil {
		return nil, err
	}

	// Generate prompt and call LLM
	prompt := FormatSingleRepoPrompt(info, basicAdvice, opts.Instructions)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	advice, err := provider.GenerateAdvice(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if !opts.NoCache {
		_ = WriteCache(info, opts.Instructions, provider.Name(), provider.Model(), advice)
	}

	return advice, nil
}

// BasicAdviceFunc is a function that returns basic advice for a repo
type BasicAdviceFunc func(*analyzer.RepoInfo) []string

// GetMultiRepoLLMAdvice returns LLM-powered advice for multiple repos
// In default mode, sends all repos together for combined analysis
// With PerRepo=true, analyzes each repo individually
func GetMultiRepoLLMAdvice(repos []*analyzer.RepoInfo, getBasicAdvice BasicAdviceFunc, opts Options) (summary []string, perRepo map[string][]string, err error) {
	// Build basic advice map
	basicAdvicePerRepo := make(map[string][]string)
	for _, repo := range repos {
		basicAdvicePerRepo[repo.Name] = getBasicAdvice(repo)
	}

	if opts.PerRepo {
		// Per-repo mode: analyze each individually
		perRepoAdvice := make(map[string][]string)
		for _, repo := range repos {
			advice, err := GetLLMAdvice(repo, basicAdvicePerRepo[repo.Name], opts)
			if err != nil {
				// Continue on error, just skip this repo
				continue
			}
			perRepoAdvice[repo.Name] = advice
		}
		return nil, perRepoAdvice, nil
	}

	// Combined mode: send all repos together
	if !opts.NoCache {
		if cached, err := ReadMultiCache(repos, opts.Instructions); err == nil {
			return cached.Advice, nil, nil
		}
	}

	provider, err := NewProvider(opts.Provider)
	if err != nil {
		return nil, nil, err
	}

	prompt := FormatMultiRepoPrompt(repos, basicAdvicePerRepo, opts.Instructions)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	advice, err := provider.GenerateAdvice(ctx, prompt)
	if err != nil {
		return nil, nil, err
	}

	if !opts.NoCache {
		_ = WriteMultiCache(repos, opts.Instructions, provider.Name(), provider.Model(), advice)
	}

	return advice, nil, nil
}
