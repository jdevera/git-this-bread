package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/identity"
)

var (
	asProfile  string
	showAll    bool
	jsonOutput bool
)

type Fork struct {
	Name           string   `json:"name"`
	FullName       string   `json:"full_name"`
	URL            string   `json:"html_url"`
	ParentName     string   `json:"parent_name"`
	ParentFullName string   `json:"parent_full_name"`
	DefaultBranch  string   `json:"default_branch"`
	Ahead          int      `json:"ahead"`
	Behind         int      `json:"behind"`
	Branches       []Branch `json:"branches,omitempty"`
	OpenPRs        int      `json:"open_prs"`
	Untouched      bool     `json:"untouched"`
}

type Branch struct {
	Name      string `json:"name"`
	LastPush  string `json:"last_push"`
	IsDefault bool   `json:"is_default"`
}

var rootCmd = &cobra.Command{
	Use:   "gh-wtfork",
	Short: "What the fork? Analyze your GitHub forks",
	Long: `gh-wtfork (a git-this-bread tool)

Analyze all your GitHub forks to see:
- How much they've deviated from upstream (ahead/behind)
- Non-default branches and when they were last pushed
- Open pull requests
- Whether forks are "untouched" (can be safely deleted)

Use --as to run with a specific identity profile managed by git-id.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVar(&asProfile, "as", "", "Run as identity profile (managed by git-id)")
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all forks (default: hide untouched)")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Set up gh command runner
	ghCmd := &ghRunner{profile: asProfile}
	defer ghCmd.cleanup()

	// Verify gh is authenticated
	if err := ghCmd.checkAuth(); err != nil {
		return err
	}

	// Get all user's repos that are forks
	forks, err := ghCmd.listForks()
	if err != nil {
		return fmt.Errorf("failed to list forks: %w", err)
	}

	if len(forks) == 0 {
		fmt.Println("No forks found.")
		return nil
	}

	// Analyze each fork
	var results []Fork
	total := len(forks)
	for i := range forks {
		// Clear line and show progress
		fmt.Fprintf(os.Stderr, "\r\033[K[%d/%d] Analyzing: %s", i+1, total, forks[i].Name)

		analyzed, err := ghCmd.analyzeFork(&forks[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n  Warning: failed to analyze %s: %v\n", forks[i].FullName, err)
			continue
		}
		results = append(results, analyzed)
	}
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line

	// Filter untouched if not showing all
	if !showAll {
		var filtered []Fork
		for i := range results {
			if !results[i].Untouched {
				filtered = append(filtered, results[i])
			}
		}
		results = filtered
	}

	// Sort: touched forks first, then by name
	sort.Slice(results, func(i, j int) bool {
		if results[i].Untouched != results[j].Untouched {
			return !results[i].Untouched
		}
		return results[i].Name < results[j].Name
	})

	// Output
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	printResults(results)
	return nil
}

func printResults(forks []Fork) {
	if len(forks) == 0 {
		fmt.Println("No active forks found. Use --all to see untouched forks.")
		return
	}

	for i := range forks {
		f := &forks[i]
		status := ""
		if f.Untouched {
			status = " (untouched)"
		}
		fmt.Printf("\n%s%s\n", f.FullName, status)
		fmt.Printf("  upstream: %s\n", f.ParentFullName)

		// Deviation
		if f.Ahead > 0 || f.Behind > 0 {
			fmt.Printf("  deviation: ")
			parts := []string{}
			if f.Ahead > 0 {
				parts = append(parts, fmt.Sprintf("%d ahead", f.Ahead))
			}
			if f.Behind > 0 {
				parts = append(parts, fmt.Sprintf("%d behind", f.Behind))
			}
			fmt.Println(strings.Join(parts, ", "))
		} else {
			fmt.Println("  deviation: in sync")
		}

		// Branches
		nonDefaultBranches := []Branch{}
		for _, b := range f.Branches {
			if !b.IsDefault {
				nonDefaultBranches = append(nonDefaultBranches, b)
			}
		}
		if len(nonDefaultBranches) > 0 {
			fmt.Printf("  branches: %d non-default\n", len(nonDefaultBranches))
			for _, b := range nonDefaultBranches {
				fmt.Printf("    - %s (pushed %s)\n", b.Name, b.LastPush)
			}
		}

		// Open PRs
		if f.OpenPRs > 0 {
			fmt.Printf("  open PRs: %d\n", f.OpenPRs)
		}
	}
}

type ghRunner struct {
	profile string
	tmpDir  string
}

func (g *ghRunner) run(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)

	if g.profile != "" {
		// Set up temporary gh config for identity
		if g.tmpDir == "" {
			if err := g.setupIdentity(); err != nil {
				return nil, err
			}
		}
		cmd.Env = append(os.Environ(), fmt.Sprintf("GH_CONFIG_DIR=%s", g.tmpDir))
	}

	return cmd.Output()
}

func (g *ghRunner) setupIdentity() error {
	profile, err := identity.Get(g.profile)
	if err != nil {
		return fmt.Errorf("profile %q not found: %w", g.profile, err)
	}

	if profile.GHUser == "" {
		return fmt.Errorf("profile %q has no GitHub user configured", g.profile)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gh-wtfork-*")
	if err != nil {
		return err
	}
	g.tmpDir = tmpDir

	// Find real gh config dir
	realConfigDir := os.Getenv("GH_CONFIG_DIR")
	if realConfigDir == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			realConfigDir = filepath.Join(xdg, "gh")
		} else {
			home, _ := os.UserHomeDir()
			realConfigDir = filepath.Join(home, ".config", "gh")
		}
	}

	// Symlink config.yml
	realConfig := filepath.Join(realConfigDir, "config.yml")
	if _, err := os.Stat(realConfig); err == nil {
		_ = os.Symlink(realConfig, filepath.Join(tmpDir, "config.yml"))
	}

	// Write hosts.yml
	hostsContent := fmt.Sprintf(`github.com:
    git_protocol: ssh
    users:
        %s:
    user: %s
`, profile.GHUser, profile.GHUser)

	return os.WriteFile(filepath.Join(tmpDir, "hosts.yml"), []byte(hostsContent), 0o600)
}

func (g *ghRunner) cleanup() {
	if g.tmpDir != "" {
		_ = os.RemoveAll(g.tmpDir)
	}
}

func (g *ghRunner) checkAuth() error {
	_, err := g.run("auth", "status")
	if err != nil {
		if g.profile != "" {
			return fmt.Errorf("not authenticated as profile %q. Run: gh auth login", g.profile)
		}
		return fmt.Errorf("not authenticated. Run: gh auth login")
	}
	return nil
}

type ghRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"nameWithOwner"`
	URL           string `json:"url"`
	IsFork        bool   `json:"isFork"`
	DefaultBranch struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
	Parent *struct {
		Name          string `json:"name"`
		FullName      string `json:"nameWithOwner"`
		DefaultBranch struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	} `json:"parent"`
}

func (g *ghRunner) listForks() ([]ghRepo, error) {
	out, err := g.run("api", "graphql", "-f", `query=
		query {
			viewer {
				repositories(first: 100, isFork: true, ownerAffiliations: OWNER) {
					nodes {
						name
						nameWithOwner
						url
						isFork
						defaultBranchRef { name }
						parent {
							name
							nameWithOwner
							defaultBranchRef { name }
						}
					}
				}
			}
		}
	`)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Viewer struct {
				Repositories struct {
					Nodes []ghRepo `json:"nodes"`
				} `json:"repositories"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	return result.Data.Viewer.Repositories.Nodes, nil
}

func (g *ghRunner) analyzeFork(repo *ghRepo) (Fork, error) { //nolint:unparam // error kept for future use
	f := Fork{
		Name:          repo.Name,
		FullName:      repo.FullName,
		URL:           repo.URL,
		DefaultBranch: repo.DefaultBranch.Name,
	}

	if repo.Parent != nil {
		f.ParentName = repo.Parent.Name
		f.ParentFullName = repo.Parent.FullName
	}

	// Get comparison with upstream
	if repo.Parent != nil {
		comparison, err := g.getComparison(repo.FullName, repo.Parent.FullName, repo.DefaultBranch.Name)
		if err == nil {
			f.Ahead = comparison.AheadBy
			f.Behind = comparison.BehindBy
		}
	}

	// Get branches
	branches, err := g.getBranches(repo.FullName)
	if err == nil {
		f.Branches = branches
	}

	// Get open PRs
	prs, err := g.getOpenPRs(repo.FullName)
	if err == nil {
		f.OpenPRs = prs
	}

	// Determine if untouched
	nonDefaultBranches := 0
	for _, b := range f.Branches {
		if !b.IsDefault {
			nonDefaultBranches++
		}
	}
	f.Untouched = f.Ahead == 0 && nonDefaultBranches == 0 && f.OpenPRs == 0

	return f, nil
}

type comparison struct {
	AheadBy  int `json:"ahead_by"`
	BehindBy int `json:"behind_by"`
}

func (g *ghRunner) getComparison(forkFullName, parentFullName, branch string) (comparison, error) {
	// Compare fork's default branch with upstream's default branch
	endpoint := fmt.Sprintf("repos/%s/compare/%s:%s...%s:%s",
		parentFullName,
		strings.Split(parentFullName, "/")[0], branch,
		strings.Split(forkFullName, "/")[0], branch,
	)

	out, err := g.run("api", endpoint, "--jq", "{ahead_by, behind_by}")
	if err != nil {
		return comparison{}, err
	}

	var c comparison
	if err := json.Unmarshal(out, &c); err != nil {
		return comparison{}, err
	}

	return c, nil
}

func (g *ghRunner) getBranches(repoFullName string) ([]Branch, error) {
	// Get default branch first
	defaultOut, err := g.run("api", fmt.Sprintf("repos/%s", repoFullName), "--jq", ".default_branch")
	if err != nil {
		return nil, err
	}
	defaultBranch := strings.TrimSpace(string(defaultOut))

	// Get branches with commit info
	out, err := g.run("api", fmt.Sprintf("repos/%s/branches", repoFullName))
	if err != nil {
		return nil, err
	}

	var rawBranches []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	if err := json.Unmarshal(out, &rawBranches); err != nil {
		return nil, err
	}

	var branches []Branch
	for _, b := range rawBranches {
		lastPush := ""
		// Get commit date for non-default branches only (to reduce API calls)
		if b.Name != defaultBranch {
			commitOut, err := g.run("api", fmt.Sprintf("repos/%s/commits/%s", repoFullName, b.Commit.SHA),
				"--jq", ".commit.committer.date")
			if err == nil {
				lastPush = formatDate(strings.TrimSpace(string(commitOut)))
			}
		}

		branches = append(branches, Branch{
			Name:      b.Name,
			LastPush:  lastPush,
			IsDefault: b.Name == defaultBranch,
		})
	}

	return branches, nil
}

func (g *ghRunner) getOpenPRs(repoFullName string) (int, error) {
	out, err := g.run("api", fmt.Sprintf("repos/%s/pulls?state=open", repoFullName), "--jq", "length")
	if err != nil {
		return 0, err
	}

	var count int
	if err := json.Unmarshal(out, &count); err != nil {
		return 0, err
	}

	return count, nil
}

func formatDate(isoDate string) string {
	if len(isoDate) >= 10 {
		return isoDate[:10]
	}
	return isoDate
}
