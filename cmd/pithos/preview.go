package main

import (
	"fmt"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	previewWatch bool
)

var previewCmd = &cobra.Command{
	Use:   "preview <book>",
	Short: "Generates the web preview and optionally monitors changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}

		bookName := args[0]
		bookDir := pipeline.ResolveBookPath(bookName)

		manifestPath := filepath.Join(bookDir, "manifest.json")
		m, err := manifest.LoadManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		// 1. Generate preview assets
		if err := pipeline.GenerateWebPreview(bookDir, m); err != nil {
			return err
		}

		// 2. Open browser preview
		previewPath := filepath.Join(bookDir, "web_preview", "preview.html")
		pipeline.TriggerBrowserOpen(cmd.Context(), previewPath)

		// 3. Start live workspace watcher if requested
		if previewWatch {
			opts := pipeline.WatchOptions{
				BookDir:    bookDir,
				ConfigFile: cfgFile,
				DryRun:     rootDryRun,
			}
			return pipeline.WatchWorkspace(cmd.Context(), opts)
		}

		return nil
	},
}

func init() {
	previewCmd.Flags().BoolVar(&previewWatch, "watch", false, "Watch the book directory for changes and hot-reload")
	rootCmd.AddCommand(previewCmd)
}
