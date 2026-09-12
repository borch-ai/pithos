package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	previewWatch bool
	previewDir   string
)

var previewCmd = &cobra.Command{
	Use:   "preview [book]",
	Short: "Generates the web preview and optionally monitors changes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}

		target, _, err := resolvePositionalOrDir(cmd, previewDir, args)
		if err != nil {
			return err
		}
		bookDir := pipeline.ResolveBookPath(target)

		manifestPath := filepath.Join(bookDir, "manifest.json")
		m, err := manifest.LoadManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		// 1. Generate preview assets
		if err := pipeline.GenerateWebPreview(bookDir, m); err != nil {
			return err
		}

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// 2. Open browser preview
		if !IsHeadless() {
			previewPath := filepath.Join(bookDir, "web_preview", "preview.html")
			if previewWatch {
				previewPath += "?watch=1"
			}
			pipeline.TriggerBrowserOpen(ctx, previewPath)
		}

		// 3. Start live workspace watcher if requested
		if previewWatch {
			opts := pipeline.WatchOptions{
				BookDir:    bookDir,
				ConfigFile: cfgFile,
				DryRun:     rootDryRun,
			}
			return pipeline.WatchWorkspace(ctx, opts)
		}

		return nil
	},
}

func init() {
	previewCmd.Flags().BoolVar(&previewWatch, "watch", false, "Watch the book directory for changes and hot-reload")
	previewCmd.Flags().StringVarP(&previewDir, "dir", "d", "", "Workspace directory path")
	rootCmd.AddCommand(previewCmd)
}
