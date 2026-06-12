package main

import (
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	brewTheme       string
	brewStyle       string
	brewOutput      string
	brewConcurrency int
	brewReview      bool
)

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Generates the manuscript and stanza illustrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := pipeline.BrewOptions{
			OutputDir:   brewOutput,
			Theme:       brewTheme,
			Style:       brewStyle,
			Concurrency: brewConcurrency,
			Review:      brewReview,
		}
		return pipeline.Brew(cmd.Context(), opts)
	},
}

func init() {
	brewCmd.Flags().StringVar(&brewTheme, "theme", "", "Theme for the manuscript (optional override)")
	brewCmd.Flags().StringVar(&brewStyle, "style", "", "Style reference for images (optional override)")
	brewCmd.Flags().StringVar(&brewOutput, "output", "book", "Output directory path")
	brewCmd.Flags().IntVar(&brewConcurrency, "concurrency", 0, "Number of concurrent image generation workers (defaults to config or 1)")
	brewCmd.Flags().BoolVar(&brewReview, "review", false, "Export manuscript for local markdown review and pause execution")
	rootCmd.AddCommand(brewCmd)
}
