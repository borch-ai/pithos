package main

import (
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	characterOutput string
	characterDir    string
)

var characterCmd = &cobra.Command{
	Use:   "character",
	Short: "Generates or regenerates the visual character seed portrait using the configured image model",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := resolveDirectoryFlag(cmd, characterDir, characterOutput)
		opts := pipeline.CharacterOptions{
			OutputDir: outputDir,
			DryRun:    rootDryRun,
		}
		return pipeline.GenerateCharacterSeed(cmd.Context(), opts)
	},
}

func init() {
	characterCmd.Flags().StringVar(&characterOutput, "output", "book", "Output directory path")
	characterCmd.Flags().StringVarP(&characterDir, "dir", "d", "", "Workspace directory path (alias for --output)")
	rootCmd.AddCommand(characterCmd)
}
