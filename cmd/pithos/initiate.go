package main

import (
	"fmt"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	initiateOutput string
	initiateTheme  string
	initiateStyle  string
	initiateFormat string
	initiatePages  int
)

var initiateCmd = &cobra.Command{
	Use:   "initiate",
	Short: "Scaffolds a new book project directory and manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := pipeline.InitiateOptions{
			OutputDir:       initiateOutput,
			Theme:           initiateTheme,
			Style:           initiateStyle,
			Format:          initiateFormat,
			TargetPageCount: initiatePages,
		}
		_, err := pipeline.Initiate(opts)
		if err != nil {
			return err
		}
		fmt.Printf("Successfully initiated book structure in %s\n", initiateOutput)
		return nil
	},
}

func init() {
	initiateCmd.Flags().StringVar(&initiateOutput, "output", "book", "Output directory path")
	initiateCmd.Flags().StringVar(&initiateTheme, "theme", "", "Theme of the book")
	initiateCmd.Flags().StringVar(&initiateStyle, "style", "", "Style reference for illustrations (supporting Midjourney sref format)")
	initiateCmd.Flags().StringVar(&initiateFormat, "format", "paperback", "KDP print format (paperback or hardcover)")
	initiateCmd.Flags().IntVar(&initiatePages, "pages", 15, "Target page count of the book")
	rootCmd.AddCommand(initiateCmd)
}
