package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/powerword/pkg/gitutil"
	"github.com/spf13/cobra"
)

var (
	checkpointBook string
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Manage local workspace git checkpoints",
	Long:  `checkpoint allows you to list and restore from local Git checkpoints created automatically by the Pithos pipeline.`,
}

var checkpointListCmd = &cobra.Command{
	Use:   "list",
	Short: "List past checkpoint milestones for the workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := pipeline.ResolveBookPath(checkpointBook)

		if _, err := exec.LookPath("git"); err != nil {
			return errors.New("git command not found in PATH")
		}

		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory %s is not inside a git repository", dir)
			}
			return fmt.Errorf("failed to check git directory: %w", err)
		}

		// Run git log --oneline
		logOut, err := gitutil.RunGitCommand(cmd.Context(), dir, "log", "--oneline")
		if err != nil {
			return fmt.Errorf("failed to retrieve checkpoint log: %w", err)
		}

		fmt.Println("Checkpoint History:")
		fmt.Println(strings.TrimSpace(logOut))
		return nil
	},
}

var checkpointRestoreCmd = &cobra.Command{
	Use:   "restore [commit-hash]",
	Short: "Restore workspace to a specific checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		commitHash := args[0]
		dir := pipeline.ResolveBookPath(checkpointBook)

		if _, err := exec.LookPath("git"); err != nil {
			return errors.New("git command not found in PATH")
		}

		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory %s is not inside a git repository", dir)
			}
			return fmt.Errorf("failed to check git directory: %w", err)
		}

		// Perform reset hard to the commit
		if err := gitutil.ResetHard(cmd.Context(), dir, commitHash); err != nil {
			return fmt.Errorf("failed to reset workspace to %s: %w", commitHash, err)
		}

		// Clean untracked files
		if err := gitutil.Clean(cmd.Context(), dir); err != nil {
			return fmt.Errorf("failed to clean workspace: %w", err)
		}

		// Find the relative path for display
		relPath := filepath.Base(dir)
		fmt.Printf("Successfully restored workspace %s to checkpoint %s\n", relPath, commitHash)
		return nil
	},
}

func init() {
	checkpointCmd.PersistentFlags().StringVar(&checkpointBook, "book", "book", "Book workspace directory path")
	checkpointCmd.AddCommand(checkpointListCmd)
	checkpointCmd.AddCommand(checkpointRestoreCmd)
	rootCmd.AddCommand(checkpointCmd)
}
