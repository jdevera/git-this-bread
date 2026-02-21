package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/identity"
)

var (
	asProfile  string
	showAll    bool
	jsonOutput bool
	noCache    bool
)

// Styles
var (
	greenBold = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	green     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	red       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	cyan      = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dim       = lipgloss.NewStyle().Faint(true)
	dimItalic = lipgloss.NewStyle().Faint(true).Italic(true)
)

// Icons
var icons = map[string]string{
	"fork":     "\uf402", // nf-oct-repo_forked
	"upstream": "\uf062", // nf-fa-arrow_up
	"branch":   "\ue725", // nf-dev-git_branch
	"pr":       "\uf407", // nf-oct-git_pull_request
	"merged":   "\uf419", // nf-oct-git_merge
	"closed":   "\uf659", // nf-mdi-close_circle
	"sync":     "\uf021", // nf-fa-refresh
	"ahead":    "\uf176", // nf-fa-long_arrow_up
	"behind":   "\uf175", // nf-fa-long_arrow_down
	"check":    "\uf00c", // nf-fa-check
	"warning":  "\uf071", // nf-fa-warning
	"spinner":  "\uf110", // nf-fa-spinner
}

// PR states
const (
	PRStateOpen   = "OPEN"
	PRStateMerged = "MERGED"
	PRStateClosed = "CLOSED"
)

// Fork categories
const (
	CategoryMaintained   = "maintained"   // Ahead on default branch - you're keeping your own version
	CategoryContribution = "contribution" // Not ahead, but has branches/PRs - just for contributing
	CategoryUntouched    = "untouched"    // No changes - can be deleted
)

type Fork struct {
	Name           string   `json:"name"`
	FullName       string   `json:"full_name"`
	URL            string   `json:"html_url"`
	ParentName     string   `json:"parent_name"`
	ParentFullName string   `json:"parent_full_name"`
	DefaultBranch  string   `json:"default_branch"`
	Category       string   `json:"category"` // maintained, contribution, or untouched
	Ahead          int      `json:"ahead"`
	Behind         int      `json:"behind"`
	ForkLastCommit string   `json:"fork_last_commit,omitempty"`     // Last commit on fork's default branch
	ForkLastAgo    string   `json:"fork_last_ago,omitempty"`        // Relative time
	UpstreamLast   string   `json:"upstream_last_commit,omitempty"` // Last commit on upstream's default branch
	UpstreamAgo    string   `json:"upstream_last_ago,omitempty"`    // Relative time
	Branches       []Branch `json:"branches,omitempty"`
	Untouched      bool     `json:"untouched"` // Deprecated: use Category == CategoryUntouched
}

type Branch struct {
	Name      string `json:"name"`
	Date      string `json:"date"`     // ISO date
	DateAgo   string `json:"date_ago"` // Human-readable relative time
	IsDefault bool   `json:"is_default"`
	PR        *PR    `json:"pr,omitempty"` // Associated PR if any
}

type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"` // OPEN, MERGED, CLOSED
	URL    string `json:"url"`
}

var rootCmd = &cobra.Command{
	Use:   "gh-wtfork",
	Short: "What the fork? Analyze your GitHub forks",
	Long: `gh-wtfork (a git-this-bread tool)

Triage years of GitHub forks. Categorizes your forks into:

  • Maintained    — ahead on default branch (your own version)
  • Contribution  — has branches/PRs (contributing upstream)
  • Untouched     — no changes (can probably delete)

For each fork shows deviation with temporal context, branches
with age, and linked PR status (open/merged/closed).

Use --as to run with a specific identity profile managed by git-id.`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVar(&asProfile, "as", "", "Run as identity profile (managed by git-id)")
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all forks (default: hide untouched)")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Bypass cache (still refreshes it)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Progress update sent from workers
type progressUpdate struct {
	repo   string
	action string
}

func run(cmd *cobra.Command, args []string) error {
	ghCmd := &ghRunner{profile: asProfile}
	defer ghCmd.cleanup()

	// Show immediate feedback
	fmt.Fprintf(os.Stderr, "%s %s",
		cyan.Render("⠋"),
		dim.Render("Checking authentication..."))

	if err := ghCmd.checkAuth(); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[K")
		return err
	}

	fmt.Fprintf(os.Stderr, "\r\033[K%s %s",
		cyan.Render("⠙"),
		dim.Render("Fetching fork list..."))

	forks, err := ghCmd.listForks()
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear before error or continue

	if err != nil {
		return fmt.Errorf("failed to list forks: %w", err)
	}

	if len(forks) == 0 {
		fmt.Println("No forks found.")
		return nil
	}

	// Parallel analysis with progress updates
	total := len(forks)
	results := make([]Fork, total)
	errors := make([]error, total)

	// Progress channel for sub-action updates
	progress := make(chan progressUpdate, 100)
	var completed atomic.Int32

	// Spinner goroutine - keeps progress on single line
	done := make(chan struct{})
	go func() {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		tick := 0
		lastUpdate := progressUpdate{}

		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case update := <-progress:
				lastUpdate = update
			case <-ticker.C:
				tick++
				spinChar := spinner[tick%len(spinner)]
				comp := completed.Load()

				// Build progress line, truncate to ~70 chars to avoid wrapping
				var line string
				if lastUpdate.repo != "" {
					repoName := lastUpdate.repo
					if len(repoName) > 20 {
						repoName = repoName[:17] + "..."
					}
					line = fmt.Sprintf("%s Analyzing [%d/%d] %s · %s",
						spinChar, comp, total, repoName, lastUpdate.action)
				} else {
					line = fmt.Sprintf("%s Analyzing [%d/%d]",
						spinChar, comp, total)
				}

				// Truncate if too long (terminal safe)
				if len(line) > 70 {
					line = line[:67] + "..."
				}

				fmt.Fprintf(os.Stderr, "\r\033[K%s", cyan.Render(line))
			}
		}
	}()

	// Worker pool - 5 concurrent workers to respect GitHub rate limits
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i := range forks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			analyzed, err := ghCmd.analyzeForkWithProgress(&forks[idx], progress)
			results[idx] = analyzed
			errors[idx] = err
			completed.Add(1)
		}(i)
	}

	wg.Wait()
	close(done)
	close(progress)

	// Collect results, report errors
	var finalResults []Fork
	for i := range results {
		if errors[i] != nil {
			fmt.Fprintf(os.Stderr, "\r\033[K  %s failed to analyze %s: %v\n",
				yellow.Render(icons["warning"]), forks[i].FullName, errors[i])
			continue
		}
		if results[i].FullName != "" {
			finalResults = append(finalResults, results[i])
		}
	}

	fmt.Fprintf(os.Stderr, "\r\033[K%s Analyzed %d forks\n\n",
		green.Render(icons["check"]), len(finalResults))

	results = finalResults

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

	// Sort: maintained > contribution > untouched, then by name
	categoryOrder := map[string]int{
		CategoryMaintained:   0,
		CategoryContribution: 1,
		CategoryUntouched:    2,
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Category != results[j].Category {
			return categoryOrder[results[i].Category] < categoryOrder[results[j].Category]
		}
		return results[i].Name < results[j].Name
	})

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
		fmt.Println(dim.Render("No active forks found. Use --all to see untouched forks."))
		return
	}

	// Group header tracking
	lastCategory := ""

	for i := range forks {
		f := &forks[i]

		// Print category header when it changes
		if f.Category != lastCategory {
			if lastCategory != "" {
				fmt.Println() // Extra space between categories
			}
			switch f.Category {
			case CategoryMaintained:
				fmt.Printf("%s %s\n", greenBold.Render("●"), greenBold.Render("Maintained"))
			case CategoryContribution:
				fmt.Printf("%s %s\n", yellow.Render("○"), yellow.Render("Contributions"))
			case CategoryUntouched:
				fmt.Printf("%s %s\n", dim.Render("·"), dim.Render("Untouched"))
			}
			lastCategory = f.Category
		}

		// Fork name with icon
		forkIcon := icons["fork"]
		var nameStyled string
		switch f.Category {
		case CategoryMaintained:
			nameStyled = greenBold.Render(f.FullName)
			fmt.Printf("%s %s\n", green.Render(forkIcon), nameStyled)
		case CategoryContribution:
			nameStyled = yellow.Render(f.FullName)
			fmt.Printf("%s %s\n", yellow.Render(forkIcon), nameStyled)
		case CategoryUntouched:
			nameStyled = dim.Render(f.FullName)
			fmt.Printf("%s %s\n", dim.Render(forkIcon), nameStyled)
		}

		// Upstream
		fmt.Printf("    %s %s\n", dim.Render(icons["upstream"]), dim.Render(f.ParentFullName))

		// Deviation with temporal context
		if f.Ahead > 0 || f.Behind > 0 {
			var parts []string
			if f.Ahead > 0 {
				aheadStr := fmt.Sprintf("%s %d ahead", icons["ahead"], f.Ahead)
				if f.ForkLastAgo != "" {
					aheadStr += fmt.Sprintf(" (%s)", f.ForkLastAgo)
				}
				parts = append(parts, greenBold.Render(aheadStr))
			}
			if f.Behind > 0 {
				behindStr := fmt.Sprintf("%s %d behind", icons["behind"], f.Behind)
				if f.UpstreamAgo != "" {
					behindStr += fmt.Sprintf(" (upstream: %s)", f.UpstreamAgo)
				}
				parts = append(parts, red.Render(behindStr))
			}
			fmt.Printf("    %s\n", strings.Join(parts, "  "))
		} else {
			syncStr := "in sync"
			if f.UpstreamAgo != "" {
				syncStr += fmt.Sprintf(" (upstream: %s)", f.UpstreamAgo)
			}
			fmt.Printf("    %s %s\n", green.Render(icons["sync"]), green.Render(syncStr))
		}

		// Branches (non-default only)
		var nonDefaultBranches []Branch
		for j := range f.Branches {
			if !f.Branches[j].IsDefault {
				nonDefaultBranches = append(nonDefaultBranches, f.Branches[j])
			}
		}

		if len(nonDefaultBranches) > 0 {
			for _, b := range nonDefaultBranches {
				branchLine := fmt.Sprintf("    %s %s", cyan.Render(icons["branch"]), cyan.Render(b.Name))

				// Date and age
				if b.Date != "" {
					branchLine += fmt.Sprintf("  %s", dim.Render(b.Date))
					if b.DateAgo != "" {
						branchLine += fmt.Sprintf(" · %s", dimItalic.Render(b.DateAgo))
					}
				}
				fmt.Println(branchLine)

				// PR info
				if b.PR != nil {
					prIcon := icons["pr"]
					prStyle := yellow // default for open
					stateLabel := "open"

					switch b.PR.State {
					case PRStateMerged:
						prIcon = icons["merged"]
						prStyle = greenBold
						stateLabel = "merged"
					case PRStateClosed:
						prIcon = icons["closed"]
						prStyle = red
						stateLabel = "closed"
					}

					fmt.Printf("        %s %s #%d %s\n",
						prStyle.Render(prIcon),
						prStyle.Render(stateLabel),
						b.PR.Number,
						dim.Render(truncate(b.PR.Title, 50)))
				}
			}
		}

		fmt.Println()
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// relativeTime returns a human-readable relative time string
// If years present: "Xy Xmo"
// If months present: "Xmo Xd"
// Otherwise: "Xd"
func relativeTime(isoDate string) string {
	if len(isoDate) < 10 {
		return ""
	}

	t, err := time.Parse("2006-01-02", isoDate[:10])
	if err != nil {
		// Try ISO 8601 format
		t, err = time.Parse(time.RFC3339, isoDate)
		if err != nil {
			return ""
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	days := int(diff.Hours() / 24)
	months := days / 30
	years := months / 12
	months %= 12
	days %= 30

	if years > 0 {
		if months > 0 {
			return fmt.Sprintf("%dy %dmo ago", years, months)
		}
		return fmt.Sprintf("%dy ago", years)
	}
	if months > 0 {
		if days > 0 {
			return fmt.Sprintf("%dmo %dd ago", months, days)
		}
		return fmt.Sprintf("%dmo ago", months)
	}
	if days > 0 {
		return fmt.Sprintf("%dd ago", days)
	}
	return "today"
}

type ghRunner struct {
	profile string
	tmpDir  string
}

func (g *ghRunner) run(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)

	if g.profile != "" {
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

	tmpDir, err := os.MkdirTemp("", "gh-wtfork-*")
	if err != nil {
		return err
	}
	g.tmpDir = tmpDir

	realConfigDir := os.Getenv("GH_CONFIG_DIR")
	if realConfigDir == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			realConfigDir = filepath.Join(xdg, "gh")
		} else {
			home, _ := os.UserHomeDir()
			realConfigDir = filepath.Join(home, ".config", "gh")
		}
	}

	realConfig := filepath.Join(realConfigDir, "config.yml")
	if _, err := os.Stat(realConfig); err == nil {
		_ = os.Symlink(realConfig, filepath.Join(tmpDir, "config.yml"))
	}

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

func (g *ghRunner) analyzeForkWithProgress(repo *ghRepo, progress chan<- progressUpdate) (Fork, error) { //nolint:unparam // error kept for future use
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

	// Get comparison with upstream and last commit dates
	if repo.Parent != nil {
		progress <- progressUpdate{repo: repo.Name, action: "comparing with upstream"}
		comparison, err := g.getComparison(repo.FullName, repo.Parent.FullName, repo.DefaultBranch.Name)
		if err == nil {
			f.Ahead = comparison.AheadBy
			f.Behind = comparison.BehindBy
		}

		// Get last commit dates for both fork and upstream default branches
		progress <- progressUpdate{repo: repo.Name, action: "checking commit dates"}
		if forkDate, err := g.getLastCommitDate(repo.FullName, repo.DefaultBranch.Name); err == nil {
			f.ForkLastCommit = formatDate(forkDate)
			f.ForkLastAgo = relativeTime(forkDate)
		}
		if upstreamDate, err := g.getLastCommitDate(repo.Parent.FullName, repo.Parent.DefaultBranch.Name); err == nil {
			f.UpstreamLast = formatDate(upstreamDate)
			f.UpstreamAgo = relativeTime(upstreamDate)
		}
	}

	// Get branches
	progress <- progressUpdate{repo: repo.Name, action: "fetching branches"}
	branches, err := g.getBranches(repo.FullName)
	if err == nil {
		f.Branches = branches
	}

	// Get PRs and link to branches
	if repo.Parent != nil {
		progress <- progressUpdate{repo: repo.Name, action: "fetching PRs"}
		prs, err := g.getPRsForFork(repo.FullName, repo.Parent.FullName)
		if err == nil {
			g.linkPRsToBranches(&f, prs)
		}
	}

	// Categorize the fork
	nonDefaultBranches := 0
	hasOpenPR := false
	for i := range f.Branches {
		b := &f.Branches[i]
		if !b.IsDefault {
			nonDefaultBranches++
		}
		if b.PR != nil && b.PR.State == PRStateOpen {
			hasOpenPR = true
		}
	}

	// Determine category:
	// - Maintained: ahead on default branch (you're keeping your own version)
	// - Contribution: not ahead, but has branches/PRs (just for contributing)
	// - Untouched: no changes at all
	switch {
	case f.Ahead > 0:
		f.Category = CategoryMaintained
	case nonDefaultBranches > 0 || hasOpenPR:
		f.Category = CategoryContribution
	default:
		f.Category = CategoryUntouched
	}
	f.Untouched = f.Category == CategoryUntouched

	return f, nil
}

type comparison struct {
	AheadBy  int `json:"ahead_by"`
	BehindBy int `json:"behind_by"`
}

func (g *ghRunner) getComparison(forkFullName, parentFullName, branch string) (comparison, error) {
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

func (g *ghRunner) getLastCommitDate(repoFullName, branch string) (string, error) {
	// Get the last commit on the specified branch
	endpoint := fmt.Sprintf("repos/%s/commits?sha=%s&per_page=1", repoFullName, branch)
	out, err := g.run("api", endpoint, "--jq", ".[0].commit.committer.date")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *ghRunner) getBranches(repoFullName string) ([]Branch, error) {
	defaultOut, err := g.run("api", fmt.Sprintf("repos/%s", repoFullName), "--jq", ".default_branch")
	if err != nil {
		return nil, err
	}
	defaultBranch := strings.TrimSpace(string(defaultOut))

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
		branch := Branch{
			Name:      b.Name,
			IsDefault: b.Name == defaultBranch,
		}

		// Get commit date for non-default branches only
		if b.Name != defaultBranch {
			commitOut, err := g.run("api", fmt.Sprintf("repos/%s/commits/%s", repoFullName, b.Commit.SHA),
				"--jq", ".commit.committer.date")
			if err == nil {
				isoDate := strings.TrimSpace(string(commitOut))
				branch.Date = formatDate(isoDate)
				branch.DateAgo = relativeTime(isoDate)
			}
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

// ghPR represents a pull request from the GitHub API
type ghPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Head   struct {
		Ref string `json:"ref"` // Branch name
	} `json:"headRefName"`
}

func (g *ghRunner) getPRsForFork(forkFullName, parentFullName string) ([]ghPR, error) {
	// Load cached PRs (unless --no-cache)
	var cache *PRCache
	if !noCache {
		cache, _ = loadPRCache(parentFullName)
	} else {
		cache = &PRCache{PRs: make(map[int]CachedPR)}
	}

	// Search for PRs from this fork to the parent repo
	forkOwner := strings.Split(forkFullName, "/")[0]

	// Use GraphQL search to find PRs authored by fork owner in parent repo
	searchQuery := fmt.Sprintf("is:pr repo:%s author:%s", parentFullName, forkOwner)

	query := fmt.Sprintf(`query {
		search(query: "%s", type: ISSUE, first: 100) {
			nodes {
				... on PullRequest {
					number
					title
					state
					url
					headRefName
				}
			}
		}
	}`, searchQuery)

	out, err := g.run("api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	if err != nil {
		// API failed - fall back to cache if available
		if len(cache.PRs) > 0 {
			var cachedPRs []ghPR
			for _, cpr := range cache.PRs {
				cachedPRs = append(cachedPRs, ghPR{
					Number: cpr.Number,
					Title:  cpr.Title,
					State:  cpr.State,
					URL:    cpr.URL,
					Head: struct {
						Ref string `json:"ref"`
					}{Ref: cpr.Branch},
				})
			}
			return cachedPRs, nil
		}
		return nil, err
	}

	var result struct {
		Data struct {
			Search struct {
				Nodes []struct {
					Number      int    `json:"number"`
					Title       string `json:"title"`
					State       string `json:"state"`
					URL         string `json:"url"`
					HeadRefName string `json:"headRefName"`
				} `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	var prs []ghPR
	for _, pr := range result.Data.Search.Nodes {
		if pr.Number == 0 {
			continue // Skip empty nodes
		}
		prs = append(prs, ghPR{
			Number: pr.Number,
			Title:  pr.Title,
			State:  pr.State,
			URL:    pr.URL,
			Head: struct {
				Ref string `json:"ref"`
			}{Ref: pr.HeadRefName},
		})
	}

	// Merge with cached PRs (adds old merged/closed PRs not in search results)
	prs = mergeCachedPRs(prs, cache)

	// Save merged/closed PRs to cache for next time
	_ = savePRCache(parentFullName, prs)

	return prs, nil
}

func (g *ghRunner) linkPRsToBranches(fork *Fork, prs []ghPR) {
	// Create a map of branch name to PRs (use the most relevant PR)
	branchPRs := make(map[string]*PR)

	for i := range prs {
		pr := &prs[i]
		branchName := pr.Head.Ref

		existing, exists := branchPRs[branchName]
		// Prefer: Open > Merged > Closed
		if !exists {
			branchPRs[branchName] = &PR{
				Number: pr.Number,
				Title:  pr.Title,
				State:  pr.State,
				URL:    pr.URL,
			}
		} else if pr.State == PRStateOpen || (pr.State == PRStateMerged && existing.State == PRStateClosed) {
			// Update if this PR is more relevant
			branchPRs[branchName] = &PR{
				Number: pr.Number,
				Title:  pr.Title,
				State:  pr.State,
				URL:    pr.URL,
			}
		}
	}

	// Link PRs to branches
	for i := range fork.Branches {
		if pr, ok := branchPRs[fork.Branches[i].Name]; ok {
			fork.Branches[i].PR = pr
		}
	}
}

func formatDate(isoDate string) string {
	if len(isoDate) >= 10 {
		return isoDate[:10]
	}
	return isoDate
}

// --- PR Cache ---
// Caches merged/closed PRs to avoid re-fetching data that won't change.

// CachedPR represents a PR stored in the cache
type CachedPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

// PRCache holds cached PRs for an upstream repo
type PRCache struct {
	PRs       map[int]CachedPR `json:"prs"` // keyed by PR number
	UpdatedAt time.Time        `json:"updated_at"`
}

// getCacheDir returns the cache directory for gh-wtfork
func getCacheDir() (string, error) {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheHome, "git-this-bread", "gh-wtfork", "prs"), nil
}

// cacheFileName returns a safe filename for an upstream repo
func cacheFileName(upstreamFullName string) string {
	// Replace / with _ for safe filename
	return strings.ReplaceAll(upstreamFullName, "/", "_") + ".json"
}

// loadPRCache loads cached PRs for an upstream repo
func loadPRCache(upstreamFullName string) (*PRCache, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(cacheDir, cacheFileName(upstreamFullName))
	data, err := os.ReadFile(cachePath) //nolint:gosec // cachePath is constructed safely from repo name
	if err != nil {
		if os.IsNotExist(err) {
			return &PRCache{PRs: make(map[int]CachedPR)}, nil
		}
		return nil, err
	}

	var cache PRCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache, start fresh
		return &PRCache{PRs: make(map[int]CachedPR)}, nil
	}

	if cache.PRs == nil {
		cache.PRs = make(map[int]CachedPR)
	}

	return &cache, nil
}

// savePRCache saves PRs to the cache (only merged/closed)
func savePRCache(upstreamFullName string, prs []ghPR) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return err
	}

	// Load existing cache to preserve PRs we didn't fetch this time
	cache, _ := loadPRCache(upstreamFullName)

	// Add/update merged and closed PRs
	for _, pr := range prs {
		if pr.State == PRStateMerged || pr.State == PRStateClosed {
			cache.PRs[pr.Number] = CachedPR{
				Number: pr.Number,
				Title:  pr.Title,
				State:  pr.State,
				URL:    pr.URL,
				Branch: pr.Head.Ref,
			}
		}
	}

	cache.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, cacheFileName(upstreamFullName))
	return os.WriteFile(cachePath, data, 0o600)
}

// mergeCachedPRs merges cached PRs with freshly fetched PRs
// Fresh data takes precedence (a cached "open" PR might now be "merged")
func mergeCachedPRs(fresh []ghPR, cached *PRCache) []ghPR {
	// Build a set of PR numbers we already have
	seen := make(map[int]bool)
	for _, pr := range fresh {
		seen[pr.Number] = true
	}

	// Add cached PRs that weren't in fresh results
	// (This can happen if the search API didn't return old merged PRs)
	for _, cpr := range cached.PRs {
		if !seen[cpr.Number] {
			fresh = append(fresh, ghPR{
				Number: cpr.Number,
				Title:  cpr.Title,
				State:  cpr.State,
				URL:    cpr.URL,
				Head: struct {
					Ref string `json:"ref"`
				}{Ref: cpr.Branch},
			})
		}
	}

	return fresh
}
