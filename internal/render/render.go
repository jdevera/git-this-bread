package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jdevera/git-this-bread/internal/analyzer"
)

// Nerdfont icons
var Icons = map[string]string{
	"repo":       "\uf1d3", // nf-fa-git_square
	"fork":       "\uf402", // nf-oct-repo_forked
	"clone":      "\uf24d", // nf-fa-clone
	"branch":     "\ue725", // nf-dev-git_branch
	"commit":     "\uf417", // nf-oct-git_commit
	"remote":     "\uf0c2", // nf-fa-cloud
	"dirty":      "\uf044", // nf-fa-pencil
	"clean":      "\uf00c", // nf-fa-check
	"unpushed":   "\uf062", // nf-fa-arrow_up
	"stash":      "\uf187", // nf-fa-archive
	"calendar":   "\uf073", // nf-fa-calendar
	"error":      "\uf071", // nf-fa-warning
	"no_contrib": "\uf05e", // nf-fa-ban
	"folder":     "\uf07b", // nf-fa-folder
}

// Styles
var (
	green        = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	greenBold    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	magenta      = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	magentaBold  = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	blue         = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	blueBold     = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	yellow       = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	red          = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	redBold      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	dim          = lipgloss.NewStyle().Faint(true)
	dimItalic    = lipgloss.NewStyle().Faint(true).Italic(true)
	whiteBold    = lipgloss.NewStyle().Bold(true)
)

type Options struct {
	Verbose    bool
	ShowAdvice bool
	UseJSON    bool
}

func RenderRepo(info analyzer.RepoInfo, opts Options) {
	if opts.UseJSON {
		data, _ := json.MarshalIndent(toMap(info), "", "  ")
		fmt.Println(string(data))
		return
	}

	if opts.Verbose {
		renderRepoVerbose(info, opts)
	} else {
		renderRepoCompact(info, opts)
	}
}

// renderRepoCompact renders a single-line summary of the repo
func renderRepoCompact(info analyzer.RepoInfo, opts Options) {
	if !info.IsGitRepo {
		fmt.Printf("%s %s  %s\n",
			dim.Render(Icons["folder"]),
			dim.Render(info.Name),
			dimItalic.Render("not a git repo"))
		return
	}

	if info.Error != "" {
		fmt.Printf("%s %s  %s\n",
			red.Render(Icons["error"]),
			redBold.Render(info.Name),
			red.Render(info.Error))
		return
	}

	hasContributions := info.HasUserRemote || info.TotalUserCommits > 0

	// Determine icon and style
	var icon, nameStyle string
	if info.IsFork {
		icon = Icons["fork"]
		nameStyle = magentaBold.Render(info.Name)
	} else if hasContributions {
		icon = Icons["repo"]
		nameStyle = greenBold.Render(info.Name)
	} else {
		icon = Icons["clone"]
		nameStyle = whiteBold.Render(info.Name)
	}

	// Build output line
	var parts []string
	parts = append(parts, icon+" "+nameStyle)

	// Branch
	if info.CurrentBranch != "" {
		parts = append(parts, magenta.Render(Icons["branch"]+" "+info.CurrentBranch))
	}

	// Remote
	if info.HasUserRemote {
		parts = append(parts, greenBold.Render(Icons["remote"]+" "+strings.Join(info.UserRemotes, ",")))
	}

	// Commits
	if info.TotalUserCommits > 0 {
		parts = append(parts, blueBold.Render(fmt.Sprintf("%s %d", Icons["commit"], info.TotalUserCommits)))
	}

	// Last commit date
	if info.LastRepoCommitDate != "" {
		parts = append(parts, dim.Render(Icons["calendar"]+" "+info.LastRepoCommitDate))
	}

	// Dirty
	if info.HasUncommittedChanges {
		dirtyStr := "dirty"
		if info.DirtyDetails != nil {
			dirtyStr = info.DirtyDetails.String()
		}
		parts = append(parts, yellow.Render(Icons["dirty"]+" "+dirtyStr))
	}

	// Unpushed
	if info.Ahead > 0 {
		parts = append(parts, redBold.Render(fmt.Sprintf("%s %d unpushed", Icons["unpushed"], info.Ahead)))
	}

	// Stash
	if info.StashCount > 0 {
		parts = append(parts, magenta.Render(fmt.Sprintf("%s %d stash", Icons["stash"], info.StashCount)))
	}

	// Fork indicator
	if info.IsFork {
		parts = append(parts, dimItalic.Render("fork"))
	}

	// No contributions
	if !hasContributions {
		parts = append(parts, dim.Render(Icons["no_contrib"])+" "+dimItalic.Render("no contributions"))
	}

	fmt.Println(strings.Join(parts, "  "))

	// Advice
	if opts.ShowAdvice {
		for _, advice := range GetAdvice(info) {
			fmt.Printf("    → %s\n", advice)
		}
	}
}

// renderRepoVerbose renders a detailed multi-line view of the repo
func renderRepoVerbose(info analyzer.RepoInfo, opts Options) {
	if !info.IsGitRepo {
		fmt.Printf("%s %s  %s\n",
			dim.Render(Icons["folder"]),
			dim.Render(info.Name),
			dimItalic.Render("not a git repo"))
		return
	}

	if info.Error != "" {
		fmt.Printf("%s %s  %s\n",
			red.Render(Icons["error"]),
			redBold.Render(info.Name),
			red.Render(info.Error))
		return
	}

	hasContributions := info.HasUserRemote || info.TotalUserCommits > 0

	// Determine icon and style for repo name
	var icon, nameStyle string
	if info.IsFork {
		icon = Icons["fork"]
		nameStyle = magentaBold.Render(info.Name)
	} else if hasContributions {
		icon = Icons["repo"]
		nameStyle = greenBold.Render(info.Name)
	} else {
		icon = Icons["clone"]
		nameStyle = whiteBold.Render(info.Name)
	}

	// Repo name
	fmt.Printf("%s %s\n", icon, nameStyle)

	// Branch
	if info.CurrentBranch != "" {
		fmt.Printf("    %s %s\n", magenta.Render(Icons["branch"]), magenta.Render(info.CurrentBranch))
	}

	// Remotes (show all with full URLs)
	if len(info.AllRemotes) == 1 {
		r := info.AllRemotes[0]
		mine := ""
		if r.IsMine {
			mine = greenBold.Render(" (mine)")
		}
		fmt.Printf("    %s %s → %s%s\n",
			green.Render(Icons["remote"]),
			green.Render(r.Name),
			green.Render(r.URL),
			mine)
	} else if len(info.AllRemotes) > 1 {
		fmt.Printf("    %s %s\n", green.Render(Icons["remote"]), green.Render("Remotes:"))
		for _, r := range info.AllRemotes {
			mine := ""
			if r.IsMine {
				mine = greenBold.Render(" (mine)")
			}
			fmt.Printf("        %s → %s%s\n",
				green.Render(r.Name),
				dim.Render(r.URL),
				mine)
		}
	}

	// Commits
	if info.TotalUserCommits > 0 {
		fmt.Printf("    %s %s\n",
			blueBold.Render(Icons["commit"]),
			blueBold.Render(fmt.Sprintf("%d commits by you", info.TotalUserCommits)))
	}

	// Last commit date
	if info.LastRepoCommitDate != "" {
		fmt.Printf("    %s Last commit: %s\n",
			dim.Render(Icons["calendar"]),
			dim.Render(info.LastRepoCommitDate))
	}

	// Dirty
	if info.HasUncommittedChanges {
		dirtyStr := "dirty"
		if info.DirtyDetails != nil {
			dirtyStr = info.DirtyDetails.String()
		}
		fmt.Printf("    %s %s\n", yellow.Render(Icons["dirty"]), yellow.Render(dirtyStr))
	}

	// Unpushed
	if info.Ahead > 0 {
		fmt.Printf("    %s %s\n",
			redBold.Render(Icons["unpushed"]),
			redBold.Render(fmt.Sprintf("%d unpushed", info.Ahead)))
	}

	// Stash
	if info.StashCount > 0 {
		fmt.Printf("    %s %s\n",
			magenta.Render(Icons["stash"]),
			magenta.Render(fmt.Sprintf("%d stash", info.StashCount)))
	}

	// No contributions
	if !hasContributions {
		fmt.Printf("    %s %s\n",
			dim.Render(Icons["no_contrib"]),
			dimItalic.Render("no contributions"))
	}

	// Branches with user commits
	if len(info.BranchesWithCommits) > 0 {
		fmt.Println()
		fmt.Println("    Branches with your commits:")
		for i, branch := range info.BranchesWithCommits {
			if i >= 5 {
				break
			}
			marker := "○"
			style := dim
			nameWidth := 30
			if branch.IsCurrent {
				marker = "●"
				style = green
			}
			commits := "commit"
			if branch.CommitCount != 1 {
				commits = "commits"
			}
			fmt.Printf("        %s %-*s  %d %s  (%s)\n",
				style.Render(marker),
				nameWidth,
				style.Render(branch.Name),
				branch.CommitCount,
				commits,
				branch.LastCommitDate)
		}
	}

	// Advice
	if opts.ShowAdvice {
		adviceList := GetAdvice(info)
		if len(adviceList) > 0 {
			fmt.Println()
			fmt.Println("    Advice:")
			for _, advice := range adviceList {
				fmt.Printf("        → %s\n", advice)
			}
		}
	}

	fmt.Println()
}

func RenderTable(repos []analyzer.RepoInfo) {
	// Header
	fmt.Printf("%-30s %-12s %8s %-12s %s\n", "Repository", "Remote", "Commits", "Last", "Status")
	fmt.Println(strings.Repeat("-", 80))

	for _, info := range repos {
		if !info.IsGitRepo {
			continue
		}

		name := info.Name
		hasContributions := info.HasUserRemote || info.TotalUserCommits > 0
		if info.IsFork {
			name = Icons["fork"] + " " + name
		} else if hasContributions {
			name = Icons["repo"] + " " + name
		} else {
			name = Icons["clone"] + " " + name
		}

		remote := "-"
		if len(info.UserRemotes) > 0 {
			remote = strings.Join(info.UserRemotes, ",")
		}

		commits := "-"
		if info.TotalUserCommits > 0 {
			commits = fmt.Sprintf("%d", info.TotalUserCommits)
		}

		last := "-"
		if info.LastRepoCommitDate != "" {
			last = info.LastRepoCommitDate
		}

		var status []string
		if info.HasUncommittedChanges {
			status = append(status, Icons["dirty"])
		}
		if info.Ahead > 0 {
			status = append(status, fmt.Sprintf("%s%d", Icons["unpushed"], info.Ahead))
		}
		if info.StashCount > 0 {
			status = append(status, fmt.Sprintf("%s%d", Icons["stash"], info.StashCount))
		}
		if len(status) == 0 {
			status = append(status, Icons["clean"])
		}

		fmt.Printf("%-30s %-12s %8s %-12s %s\n",
			truncate(name, 30),
			truncate(remote, 12),
			commits,
			last,
			strings.Join(status, " "))
	}
}

func RenderJSON(repos []analyzer.RepoInfo) {
	var data []map[string]interface{}
	for _, r := range repos {
		data = append(data, toMap(r))
	}
	out, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(out))
}

func PrintLegend() {
	fmt.Println()
	fmt.Println("Legend")
	fmt.Println()
	fmt.Println("Repository types:")
	fmt.Printf("  %s name     Repository with your contributions\n", Icons["repo"])
	fmt.Printf("  %s name     Fork (has upstream remote)\n", Icons["fork"])
	fmt.Printf("  %s name     Clone without contributions\n", Icons["clone"])
	fmt.Println()
	fmt.Println("Status indicators:")
	fmt.Printf("  %s branch   Current branch name\n", Icons["branch"])
	fmt.Printf("  %s origin   Your remote\n", Icons["remote"])
	fmt.Printf("  %s N        Number of your commits\n", Icons["commit"])
	fmt.Printf("  %s date     Date of last commit\n", Icons["calendar"])
	fmt.Printf("  %s dirty    Uncommitted changes\n", Icons["dirty"])
	fmt.Printf("  %s N        Unpushed commits\n", Icons["unpushed"])
	fmt.Printf("  %s N        Stashed changes\n", Icons["stash"])
	fmt.Println()
}

func GetAdvice(info analyzer.RepoInfo) []string {
	var advice []string
	hasContributions := info.HasUserRemote || info.TotalUserCommits > 0

	if !hasContributions {
		if info.HasUncommittedChanges || info.StashCount > 0 {
			advice = append(advice, "Has local changes but no remote - set up your fork or commit upstream")
		} else {
			advice = append(advice, "No contributions - consider removing if not needed")
		}
	}

	if info.HasUserRemote && info.TotalUserCommits == 0 {
		advice = append(advice, "Forked but no commits yet - start contributing or remove")
	}

	if info.Ahead > 0 {
		advice = append(advice, fmt.Sprintf("Push your %d unpushed commit(s)", info.Ahead))
	}

	if info.HasUncommittedChanges && info.DirtyDetails != nil {
		d := info.DirtyDetails
		if d.StagedFiles > 0 && d.UnstagedFiles == 0 && d.Untracked == 0 {
			advice = append(advice, fmt.Sprintf("Staged changes ready - commit %d file(s)", d.StagedFiles))
		}
		if d.Untracked > 5 {
			advice = append(advice, fmt.Sprintf("%d untracked files - add to .gitignore or stage", d.Untracked))
		}
	}

	if info.StashCount > 0 {
		advice = append(advice, fmt.Sprintf("Review %d stash(es) - apply or drop", info.StashCount))
	}

	return advice
}

func toMap(info analyzer.RepoInfo) map[string]interface{} {
	m := map[string]interface{}{
		"name":       info.Name,
		"path":       info.Path,
		"is_git_repo": info.IsGitRepo,
	}
	if !info.IsGitRepo {
		return m
	}
	if info.Error != "" {
		m["error"] = info.Error
		return m
	}

	m["current_branch"] = info.CurrentBranch
	m["default_branch"] = info.DefaultBranch
	m["is_fork"] = info.IsFork
	m["commits"] = map[string]interface{}{
		"user_total":       info.TotalUserCommits,
		"last_user_commit": info.LastCommitDate,
		"last_repo_commit": info.LastRepoCommitDate,
	}
	if info.DirtyDetails != nil {
		m["dirty"] = map[string]interface{}{
			"staged":    info.DirtyDetails.StagedFiles,
			"unstaged":  info.DirtyDetails.UnstagedFiles,
			"untracked": info.DirtyDetails.Untracked,
		}
	}
	m["ahead"] = info.Ahead
	m["stash_count"] = info.StashCount

	var remotes []map[string]interface{}
	for _, r := range info.AllRemotes {
		remotes = append(remotes, map[string]interface{}{
			"name":    r.Name,
			"url":     r.URL,
			"is_mine": r.IsMine,
		})
	}
	m["remotes"] = remotes

	return m
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// lipgloss handles NO_COLOR automatically via termenv
