package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	insertionRe = regexp.MustCompile(`(\d+) insertion`)
	deletionRe  = regexp.MustCompile(`(\d+) deletion`)
)

// Config for identifying user commits (loaded from git config)
var (
	userEmail    string
	githubUser   string
	configLoaded bool
	configError  error
)

// LoadGitConfig loads required git config values. Returns an error if required values are missing.
//
// We use the git command rather than go-git's config API because go-git does not support
// [include] or [includeIf] directives (see https://github.com/go-git/go-git/issues/395).
// The git command properly handles all config levels (system, global, local) and includes.
func LoadGitConfig() error {
	if configLoaded {
		return configError
	}
	configLoaded = true

	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		userEmail = strings.TrimSpace(string(out))
	}

	if out, err := exec.Command("git", "config", "github.user").Output(); err == nil {
		githubUser = strings.TrimSpace(string(out))
	}

	// Validate required config
	var missing []string
	if userEmail == "" {
		missing = append(missing, "user.email")
	}
	if githubUser == "" {
		missing = append(missing, "github.user")
	}

	if len(missing) > 0 {
		configError = fmt.Errorf(`missing required git config: %s

Set them with:
    git config --global user.email "you@example.com"
    git config --global github.user "yourusername"`, strings.Join(missing, ", "))
		return configError
	}

	return nil
}

// isUserRemote checks if a remote URL belongs to the user
func isUserRemote(url string) bool {
	url = strings.ToLower(url)
	return githubUser != "" && strings.Contains(url, strings.ToLower(githubUser))
}

type Options struct {
	Verbose bool
}

type DirtyDetails struct {
	Untracked         int
	StagedFiles       int
	StagedInsertions  int
	StagedDeletions   int
	UnstagedFiles     int
	UnstagedInsertions int
	UnstagedDeletions  int
}

func (d DirtyDetails) TotalFiles() int {
	return d.StagedFiles + d.UnstagedFiles + d.Untracked
}

func (d DirtyDetails) String() string {
	var parts []string
	if d.StagedFiles > 0 {
		parts = append(parts, "staged:"+itoa(d.StagedFiles)+" +"+itoa(d.StagedInsertions)+"/-"+itoa(d.StagedDeletions))
	}
	if d.UnstagedFiles > 0 {
		parts = append(parts, "modified:"+itoa(d.UnstagedFiles)+" +"+itoa(d.UnstagedInsertions)+"/-"+itoa(d.UnstagedDeletions))
	}
	if d.Untracked > 0 {
		parts = append(parts, "untracked:"+itoa(d.Untracked))
	}
	return strings.Join(parts, " ")
}

type BranchInfo struct {
	Name           string
	IsCurrent      bool
	CommitCount    int
	LastCommitDate string
}

type RemoteInfo struct {
	Name   string
	URL    string
	IsMine bool
}

type RepoInfo struct {
	Path                 string
	Name                 string
	IsGitRepo            bool
	HasUserRemote        bool
	UserRemotes          []string
	AllRemotes           []RemoteInfo
	BranchesWithCommits  []BranchInfo
	TotalUserCommits     int
	LastCommitDate       string // Last commit by user
	LastRepoCommitDate   string // Last commit by anyone
	HasUncommittedChanges bool
	DirtyDetails         *DirtyDetails
	CurrentBranch        string
	DefaultBranch        string
	Ahead                int
	Behind               int
	StashCount           int
	IsFork               bool
	UpstreamURL          string
	Error                string
}

func IsGitRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

func isUserCommit(commit *object.Commit) bool {
	if userEmail == "" {
		return false
	}
	return strings.EqualFold(commit.Author.Email, userEmail)
}

func commitDateStr(commit *object.Commit) string {
	return commit.Author.When.Format("2006-01-02")
}

func AnalyzeRepo(path string, opts Options) RepoInfo {
	info := RepoInfo{
		Path: path,
		Name: filepath.Base(path),
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return info
	}
	info.IsGitRepo = true

	// Get remotes
	remotes, err := repo.Remotes()
	if err == nil {
		for _, remote := range remotes {
			cfg := remote.Config()
			url := ""
			if len(cfg.URLs) > 0 {
				url = cfg.URLs[0]
			}
			isMine := isUserRemote(url)
			info.AllRemotes = append(info.AllRemotes, RemoteInfo{
				Name:   cfg.Name,
				URL:    url,
				IsMine: isMine,
			})
			if isMine {
				info.UserRemotes = append(info.UserRemotes, cfg.Name)
				info.HasUserRemote = true
			}
		}
	}

	// Detect fork: has user remote AND non-user remote
	hasOther := false
	for _, r := range info.AllRemotes {
		if !r.IsMine {
			hasOther = true
			if info.UpstreamURL == "" {
				info.UpstreamURL = r.URL
			}
		}
	}
	info.IsFork = info.HasUserRemote && hasOther

	// Current branch
	head, err := repo.Head()
	if err == nil {
		if head.Name().IsBranch() {
			info.CurrentBranch = head.Name().Short()
		} else {
			info.CurrentBranch = "(detached)"
		}
	}

	// Default branch
	info.DefaultBranch = detectDefaultBranch(repo)

	// Working directory status and diff stats
	info.HasUncommittedChanges, info.DirtyDetails = getDirtyDetails(path)

	// Stash count
	info.StashCount = getStashCount(path)

	// Ahead/behind
	if head != nil && info.CurrentBranch != "(detached)" {
		branch, err := repo.Branch(info.CurrentBranch)
		if err == nil && branch.Remote != "" {
			remoteBranch := plumbing.NewRemoteReferenceName(branch.Remote, branch.Name)
			remoteRef, err := repo.Reference(remoteBranch, true)
			if err == nil {
				ahead, behind := countAheadBehind(repo, head.Hash(), remoteRef.Hash())
				info.Ahead = ahead
				info.Behind = behind
			}
		}
	}

	// Walk commits
	userCount, lastUserDate, lastRepoDate := walkCommits(repo)
	info.TotalUserCommits = userCount
	info.LastCommitDate = lastUserDate
	info.LastRepoCommitDate = lastRepoDate

	// Branches with user commits (only in verbose mode)
	if opts.Verbose {
		info.BranchesWithCommits = getBranchesWithUserCommits(repo, info.CurrentBranch)
	}

	return info
}

// runGit runs a git command in the given directory and returns stdout or empty string on error
func runGit(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// parseShortstat parses `git diff --shortstat` output into (insertions, deletions)
func parseShortstat(output string) (int, int) {
	insertions, deletions := 0, 0
	// Format: " 3 files changed, 10 insertions(+), 5 deletions(-)"
	if m := insertionRe.FindStringSubmatch(output); m != nil {
		insertions, _ = strconv.Atoi(m[1])
	}
	if m := deletionRe.FindStringSubmatch(output); m != nil {
		deletions, _ = strconv.Atoi(m[1])
	}
	return insertions, deletions
}

// getDirtyDetails gets working directory status using git commands
func getDirtyDetails(dir string) (bool, *DirtyDetails) {
	porcelain := runGit(dir, "status", "--porcelain")
	if porcelain == "" {
		return false, nil
	}

	details := &DirtyDetails{}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			details.Untracked++
		} else {
			if x != ' ' && x != '?' {
				details.StagedFiles++
			}
			if y != ' ' && y != '?' {
				details.UnstagedFiles++
			}
		}
	}

	// Get staged diff stats
	stagedStat := runGit(dir, "diff", "--cached", "--shortstat")
	if stagedStat != "" {
		details.StagedInsertions, details.StagedDeletions = parseShortstat(stagedStat)
	}

	// Get unstaged diff stats
	unstagedStat := runGit(dir, "diff", "--shortstat")
	if unstagedStat != "" {
		details.UnstagedInsertions, details.UnstagedDeletions = parseShortstat(unstagedStat)
	}

	hasChanges := details.TotalFiles() > 0
	if hasChanges {
		return true, details
	}
	return false, nil
}

// getStashCount returns the number of stashes in the repo
func getStashCount(dir string) int {
	output := runGit(dir, "stash", "list")
	if output == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(output), "\n"))
}

func detectDefaultBranch(repo *git.Repository) string {
	// Try origin/HEAD
	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err == nil {
		name := ref.Name().Short()
		return strings.TrimPrefix(name, "origin/")
	}

	// Fallback to common names
	for _, name := range []string{"main", "master"} {
		_, err := repo.Reference(plumbing.NewBranchReferenceName(name), false)
		if err == nil {
			return name
		}
	}
	return ""
}

func countAheadBehind(repo *git.Repository, local, remote plumbing.Hash) (ahead, behind int) {
	// Simple implementation: count commits reachable from local but not remote
	localCommits := make(map[plumbing.Hash]bool)
	remoteCommits := make(map[plumbing.Hash]bool)

	iter, _ := repo.Log(&git.LogOptions{From: local})
	if iter != nil {
		iter.ForEach(func(c *object.Commit) error {
			localCommits[c.Hash] = true
			return nil
		})
	}

	iter, _ = repo.Log(&git.LogOptions{From: remote})
	if iter != nil {
		iter.ForEach(func(c *object.Commit) error {
			remoteCommits[c.Hash] = true
			return nil
		})
	}

	for h := range localCommits {
		if !remoteCommits[h] {
			ahead++
		}
	}
	for h := range remoteCommits {
		if !localCommits[h] {
			behind++
		}
	}
	return
}

func walkCommits(repo *git.Repository) (userCount int, lastUserDate, lastRepoDate string) {
	head, err := repo.Head()
	if err != nil {
		return
	}

	iter, err := repo.Log(&git.LogOptions{From: head.Hash(), All: true})
	if err != nil {
		return
	}

	seen := make(map[plumbing.Hash]bool)
	iter.ForEach(func(c *object.Commit) error {
		if seen[c.Hash] {
			return nil
		}
		seen[c.Hash] = true

		if lastRepoDate == "" {
			lastRepoDate = commitDateStr(c)
		}

		if isUserCommit(c) {
			userCount++
			if lastUserDate == "" {
				lastUserDate = commitDateStr(c)
			}
		}
		return nil
	})
	return
}

func getBranchesWithUserCommits(repo *git.Repository, currentBranch string) []BranchInfo {
	var branches []BranchInfo

	refs, err := repo.References()
	if err != nil {
		return branches
	}

	refs.ForEach(func(ref *plumbing.Reference) error {
		if !ref.Name().IsBranch() {
			return nil
		}
		branchName := ref.Name().Short()

		iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			return nil
		}

		userCount := 0
		var lastDate string
		iter.ForEach(func(c *object.Commit) error {
			if isUserCommit(c) {
				userCount++
				if lastDate == "" {
					lastDate = commitDateStr(c)
				}
			}
			return nil
		})

		if userCount > 0 {
			branches = append(branches, BranchInfo{
				Name:           branchName,
				IsCurrent:      branchName == currentBranch,
				CommitCount:    userCount,
				LastCommitDate: lastDate,
			})
		}
		return nil
	})

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].CommitCount > branches[j].CommitCount
	})

	return branches
}

func AnalyzeDirectory(path string, opts Options, showProgress bool) []RepoInfo {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(path, e.Name()))
		}
	}

	results := make([]RepoInfo, len(dirs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // limit concurrency

	for i, dir := range dirs {
		wg.Add(1)
		go func(idx int, d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = AnalyzeRepo(d, opts)
		}(i, dir)
	}

	if showProgress {
		// Simple progress indicator
		go func() {
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	return results
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
