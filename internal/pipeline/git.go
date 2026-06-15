package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/borch-ai/powerword/pkg/gitutil"
)

// Checkpoint creates a local git checkpoint (stage and commit) for the current workspace.
// If the git executable is not available in PATH, it logs a warning and proceeds.
func Checkpoint(ctx context.Context, dir string, message string) error {
	// 1. Checks if the system has git in PATH. If not, outputs a warning and returns nil.
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: 'git' executable not found in PATH; skipping checkpointing: %v\n", err)
		return nil
	}

	// 2. Resolves the directory path.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of %s: %w", dir, err)
	}

	// Ensure the directory exists
	if _, err := os.Stat(absDir); err != nil {
		if err := os.MkdirAll(absDir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s for git checkpoint: %w", absDir, err)
		}
	}

	// 3. Checks if the directory has its own git repository via checking for .git folder.
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// 4. If not, initializes a git repository using gitutil.Init.
		if err := gitutil.Init(ctx, absDir); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}
	}

	// Robustness check: ensure git user.name and user.email are configured so commit does not fail.
	if _, err := gitutil.RunGitCommand(ctx, absDir, "config", "user.name"); err != nil {
		_, _ = gitutil.RunGitCommand(ctx, absDir, "config", "user.name", "Pithos Agent")
	}
	if _, err := gitutil.RunGitCommand(ctx, absDir, "config", "user.email"); err != nil {
		_, _ = gitutil.RunGitCommand(ctx, absDir, "config", "user.email", "agent@borch.ai")
	}

	// 5. Adds modified/untracked files using gitutil.AddAll.
	if err := gitutil.AddAll(ctx, absDir); err != nil {
		return fmt.Errorf("failed to add files to git staging: %w", err)
	}

	// 6. Commits changes using gitutil.Commit with the milestone message.
	if err := gitutil.Commit(ctx, absDir, message); err != nil {
		// Ignore "nothing to commit" errors as they are non-fatal.
		if strings.Contains(err.Error(), "nothing to commit") || strings.Contains(err.Error(), "working tree clean") {
			return nil
		}
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}
