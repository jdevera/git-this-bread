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

Valid keys: sshkey, email, user, ghuser

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

func init() {
	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(setCmd)

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
