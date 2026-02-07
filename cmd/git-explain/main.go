package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/analyzer"
	"github.com/jdevera/git-this-bread/internal/render"
)

var (
	verbose    bool
	showAll    bool
	useTable   bool
	showLegend bool
	quiet      bool
	showAdvice bool
	useJSON    bool
)

var rootCmd = &cobra.Command{
	Use:   "git-explain [directory]",
	Short: "Check contribution status in git repositories",
	Long: `git-explain (a 🍞 git-this-bread tool)

Check your contribution status across git repositories.

If DIRECTORY is a git repo, analyze it directly.
Otherwise, analyze all immediate subdirectories.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExplain,
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed branch information")
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all directories, even non-git ones")
	rootCmd.Flags().BoolVarP(&useTable, "table", "t", false, "Show compact table view")
	rootCmd.Flags().BoolVarP(&showLegend, "legend", "l", false, "Show legend explaining icons and colors")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress bar")
	rootCmd.Flags().BoolVar(&showAdvice, "advice", false, "Show actionable advice for each repo")
	rootCmd.Flags().BoolVar(&useJSON, "json", false, "Output as JSON")
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

	opts := analyzer.Options{
		Verbose: verbose || useJSON,
	}

	if analyzer.IsGitRepo(target) {
		// Single repo mode
		repoInfo := analyzer.AnalyzeRepo(target, opts)
		render.RenderRepo(&repoInfo, render.Options{
			Verbose:    verbose,
			ShowAdvice: showAdvice,
			UseJSON:    useJSON,
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
			for i := range repos {
				repo := &repos[i]
				if showAll || repo.IsGitRepo {
					render.RenderRepo(repo, render.Options{
						Verbose:    verbose,
						ShowAdvice: showAdvice,
					})
				}
			}
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
