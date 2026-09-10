package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cleanOrphans     bool
	cleanResetFailed bool
	cleanAll         bool
	cleanDir         string
)

var cleanCmd = &cobra.Command{
	Use:   "clean [book]",
	Short: "Clean orphans and reset failed page states in a book workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}
		if config.Cfg.WorkspacesRoot == "" {
			return fmt.Errorf("workspaces root is empty in config")
		}

		if !cleanOrphans && !cleanResetFailed && !cleanAll {
			return fmt.Errorf("must specify at least one clean action (--orphans, --reset-failed, --all)")
		}
		if cleanAll && cleanResetFailed {
			return fmt.Errorf("--all and --reset-failed are mutually exclusive")
		}

		target, err := resolvePositionalOrDir(cmd, cleanDir, args)
		if err != nil {
			return err
		}

		workspaceRoot := config.Cfg.WorkspacesRoot
		bookName := target
		if strings.ContainsAny(target, "/\\") || filepath.IsAbs(target) {
			resolved := pipeline.ResolveBookPath(target)
			workspaceRoot = filepath.Dir(resolved)
			bookName = filepath.Base(resolved)
		}

		err = pipeline.CleanWorkspace(workspaceRoot, bookName, cleanOrphans, cleanResetFailed, cleanAll)
		if err != nil {
			return fmt.Errorf("failed to clean workspace %q: %w", bookName, err)
		}

		fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("Workspace %q cleaned successfully.", bookName)))
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanOrphans, "orphans", false, "Delete orphaned images not referenced in manifest")
	cleanCmd.Flags().BoolVar(&cleanResetFailed, "reset-failed", false, "Reset stuck or failed pages to pending")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Reset all pages to pending")
	cleanCmd.Flags().StringVarP(&cleanDir, "dir", "d", "", "Workspace directory path")
	rootCmd.AddCommand(cleanCmd)
}
