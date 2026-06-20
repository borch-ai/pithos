package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	initiateOutput       string
	initiateTheme        string
	initiateStyle        string
	initiateFormat       string
	initiatePages        int
	initiateTrimSize     string
	initiateNoBrainstorm bool
)

var initiateCmd = &cobra.Command{
	Use:   "initiate",
	Short: "Scaffolds a new book project directory and manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := initiateOutput
		if !cmd.Flags().Changed("output") {
			// User did not provide --output, resolve unique output dir under books/book
			resolvedBase := pipeline.ResolveBookPath(outputDir)
			outputDir = pipeline.GetUniqueOutputDir(resolvedBase)
		} else {
			// User explicitly provided --output. If it exists, ask for confirmation
			resolvedDir := pipeline.ResolveBookPath(outputDir)
			if _, err := os.Stat(resolvedDir); err == nil {
				confirm, err := pipeline.ConfirmOverwrite(os.Stdin, os.Stdout, resolvedDir)
				if err != nil {
					return err
				}
				if !confirm {
					return fmt.Errorf("initiation cancelled: directory %s already exists and manifest overwrite was declined", resolvedDir)
				}
			}
		}

		opts := pipeline.InitiateOptions{
			OutputDir:       outputDir,
			Theme:           initiateTheme,
			Style:           initiateStyle,
			Format:          initiateFormat,
			TargetPageCount: initiatePages,
			TrimSize:        initiateTrimSize,
			NoBrainstorm:    initiateNoBrainstorm,
			Context:         cmd.Context(),
		}
		m, err := pipeline.Initiate(opts)
		if err != nil {
			return err
		}
		fmt.Printf("Successfully initiated book structure in %s\n", filepath.Dir(m.FilePath()))
		return nil
	},
}

func init() {
	initiateCmd.Flags().StringVar(&initiateOutput, "output", "book", "Output directory path")
	initiateCmd.Flags().StringVar(&initiateTheme, "theme", "", "Theme of the book")
	initiateCmd.Flags().StringVar(&initiateStyle, "style", "", "Style reference for illustrations (supporting Midjourney sref format)")
	initiateCmd.Flags().StringVar(&initiateFormat, "format", "paperback", "KDP print format (paperback or hardcover)")
	initiateCmd.Flags().IntVar(&initiatePages, "pages", 15, "Target page count of the book")
	initiateCmd.Flags().StringVar(&initiateTrimSize, "trim-size", "8.5x8.5", "Trim size of the book (e.g. 6x9, 8.5x8.5)")
	initiateCmd.Flags().BoolVar(&initiateNoBrainstorm, "no-brainstorm", false, "Disable automated visual style and character profile brainstorming")
	rootCmd.AddCommand(initiateCmd)
}
