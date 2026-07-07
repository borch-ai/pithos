package main

import (
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	characterOutput string
)

var characterCmd = &cobra.Command{
	Use:   "character",
	Short: "Generates or regenerates the visual character seed portrait using the configured image model",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := pipeline.CharacterOptions{
			OutputDir: characterOutput,
			DryRun:    rootDryRun,
		}
		return pipeline.GenerateCharacterSeed(cmd.Context(), opts)
	},
}

func init() {
	characterCmd.Flags().StringVar(&characterOutput, "output", "book", "Output directory path")
	rootCmd.AddCommand(characterCmd)
}
