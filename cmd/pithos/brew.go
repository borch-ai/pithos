package main

import (
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	brewTheme  string
	brewStyle  string
	brewOutput string
)

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Generates the manuscript and stanza illustrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := pipeline.BrewOptions{
			OutputDir: brewOutput,
			Theme:     brewTheme,
			Style:     brewStyle,
		}
		return pipeline.Brew(cmd.Context(), opts)
	},
}

func init() {
	brewCmd.Flags().StringVar(&brewTheme, "theme", "", "Theme for the manuscript (optional override)")
	brewCmd.Flags().StringVar(&brewStyle, "style", "", "Style reference for images (optional override)")
	brewCmd.Flags().StringVar(&brewOutput, "output", "book", "Output directory path")
	rootCmd.AddCommand(brewCmd)
}
