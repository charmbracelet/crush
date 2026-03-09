package orchestra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorktreeManager manages git worktrees for agent isolation.
type WorktreeManager struct {
	basePath string
	repoRoot string
}

// NewWorktreeManager creates a new worktree manager.
func NewWorktreeManager(repoRoot, basePath string) *WorktreeManager {
	return &WorktreeManager{
		basePath: basePath,
		repoRoot: repoRoot,
	}
}

// Create creates a new worktree for a task and agent.
func (w *WorktreeManager) Create(taskID, agentName string) (*Worktree, error) {
	branchName := fmt.Sprintf("task/%s/%s", taskID, agentName)
	worktreePath := filepath.Join(w.basePath, taskID, agentName)

	// Ensure the base directory exists
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create the branch if it doesn't exist
	if err := w.createBranch(branchName); err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	// Create the worktree
	if err := w.git("worktree", "add", "-b", branchName, worktreePath, "main"); err != nil {
		// Try without -b if branch already exists
		if err := w.git("worktree", "add", worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	worktree := &Worktree{
		Path:      worktreePath,
		Branch:    branchName,
		AgentName: agentName,
		TaskID:    taskID,
		Created:   time.Now(),
		Status:    "active",
	}

	return worktree, nil
}

// Remove removes a worktree.
func (w *WorktreeManager) Remove(worktreePath string) error {
	// Remove the worktree
	if err := w.git("worktree", "remove", worktreePath, "--force"); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// List lists all worktrees.
func (w *WorktreeManager) List() ([]Worktree, error) {
	output, err := w.gitOutput("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current *Worktree

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			if current != nil {
				worktrees = append(worktrees, *current)
			}
			current = &Worktree{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			current.Branch = strings.TrimPrefix(line, "branch ")
		}
	}

	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// Commit creates a commit in the worktree.
func (w *WorktreeManager) Commit(worktreePath, message string) error {
	// Stage all changes
	if err := w.gitIn(worktreePath, "add", "-A"); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Create the commit
	if err := w.gitIn(worktreePath, "commit", "-m", message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// Push pushes the worktree's branch to remote.
func (w *WorktreeManager) Push(worktreePath string) error {
	// Get the current branch name
	branch, err := w.gitOutputIn(worktreePath, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	branch = strings.TrimSpace(branch)

	// Push to remote
	if err := w.gitIn(worktreePath, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}

// Merge merges a worktree's branch into main.
func (w *WorktreeManager) Merge(worktreePath string) error {
	// Get the current branch name
	branch, err := w.gitOutputIn(worktreePath, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	branch = strings.TrimSpace(branch)

	// Switch to main and merge
	if err := w.git("checkout", "main"); err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	if err := w.git("merge", branch, "--no-ff", "-m", fmt.Sprintf("Merge %s", branch)); err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	return nil
}

// GetStatus returns the git status of a worktree.
func (w *WorktreeManager) GetStatus(worktreePath string) (string, error) {
	return w.gitOutputIn(worktreePath, "status", "--short")
}

// GetDiff returns the diff of a worktree.
func (w *WorktreeManager) GetDiff(worktreePath string) (string, error) {
	return w.gitOutputIn(worktreePath, "diff", "main...HEAD")
}

func (w *WorktreeManager) createBranch(name string) error {
	// Check if branch exists
	_, err := w.gitOutput("rev-parse", "--verify", name)
	if err == nil {
		// Branch exists
		return nil
	}

	// Create the branch
	return w.git("branch", name)
}

func (w *WorktreeManager) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (w *WorktreeManager) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.repoRoot
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", err, string(ee.Stderr))
		}
		return "", err
	}
	return string(output), nil
}

func (w *WorktreeManager) gitIn(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (w *WorktreeManager) gitOutputIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", err, string(ee.Stderr))
		}
		return "", err
	}
	return string(output), nil
}
