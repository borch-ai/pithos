package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/spf13/cobra"
)

var (
	packDir    string
	packForce  bool
	packOutput string
)

var packCmd = &cobra.Command{
	Use:   "pack [book]",
	Short: "Package verified print-ready artifacts into a distribution archive",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}
		target, isPositional, err := resolvePositionalOrDir(cmd, packDir, args)
		if err != nil {
			return err
		}
		if isPositional && config.Cfg.WorkspacesRoot == "" {
			return fmt.Errorf("workspaces root is empty in config")
		}

		workspaceRoot, bookName := resolveWorkspaceAndBook(target, isPositional)
		workspaceDir := filepath.Join(workspaceRoot, bookName)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		opts := pipeline.PackOptions{
			WorkspaceDir: workspaceDir,
			OutputPath:   packOutput,
			Force:        packForce,
			DryRun:       rootDryRun,
		}

		bundle, err := pipeline.PackWorkspace(ctx, opts)
		if err != nil {
			return err
		}

		// Plain text formatting for non-TTY or headless execution
		out := cmd.OutOrStdout()
		if !isTTY() || IsHeadless() {
			_, _ = fmt.Fprintf(out, "pithos pack: Successfully packaged %s to %s\n", bookName, bundle.ArchivePath)
			keys := make([]string, 0, len(bundle.Checksums))
			for k := range bundle.Checksums {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				_, _ = fmt.Fprintf(out, "  %s: %s\n", k, bundle.Checksums[k])
			}
			return nil
		}

		// Styled Lipgloss box output for interactive terminal
		var sb strings.Builder
		title := ui.HeaderStyle.Render(fmt.Sprintf("PRINT-READY ARTIFACT PACKAGE: %s", bookName))
		sb.WriteString(title + "\n\n")

		sb.WriteString(ui.KeyStyle.Render("Archive Path: ") + ui.ValStyle.Render(bundle.ArchivePath) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Packaged At:  ") + ui.ValStyle.Render(bundle.PackagedAt) + "\n")

		absArchive := bundle.ArchivePath
		if !filepath.IsAbs(absArchive) {
			absArchive = filepath.Join(workspaceDir, absArchive)
		}
		if info, statErr := os.Stat(absArchive); statErr == nil {
			sizeKB := float64(info.Size()) / 1024.0
			sb.WriteString(ui.KeyStyle.Render("Archive Size: ") + ui.ValStyle.Render(fmt.Sprintf("%.1f KB", sizeKB)) + "\n")
		}
		sb.WriteString("\n")

		sb.WriteString(ui.HighlightStyle.Render("SHA-256 CHECKSUMS") + "\n")
		keys := make([]string, 0, len(bundle.Checksums))
		for k := range bundle.Checksums {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(ui.KeyStyle.Render(fmt.Sprintf("  %-15s ", k+":")) + ui.ValStyle.Render(bundle.Checksums[k]) + "\n")
		}

		box := ui.BoxStyle.Render(sb.String())
		_, _ = fmt.Fprintln(out, box)

		return nil
	},
}

func init() {
	packCmd.Flags().StringVarP(&packDir, "dir", "d", "", "Workspace directory path")
	packCmd.Flags().BoolVarP(&packForce, "force", "f", false, "Force packaging even if preflight checks failed or are incomplete")
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "Output path or directory for the distribution archive")
	rootCmd.AddCommand(packCmd)
}
