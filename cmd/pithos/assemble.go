package main

import (
	"context"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	assembleInput     string
	assembleFormat    string
	assembleBleed     bool
	assembleTrimSize  string
	assemblePaperType string
)

var assembleCmd = &cobra.Command{
	Use:   "assemble",
	Short: "Calculates book geometry and generates the layout manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := pipeline.AssembleOptions{
			InputDir:  assembleInput,
			Format:    assembleFormat,
			Bleed:     assembleBleed,
			TrimSize:  assembleTrimSize,
			PaperType: assemblePaperType,
		}
		return pipeline.Assemble(context.Background(), opts)
	},
}

func init() {
	assembleCmd.Flags().StringVar(&assembleInput, "input", "book", "Input directory path")
	assembleCmd.Flags().StringVar(&assembleFormat, "format", "paperback", "KDP print format (paperback or hardcover)")
	assembleCmd.Flags().BoolVar(&assembleBleed, "bleed", false, "Include bleed margins")
	assembleCmd.Flags().StringVar(&assembleTrimSize, "trim-size", "6x9", "Trim size of the book (e.g. 6x9, 5.5x8.5)")
	assembleCmd.Flags().StringVar(&assemblePaperType, "paper-type", "white", "Paper type of the book (white, cream, standard_color, premium_color)")
	rootCmd.AddCommand(assembleCmd)
}
