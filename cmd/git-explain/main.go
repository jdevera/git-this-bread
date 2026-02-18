package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/analyzer"
	"github.com/jdevera/git-this-bread/internal/llmadvice"
	"github.com/jdevera/git-this-bread/internal/render"
)

var (
	verbose         bool
	compact         bool
	showAll         bool
	useTable        bool
	showLegend      bool
	quiet           bool
	showAdvice      bool
	useJSON         bool
	llmAdvice       bool
	llmProvider     string
	llmInstructions string
	noCache         bool
	perRepo         bool
)

var rootCmd = &cobra.Command{
	Use:   "git-explain [directory]",
	Short: "Check contribution status in git repositories",
	Long: `git-explain (a 🍞 git-this-bread tool)

Check your contribution status across git repositories.

If DIRECTORY is a git repo, analyze it directly.
Otherwise, analyze all immediate subdirectories.

LLM-POWERED ADVICE

Enable intelligent, context-aware suggestions with --llm-advice.
Requires an API key set in the environment:

  OpenAI (default):
    export OPENAI_API_KEY=sk-...
    git explain --llm-advice --advice

  Anthropic:
    export ANTHROPIC_API_KEY=sk-ant-...
    git explain --llm-advice --llm-provider anthropic --advice

Advice is cached based on repo state. Use --no-cache to bypass.
If the API is unavailable, falls back to rule-based advice.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExplain,
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output (default for single repo)")
	rootCmd.Flags().BoolVarP(&compact, "compact", "c", false, "Show compact one-line output (default for multi-repo)")
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all directories, even non-git ones")
	rootCmd.Flags().BoolVarP(&useTable, "table", "t", false, "Show compact table view")
	rootCmd.Flags().BoolVarP(&showLegend, "legend", "l", false, "Show legend explaining icons and colors")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress bar")
	rootCmd.Flags().BoolVar(&showAdvice, "advice", false, "Show actionable advice for each repo")
	rootCmd.Flags().BoolVar(&useJSON, "json", false, "Output as JSON")
	rootCmd.Flags().BoolVar(&llmAdvice, "llm-advice", false, "Enable LLM-powered advice (requires API key in env)")
	rootCmd.Flags().StringVar(&llmProvider, "llm-provider", "openai", "LLM provider: openai, anthropic")
	rootCmd.Flags().StringVar(&llmInstructions, "llm-instructions", "", "Custom instructions for the LLM (e.g., persona or style)")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass LLM advice cache")
	rootCmd.Flags().BoolVar(&perRepo, "per-repo", false, "In multi-repo mode, analyze each repo individually with LLM")
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "compact")
}

func runExplain(cmd *cobra.Command, args []string) error {
	if showLegend {
		render.PrintLegend()
		return nil
	}

	// Load and validate git config before doing anything
	if err := analyzer.LoadGitConfig(); err != nil {
		return err
	}

	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	target, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", target)
	}

	isSingleRepo := analyzer.IsGitRepo(target)

	// Determine verbose mode:
	// - Single repo: verbose by default, unless --compact
	// - Multi-repo: compact by default, unless --verbose
	useVerbose := verbose || (isSingleRepo && !compact)

	opts := analyzer.Options{
		Verbose: useVerbose || useJSON,
	}

	// Build LLM options if enabled
	var llmOpts *llmadvice.Options
	if llmAdvice {
		llmOpts = &llmadvice.Options{
			Provider:     llmadvice.ProviderType(llmProvider),
			NoCache:      noCache,
			PerRepo:      perRepo,
			Instructions: llmInstructions,
		}
		// --llm-advice implies --advice
		showAdvice = true
	}

	if isSingleRepo {
		// Single repo mode
		repoInfo := analyzer.AnalyzeRepo(target, opts)
		render.RenderRepo(&repoInfo, render.Options{
			Verbose:    useVerbose,
			ShowAdvice: showAdvice,
			UseJSON:    useJSON,
			LLMOpts:    llmOpts,
		})
	} else {
		// Multi-repo mode
		repos := analyzer.AnalyzeDirectory(target, opts, !quiet)

		switch {
		case useJSON:
			render.RenderJSON(repos)
		case useTable:
			render.RenderTable(repos)
		default:
			render.RenderRepos(repos, render.Options{
				Verbose:    useVerbose,
				ShowAdvice: showAdvice,
				ShowAll:    showAll,
				LLMOpts:    llmOpts,
			})
		}
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
