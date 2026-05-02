package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdevera/git-this-bread/internal/identity"
)

var (
	fileFlag     string
	yesFlag      bool
	detachedFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "git-id",
	Short: "Manage git identity profiles",
	Long: `git-id (a git-this-bread tool)

Manage git/GitHub identity profiles stored in git config.

Profiles are stored as [identity.<name>] sections in your git config.
Each profile can have:
  - name:   Display name for git commits (optional, overrides user)
  - sshkey: Path to SSH private key (required for git-as)
  - email:  Git author/committer email (required for git-as)
  - user:   Git author/committer name (optional)
  - ghuser: GitHub username for gh-as (optional)

Examples:
  git-id                    # List all profiles
  git-id add personal       # Create a new profile interactively
  git-id show personal      # Show profile details
  git-id set personal email me@example.com
  git-id remove personal    # Delete a profile`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCmd.RunE(cmd, args)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all identity profiles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := identity.List()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Println("No identity profiles configured.")
			fmt.Println("Use 'git-id add <name>' to create one.")
			return nil
		}

		for _, name := range names {
			profile, err := identity.Get(name)
			if err != nil {
				fmt.Printf("  %s (error reading)\n", name)
				continue
			}

			// Check GitHub auth status
			status := identity.GetGHAuthStatus(profile.GHUser)
			var ghStatus string
			if profile.GHUser == "" {
				ghStatus = "(gh: not configured)"
			} else if status.Authenticated {
				ghStatus = fmt.Sprintf("(gh: %s ✓)", profile.GHUser)
			} else {
				ghStatus = fmt.Sprintf("(gh: %s ⚠)", profile.GHUser)
			}

			fmt.Printf("  %s: %s %s\n", name, profile.Email, ghStatus)
		}

		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <profile>",
	Short: "Show profile details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		profile, err := identity.Get(name)
		if err != nil {
			return err
		}

		// Get source file
		source, _ := identity.GetSourceFile(name)

		fmt.Printf("Profile: %s\n", profile.Name)
		if source != "" {
			fmt.Printf("Source:  %s\n", source)
		}
		fmt.Println()

		if profile.DisplayName != "" {
			fmt.Printf("  name:   %s\n", profile.DisplayName)
		} else {
			fmt.Println("  name:   (not set)")
		}

		if profile.SSHKey != "" {
			// Validate SSH key
			sshStatus := "✓"
			if err := identity.ValidateSSHKey(profile.SSHKey); err != nil {
				sshStatus = "⚠ " + err.Error()
			}
			fmt.Printf("  sshkey: %s %s\n", profile.SSHKey, sshStatus)
		} else {
			fmt.Println("  sshkey: (not set)")
		}

		if profile.Email != "" {
			fmt.Printf("  email:  %s\n", profile.Email)
		} else {
			fmt.Println("  email:  (not set)")
		}

		if profile.User != "" {
			fmt.Printf("  user:   %s\n", profile.User)
		} else {
			fmt.Println("  user:   (not set)")
		}

		if profile.GHUser != "" {
			status := identity.GetGHAuthStatus(profile.GHUser)
			var ghStatus string
			if status.Authenticated {
				ghStatus = "✓ authenticated"
			} else {
				ghStatus = "⚠ " + status.Message
			}
			fmt.Printf("  ghuser: %s %s\n", profile.GHUser, ghStatus)
		} else {
			fmt.Println("  ghuser: (not set)")
		}

		// Only print useCustomAgent when explicitly disabled — the default-true
		// case isn't worth the line.
		if !profile.UseCustomAgent {
			fmt.Println("  usecustomagent: false ⚠ profile opts out of git-as's per-profile sub-agent")
		}

		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <profile>",
	Short: "Create a new identity profile interactively",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if profile already exists
		if _, err := identity.Get(name); err == nil {
			return fmt.Errorf("profile %q already exists. Use 'git-id set' to modify it", name)
		}

		reader := bufio.NewReader(os.Stdin)
		profile := &identity.Profile{Name: name}

		fmt.Printf("Creating profile: %s\n\n", name)

		// SSH Key (required)
		fmt.Print("SSH key path (required): ")
		sshkey, _ := reader.ReadString('\n')
		sshkey = strings.TrimSpace(sshkey)
		if sshkey == "" {
			return fmt.Errorf("SSH key path is required")
		}
		if err := identity.ValidateSSHKey(sshkey); err != nil {
			return err
		}
		profile.SSHKey = sshkey

		// Email (required)
		fmt.Print("Email (required): ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email is required")
		}
		profile.Email = email

		// Display name (optional)
		fmt.Print("Display name for commits (optional): ")
		displayName, _ := reader.ReadString('\n')
		displayName = strings.TrimSpace(displayName)
		profile.DisplayName = displayName

		// User name (optional)
		fmt.Print("User name (optional): ")
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)
		profile.User = user

		// GitHub username (optional)
		fmt.Print("GitHub username (optional): ")
		ghuser, _ := reader.ReadString('\n')
		ghuser = strings.TrimSpace(ghuser)
		profile.GHUser = ghuser

		// Save the profile
		opts := identity.SetOptions{
			File:     fileFlag,
			Yes:      yesFlag,
			Detached: detachedFlag,
		}
		targetFile, err := identity.Set(profile, opts)
		if err != nil {
			return err
		}

		fmt.Printf("\nProfile '%s' saved to %s\n", name, targetFile)

		// Show warnings for GitHub auth if needed
		if ghuser != "" {
			status := identity.GetGHAuthStatus(ghuser)
			if !status.Authenticated {
				fmt.Printf("\n⚠ GitHub user '%s' is not authenticated.\n", ghuser)
				fmt.Printf("  Run: gh auth login\n")
			}
		}

		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <profile>",
	Short: "Delete an identity profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Verify profile exists
		if _, err := identity.Get(name); err != nil {
			return err
		}

		if err := identity.Remove(name); err != nil {
			return err
		}

		fmt.Printf("Profile '%s' removed.\n", name)
		return nil
	},
}

var setCmd = &cobra.Command{
	Use:   "set <profile> <key> <value>",
	Short: "Set a profile field",
	Long: `Set a single field on an existing profile.

Valid keys: name, sshkey, email, user, ghuser

Examples:
  git-id set personal email newemail@example.com
  git-id set work sshkey ~/.ssh/id_work`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		key := args[1]
		value := args[2]

		// Validate SSH key if setting sshkey
		if key == "sshkey" {
			if err := identity.ValidateSSHKey(value); err != nil {
				return err
			}
		}

		opts := identity.SetOptions{
			File:     fileFlag,
			Yes:      yesFlag,
			Detached: detachedFlag,
		}

		targetFile, err := identity.SetField(name, key, value, opts)
		if err != nil {
			return err
		}

		fmt.Printf("Set %s.%s = %s in %s\n", name, key, value, targetFile)

		// Show warning if setting ghuser that isn't authenticated
		if key == "ghuser" {
			status := identity.GetGHAuthStatus(value)
			if !status.Authenticated {
				fmt.Printf("\n⚠ GitHub user '%s' is not authenticated.\n", value)
				fmt.Printf("  Run: gh auth login\n")
			}
		}

		return nil
	},
}

var agentAllFlag bool

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage per-profile ssh sub-agents",
	Long: `Inspect and control the per-profile ssh sub-agents that git-as uses
to isolate identities. By default git-as spawns one sub-agent per profile,
loaded with that profile's key only; setting 'usecustomagent = false' on
a profile disables the feature for it.`,
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sub-agents currently tracked under the cache dir",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, err := identity.ListAgents()
		if err != nil {
			return err
		}
		if len(agents) == 0 {
			fmt.Println("No sub-agents found.")
			return nil
		}
		for _, a := range agents {
			alive := "dead"
			keyCount := "?"
			if a.IsAlive() {
				alive = "alive"
				if keys, err := a.LoadedKeys(); err == nil {
					keyCount = fmt.Sprintf("%d", len(keys))
				}
			}
			pid := a.PID()
			fmt.Printf("  %s\t%s\tpid=%d\tkeys=%s\tsocket=%s\n",
				a.Profile.Name, alive, pid, keyCount, a.Socket)
		}
		return nil
	},
}

var agentKillCmd = &cobra.Command{
	Use:   "kill [<profile>]",
	Short: "Terminate a profile's sub-agent (or all with --all)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentAllFlag {
			if len(args) > 0 {
				return fmt.Errorf("--all is incompatible with a profile argument")
			}
			agents, err := identity.ListAgents()
			if err != nil {
				return err
			}
			for _, a := range agents {
				if err := a.Kill(); err != nil {
					fmt.Fprintf(os.Stderr, "kill %s: %v\n", a.Profile.Name, err)
					continue
				}
				fmt.Printf("killed %s\n", a.Profile.Name)
			}
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("provide a profile name or use --all")
		}
		profileName := args[0]
		// Don't go through Ensure here — that would spawn a fresh agent for a
		// profile we're trying to kill. Walk the cache dir directly.
		agents, err := identity.ListAgents()
		if err != nil {
			return err
		}
		for _, a := range agents {
			if a.Profile.Name == profileName {
				if err := a.Kill(); err != nil {
					return err
				}
				fmt.Printf("killed %s\n", profileName)
				return nil
			}
		}
		fmt.Printf("no sub-agent found for %s\n", profileName)
		return nil
	},
}

var agentReloadCmd = &cobra.Command{
	Use:   "reload <profile>",
	Short: "Reload the profile's key into its sub-agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, err := identity.Get(args[0])
		if err != nil {
			return err
		}
		if !profile.UseCustomAgent {
			return fmt.Errorf("profile %q has usecustomagent=false; nothing to reload", profile.Name)
		}
		sa, err := identity.Ensure(profile)
		if err != nil {
			return err
		}
		if err := sa.LoadKey(); err != nil {
			return err
		}
		fmt.Printf("reloaded key for %s\n", profile.Name)
		return nil
	},
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(agentCmd)

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentKillCmd)
	agentCmd.AddCommand(agentReloadCmd)
	agentKillCmd.Flags().BoolVar(&agentAllFlag, "all", false, "Kill all sub-agents")

	// Global flags for write operations
	for _, cmd := range []*cobra.Command{addCmd, setCmd} {
		cmd.Flags().StringVar(&fileFlag, "file", "", "Write to specific config file")
		cmd.Flags().BoolVar(&yesFlag, "yes", false, "Auto-accept multi-file conflict prompt")
		cmd.Flags().BoolVar(&detachedFlag, "detached", false, "Skip effectiveness check")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
