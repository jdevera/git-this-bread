package llmadvice

import (
	"fmt"
	"strings"

	"github.com/jdevera/git-this-bread/internal/analyzer"
)

const systemPrompt = `Git advisor for an experienced developer. Be brief.

You receive: repo state + basic algorithmic advice.
Your job: Rephrase and enhance the basic advice, adding context-aware insights.

Rules:
- MAX 5 suggestions total
- One short sentence each (under 15 words)
- Include important items from basic advice (unpushed commits, uncommitted work)
- Enhance with context: mention specific files, branches, ages
- Add insights the algorithm misses: stale branches, old stashes, patterns
- No git commands - user knows git
- If all good, just say "All good"

Format: numbered list, nothing else.
`

// FormatSingleRepoPrompt formats a single repo's state for the LLM
func FormatSingleRepoPrompt(info *analyzer.RepoInfo, basicAdvice []string, customInstructions string) string {
	var sb strings.Builder

	sb.WriteString(systemPrompt)

	if customInstructions != "" {
		sb.WriteString("\nAdditional instructions: ")
		sb.WriteString(customInstructions)
		sb.WriteString("\n")
	}

	sb.WriteString("\n\nRepository State:\n")
	sb.WriteString(formatRepoState(info))

	if len(basicAdvice) > 0 {
		sb.WriteString("\nBasic Advice (from algorithm):\n")
		for _, a := range basicAdvice {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
	} else {
		sb.WriteString("\nBasic Advice: (none - algorithm found nothing to suggest)\n")
	}

	return sb.String()
}

// FormatMultiRepoPrompt formats multiple repos for combined analysis
func FormatMultiRepoPrompt(repos []*analyzer.RepoInfo, basicAdvicePerRepo map[string][]string, customInstructions string) string {
	var sb strings.Builder

	sb.WriteString(systemPrompt)

	if customInstructions != "" {
		sb.WriteString("\nAdditional instructions: ")
		sb.WriteString(customInstructions)
		sb.WriteString("\n")
	}

	sb.WriteString("\n\nMultiple Repository States:\n")
	sb.WriteString("Provide an overall summary and prioritized actions across all repositories.\n\n")

	for i, info := range repos {
		sb.WriteString(fmt.Sprintf("--- Repository %d: %s ---\n", i+1, info.Name))
		sb.WriteString(formatRepoState(info))
		if advice, ok := basicAdvicePerRepo[info.Name]; ok && len(advice) > 0 {
			sb.WriteString("Basic Advice:\n")
			for _, a := range advice {
				sb.WriteString(fmt.Sprintf("  - %s\n", a))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatRepoState(info *analyzer.RepoInfo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name: %s\n", info.Name))

	// Branch context
	if info.CurrentBranch != "" {
		branchType := ""
		if info.DefaultBranch != "" && info.CurrentBranch == info.DefaultBranch {
			branchType = " (default branch)"
		} else if info.DefaultBranch != "" {
			branchType = " (feature branch)"
		}
		sb.WriteString(fmt.Sprintf("Current Branch: %s%s\n", info.CurrentBranch, branchType))
	}

	if info.IsFork {
		sb.WriteString("Type: Fork\n")
		if info.UpstreamURL != "" {
			sb.WriteString(fmt.Sprintf("Upstream: %s\n", info.UpstreamURL))
		}
	}

	// Unpushed commits with details
	if info.Ahead > 0 {
		sb.WriteString(fmt.Sprintf("Unpushed Commits: %d\n", info.Ahead))
	}
	if info.Behind > 0 {
		sb.WriteString(fmt.Sprintf("Behind Remote: %d commits\n", info.Behind))
	}

	// Recent commits for context
	if len(info.RecentCommits) > 0 {
		sb.WriteString("Recent Commits:\n")
		for _, c := range info.RecentCommits {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", c.Hash, c.Message, c.Date))
		}
	}

	// Uncommitted changes with file names
	if info.HasUncommittedChanges && info.DirtyDetails != nil {
		d := info.DirtyDetails
		sb.WriteString("Uncommitted Changes:\n")
		if d.StagedFiles > 0 {
			sb.WriteString(fmt.Sprintf("  - Staged (%d files, +%d/-%d lines): %s\n",
				d.StagedFiles, d.StagedInsertions, d.StagedDeletions,
				formatFileList(d.StagedNames, 5)))
		}
		if d.UnstagedFiles > 0 {
			sb.WriteString(fmt.Sprintf("  - Modified (%d files, +%d/-%d lines): %s\n",
				d.UnstagedFiles, d.UnstagedInsertions, d.UnstagedDeletions,
				formatFileList(d.UnstagedNames, 5)))
		}
		if d.Untracked > 0 {
			sb.WriteString(fmt.Sprintf("  - Untracked (%d files): %s\n",
				d.Untracked, formatFileList(d.UntrackedNames, 5)))
		}
	}

	// Stashes with details
	if info.StashCount > 0 {
		sb.WriteString(fmt.Sprintf("Stashes (%d):\n", info.StashCount))
		for _, s := range info.Stashes {
			sb.WriteString(fmt.Sprintf("  - stash@{%d}: %s (%s)\n", s.Index, s.Message, s.Date))
		}
	}

	// Branches with user commits
	if len(info.BranchesWithCommits) > 0 {
		sb.WriteString("Your Branches:\n")
		for _, b := range info.BranchesWithCommits {
			current := ""
			if b.IsCurrent {
				current = " (current)"
			}
			sb.WriteString(fmt.Sprintf("  - %s: %d commits, last %s%s\n",
				b.Name, b.CommitCount, b.LastCommitDate, current))
		}
	}

	hasContributions := info.HasUserRemote || info.TotalUserCommits > 0
	if !hasContributions {
		sb.WriteString("Note: No user contributions detected in this repo\n")
	}

	return sb.String()
}

// formatFileList formats a list of files, truncating if too many
func formatFileList(files []string, limit int) string {
	if len(files) == 0 {
		return ""
	}
	if len(files) <= limit {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:limit], ", ") + fmt.Sprintf(" (+%d more)", len(files)-limit)
}

// parseAdviceResponse parses the LLM response into individual advice strings
func parseAdviceResponse(response string) []string {
	var advice []string
	lines := strings.Split(strings.TrimSpace(response), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove numbering if present (e.g., "1. ", "- ")
		if len(line) > 2 {
			if (line[0] >= '1' && line[0] <= '9') && (line[1] == '.' || line[1] == ')') {
				line = strings.TrimSpace(line[2:])
			} else if line[0] == '-' || line[0] == '*' {
				line = strings.TrimSpace(line[1:])
			}
		}

		if line != "" {
			advice = append(advice, line)
		}
	}

	return advice
}
