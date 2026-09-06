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
	BotBranch            string
	MyBranch             string
	BotRemoteURL         string
	TargetRemote         string
	TargetRemoteExplicit bool
	BaseBranch           string
	BaseRepo             string
}

var cfg Config

var botURL string

var rootCmd = &cobra.Command{
	Use:   "sync-bot [bot-url]",
	Short: "Sync Chai Bot branch, rewrite author identity, and push to target remote",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg.TargetRemoteExplicit = cmd.Flags().Changed("target-remote")
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
	rootCmd.Flags().StringVarP(&cfg.TargetRemote, "target-remote", "t", "", "Remote to push to (auto-detected from parent repo when omitted)")
	rootCmd.Flags().StringVarP(&cfg.BaseBranch, "base", "a", "", "Default branch of the parent repo (auto-detected from bot fork)")
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
		if cfg.BaseRepo == "" {
			cfg.BaseRepo = resolved.BaseRepo
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
	if err := resolveParentRepo(&cfg); err != nil {
		return err
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}

	fmt.Printf("Bot remote: %s\n", cfg.BotRemoteURL)
	fmt.Printf("Bot branch: %s\n", cfg.BotBranch)
	if cfg.BaseRepo != "" {
		fmt.Printf("Parent repo: %s (default branch: %s)\n", cfg.BaseRepo, cfg.BaseBranch)
	} else {
		fmt.Printf("Base branch: %s\n", cfg.BaseBranch)
	}
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
	ensureCleanGitState()

	botRemoteName, err := ensureBotRemote(cfg.BotRemoteURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fetching branch '%s' from '%s'...\n", cfg.BotBranch, botRemoteName)
	if _, err := runCmd("git", "fetch", botRemoteName, cfg.BotBranch); err != nil {
		log.Fatalf("Failed to fetch bot branch: %v", err)
	}

	botRef := fmt.Sprintf("%s/%s", botRemoteName, cfg.BotBranch)

	baseRemote, err := resolveBaseRemote(cfg)
	if err != nil {
		log.Fatal(err)
	}

	stackView := loadStackViewOptional(baseRemote, botRef, cfg.BaseBranch)

	fmt.Printf("Checking out branch '%s'...\n", cfg.MyBranch)
	if _, err := runCmd("git", "checkout", "-B", cfg.MyBranch, botRef); err != nil {
		log.Fatalf("Failed to checkout branch: %v", err)
	}
	if _, err := runCmd("git", "reset", "--hard", botRef); err != nil {
		log.Fatalf("Failed to reset branch to bot remote: %v", err)
	}

	// 4. Rebase onto the branch the bot forked from
	rebaseExec := "git commit --amend --reset-author --no-edit"

	baseBranch, forkPoint, err := rebaseBotOntoBase(baseRemote, cfg.BaseBranch, botRef, rebaseExec, stackView)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Rebased onto %s/%s (forked at %s)\n", baseRemote, baseBranch, forkPoint)

	stackLayer, onStack := findStackLayer(stackView, baseBranch)
	if !onStack && baseBranch != cfg.BaseBranch {
		onStack = true
		stackLayer = stackBranch{Name: baseBranch}
	}
	targetRemote, err := resolvePushRemote(cfg, baseRemote, onStack, stackLayer)
	if err != nil {
		log.Fatal(err)
	}

	if onStack {
		fmt.Printf("Detected gh stack layer: %s (remote: %s)\n", stackLayer.Name, targetRemote)
	} else {
		fmt.Printf("Single-branch mode (remote: %s)\n", targetRemote)
	}

	// 5. Push to target remote
	fmt.Printf("Pushing '%s' to remote '%s'...\n", cfg.MyBranch, targetRemote)
	if _, err := runCmd("git", "push", "-u", targetRemote, cfg.MyBranch, "--force-with-lease"); err != nil {
		log.Fatalf("Failed to push branch: %v", err)
	}

	// 6. Link into gh stack when the bot branch forked from a stack layer
	if onStack {
		linkArgs := stackLinkArgs(stackView, cfg.MyBranch)
		fmt.Printf("\nLinking '%s' on top of stack layer %s...\n", cfg.MyBranch, stackLinkRef(stackLayer))
		ghArgs := append([]string{"stack", "link", "--remote", targetRemote}, linkArgs...)
		if _, err := runCmd("gh", ghArgs...); err != nil {
			log.Fatalf("Failed to link branch into gh stack: %v", err)
		}

		if err := submitStack(targetRemote, stackLayer.Name, cfg.MyBranch); err != nil {
			log.Printf("Warning: stack submit failed (branch was pushed and linked): %v", err)
		} else {
			fmt.Println("Successfully linked and submitted stacked PR!")
		}
	}

	fmt.Println("\nOperation completed successfully.")
}
