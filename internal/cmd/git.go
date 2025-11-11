package cmd

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git integration commands",
	Long: `Git integration commands for Crush.
These commands provide native git operations with AI assistance.`,
}

var gitCommitCmd = &cobra.Command{
	Use:   "commit [message]",
	Short: "Create a commit with AI-generated message",
	Long: `Create a git commit with an AI-generated commit message.
If no message is provided, Crush will analyze staged changes and generate an appropriate message.`,
	Example: `
# Generate AI commit message from staged changes
crush git commit

# Commit with custom message
crush git commit "Add new feature"

# Stage all changes and commit with AI message
crush git commit --all
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		message := strings.Join(args, " ")

		if all {
			// Stage all changes
			gitAdd := exec.Command("git", "add", ".")
			if output, err := gitAdd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to stage changes: %s\n%s", err, output)
			}
			fmt.Println("Staged all changes.")
		}

		// If message is provided, commit directly
		if message != "" {
			gitCommit := exec.Command("git", "commit", "-m", message)
			output, err := gitCommit.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to commit: %s\n%s", err, output)
			}
			fmt.Println(string(output))
			return nil
		}

		// Otherwise, generate AI commit message
		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if !app.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
		}

		prompt := "Analyze the staged git changes and generate a concise, descriptive commit message following conventional commits format. Only output the commit message, nothing else."

		// Run AI to generate commit message
		fmt.Println("Generating commit message...")
		err = app.RunNonInteractive(cmd.Context(), prompt, true)
		if err != nil {
			return err
		}

		return nil
	},
}

var gitDiffCmd = &cobra.Command{
	Use:   "diff [files...]",
	Short: "Show git diff with AI analysis",
	Long: `Show git diff for staged or unstaged changes.
Optionally request AI analysis of the changes.`,
	Example: `
# Show diff of staged changes
crush git diff

# Show diff of all changes (staged + unstaged)
crush git diff --all

# Show diff with AI analysis
crush git diff --analyze
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		analyze, _ := cmd.Flags().GetBool("analyze")
		staged, _ := cmd.Flags().GetBool("staged")

		var gitArgs []string
		if staged {
			gitArgs = append(gitArgs, "diff", "--staged")
		} else if all {
			gitArgs = append(gitArgs, "diff", "HEAD")
		} else {
			gitArgs = append(gitArgs, "diff")
		}

		if len(args) > 0 {
			gitArgs = append(gitArgs, args...)
		}

		gitDiff := exec.Command("git", gitArgs...)
		output, err := gitDiff.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get diff: %s\n%s", err, output)
		}

		if len(output) == 0 {
			fmt.Println("No changes to show.")
			return nil
		}

		fmt.Println(string(output))

		if analyze {
			app, err := setupApp(cmd)
			if err != nil {
				return err
			}
			defer app.Shutdown()

			if !app.Config().IsConfigured() {
				return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
			}

			prompt := fmt.Sprintf("Analyze these git changes and provide insights:\n\n%s", string(output))
			fmt.Println("\n--- AI Analysis ---")
			return app.RunNonInteractive(cmd.Context(), prompt, false)
		}

		return nil
	},
}

var gitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show git status",
	Long:  `Show git status of the repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitStatus := exec.Command("git", "status")
		output, err := gitStatus.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get status: %s\n%s", err, output)
		}
		fmt.Println(string(output))
		return nil
	},
}

var gitUndoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last commit (soft reset)",
	Long: `Undo the last git commit while keeping changes in working directory.
This performs a soft reset to HEAD~1.`,
	Example: `
# Undo last commit, keep changes staged
crush git undo

# Undo last commit, unstage changes
crush git undo --unstage
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		unstage, _ := cmd.Flags().GetBool("unstage")

		var resetType string
		if unstage {
			resetType = "--mixed"
		} else {
			resetType = "--soft"
		}

		gitReset := exec.Command("git", "reset", resetType, "HEAD~1")
		output, err := gitReset.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to undo commit: %s\n%s", err, output)
		}
		fmt.Println("Last commit undone successfully.")
		fmt.Println(string(output))
		return nil
	},
}

var gitLogCmd = &cobra.Command{
	Use:   "log [options]",
	Short: "Show git commit history",
	Long:  `Show git commit history with optional AI analysis.`,
	Example: `
# Show last 10 commits
crush git log

# Show last 20 commits
crush git log -n 20

# Analyze commit history
crush git log --analyze
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("n")
		analyze, _ := cmd.Flags().GetBool("analyze")

		gitArgs := []string{"log", fmt.Sprintf("-%d", n), "--oneline"}
		gitLog := exec.Command("git", gitArgs...)
		output, err := gitLog.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to get log: %s\n%s", err, output)
		}

		fmt.Println(string(output))

		if analyze {
			app, err := setupApp(cmd)
			if err != nil {
				return err
			}
			defer app.Shutdown()

			if !app.Config().IsConfigured() {
				return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
			}

			prompt := fmt.Sprintf("Analyze this git commit history and identify patterns, trends, or areas of concern:\n\n%s", string(output))
			fmt.Println("\n--- AI Analysis ---")
			return app.RunNonInteractive(cmd.Context(), prompt, false)
		}

		return nil
	},
}

var gitPushCmd = &cobra.Command{
	Use:   "push [remote] [branch]",
	Short: "Push commits to remote repository",
	Long:  `Push commits to remote repository with optional AI pre-push checks.`,
	Example: `
# Push to default remote and branch
crush git push

# Push to specific remote and branch
crush git push origin main

# Push with force (dangerous)
crush git push --force
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		var gitArgs []string
		gitArgs = append(gitArgs, "push")

		if force {
			gitArgs = append(gitArgs, "--force")
		}

		if len(args) > 0 {
			gitArgs = append(gitArgs, args...)
		}

		slog.Info("Pushing to remote", "args", gitArgs)

		gitPush := exec.Command("git", gitArgs...)
		output, err := gitPush.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to push: %s\n%s", err, output)
		}

		fmt.Println(string(output))
		return nil
	},
}

var gitPullCmd = &cobra.Command{
	Use:   "pull [remote] [branch]",
	Short: "Pull changes from remote repository",
	Long:  `Pull changes from remote repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var gitArgs []string
		gitArgs = append(gitArgs, "pull")

		if len(args) > 0 {
			gitArgs = append(gitArgs, args...)
		}

		gitPull := exec.Command("git", gitArgs...)
		output, err := gitPull.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to pull: %s\n%s", err, output)
		}

		fmt.Println(string(output))
		return nil
	},
}

func init() {
	// Commit flags
	gitCommitCmd.Flags().BoolP("all", "a", false, "Stage all changes before committing")

	// Diff flags
	gitDiffCmd.Flags().BoolP("all", "a", false, "Show all changes (staged + unstaged)")
	gitDiffCmd.Flags().Bool("staged", false, "Show only staged changes")
	gitDiffCmd.Flags().Bool("analyze", false, "AI analysis of the changes")

	// Undo flags
	gitUndoCmd.Flags().Bool("unstage", false, "Unstage changes after undo")

	// Log flags
	gitLogCmd.Flags().IntP("n", "n", 10, "Number of commits to show")
	gitLogCmd.Flags().Bool("analyze", false, "AI analysis of commit history")

	// Push flags
	gitPushCmd.Flags().BoolP("force", "f", false, "Force push (dangerous)")

	// Add subcommands to git command
	gitCmd.AddCommand(
		gitCommitCmd,
		gitDiffCmd,
		gitStatusCmd,
		gitUndoCmd,
		gitLogCmd,
		gitPushCmd,
		gitPullCmd,
	)
}
