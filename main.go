package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type Config struct {
	BotBranch    string
	MyBranch     string
	BotRemoteURL string
	TargetRemote string
	BaseBranch   string
	StackMode    bool
}

var cfg Config

var botURL string

var rootCmd = &cobra.Command{
	Use:   "sync-bot [bot-url]",
	Short: "Sync Chai Bot branch, rewrite author identity, and push to target remote",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := finalizeConfig(args); err != nil {
			log.Fatal(err)
		}
		syncAndRewrite(cfg)
	},
}

func init() {
	rootCmd.Flags().StringVar(&botURL, "url", "", "Chai Bot GitHub repo or branch URL (alternative to positional arg)")
	rootCmd.Flags().StringVarP(&cfg.BotBranch, "bot-branch", "b", "", "Name of the branch created by Chai Bot")
	rootCmd.Flags().StringVarP(&cfg.MyBranch, "my-branch", "m", "", "Name for your local/target branch (defaults to bot-branch)")
	rootCmd.Flags().StringVarP(&cfg.BotRemoteURL, "bot-remote", "r", "", "Git SSH/HTTPS URL for Chai Bot's repository")
	rootCmd.Flags().StringVarP(&cfg.TargetRemote, "target-remote", "t", "origin", "Target remote to push rewritten branch to")
	rootCmd.Flags().StringVarP(&cfg.BaseBranch, "base", "a", "", "Base branch to rebase against (defaults to upstream default branch)")
	rootCmd.Flags().BoolVarP(&cfg.StackMode, "stack", "s", false, "Enable gh stack integration for origin repo")
}

func finalizeConfig(args []string) error {
	rawURL := botURL
	if len(args) > 0 {
		rawURL = args[0]
	}

	if rawURL != "" {
		resolved, err := resolveBotURL(rawURL)
		if err != nil {
			return err
		}
		if cfg.BotRemoteURL == "" {
			cfg.BotRemoteURL = resolved.BotRemoteURL
		}
		if cfg.BotBranch == "" {
			cfg.BotBranch = resolved.BotBranch
		}
		if cfg.BaseBranch == "" {
			cfg.BaseBranch = resolved.BaseBranch
		}
	}

	if cfg.BotRemoteURL == "" || cfg.BotBranch == "" {
		return fmt.Errorf("provide a bot URL or both --bot-remote and --bot-branch")
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}
	if cfg.MyBranch == "" {
		cfg.MyBranch = cfg.BotBranch
	}

	fmt.Printf("Bot remote: %s\n", cfg.BotRemoteURL)
	fmt.Printf("Bot branch: %s\n", cfg.BotBranch)
	fmt.Printf("Base branch: %s\n", cfg.BaseBranch)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command '%s %s' failed: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func syncAndRewrite(cfg Config) {
	botRemoteName := "chai-bot-remote"

	// 1. Add bot remote if missing
	remotes, _ := runCmd("git", "remote")
	if !strings.Contains(remotes, botRemoteName) {
		fmt.Printf("Adding remote '%s'...\n", botRemoteName)
		if _, err := runCmd("git", "remote", "add", botRemoteName, cfg.BotRemoteURL); err != nil {
			log.Fatalf("Failed to add remote: %v", err)
		}
	}

	// 2. Fetch bot branch
	fmt.Printf("Fetching branch '%s' from '%s'...\n", cfg.BotBranch, botRemoteName)
	if _, err := runCmd("git", "fetch", botRemoteName, cfg.BotBranch); err != nil {
		log.Fatalf("Failed to fetch bot branch: %v", err)
	}

	// 3. Checkout local branch
	fmt.Printf("Checking out branch '%s'...\n", cfg.MyBranch)
	if _, err := runCmd("git", "checkout", "-B", cfg.MyBranch, fmt.Sprintf("%s/%s", botRemoteName, cfg.BotBranch)); err != nil {
		log.Fatalf("Failed to checkout branch: %v", err)
	}

	// 4. Fetch base branch & find merge base
	runCmd("git", "fetch", cfg.TargetRemote, cfg.BaseBranch)
	baseCommit, err := runCmd("git", "merge-base", fmt.Sprintf("%s/%s", cfg.TargetRemote, cfg.BaseBranch), "HEAD")
	if err != nil {
		log.Fatalf("Failed to calculate merge base: %v", err)
	}

	// 5. Rewrite commit author
	fmt.Println("Rewriting commit author to match local git user profile...")
	rebaseExec := "git commit --amend --reset-author --no-edit"
	if _, err := runCmd("git", "rebase", baseCommit, "--exec", rebaseExec); err != nil {
		log.Fatalf("Failed to rewrite commit authors during rebase: %v", err)
	}

	// 6. Push to target remote
	fmt.Printf("Pushing '%s' to remote '%s'...\n", cfg.MyBranch, cfg.TargetRemote)
	if _, err := runCmd("git", "push", "-u", cfg.TargetRemote, cfg.MyBranch, "--force-with-lease"); err != nil {
		log.Fatalf("Failed to push branch: %v", err)
	}

	// 7. Handle gh stack workflow
	if cfg.StackMode {
		fmt.Println("\nInitializing or updating gh stack on origin...")
		if _, err := runCmd("gh", "stack", "init", cfg.MyBranch); err != nil {
			fmt.Println("Stack already initialized or existing layer detected, syncing...")
		}

		if _, err := runCmd("gh", "stack", "submit", "--auto"); err != nil {
			log.Printf("Warning: gh stack submit failed or requires manual interaction: %v", err)
		} else {
			fmt.Println("Successfully submitted stacked PRs to origin repository!")
		}
	}

	fmt.Println("\nOperation completed successfully.")
}
