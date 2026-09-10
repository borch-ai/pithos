package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	initiateOutput       string
	initiateDir          string
	initiateTitle        string
	initiateAuthor       string
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
		if initiateTheme == "" {
			pagesStr := strconv.Itoa(initiatePages)
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Theme").
						Description("What is the parodic theme of the book?").
						Placeholder("e.g. existential dread of being a house cat").
						Value(&initiateTheme).
						Validate(func(str string) error {
							if strings.TrimSpace(str) == "" {
								return errors.New("theme cannot be empty")
							}
							return nil
						}),
					huh.NewSelect[string]().
						Title("Format").
						Description("KDP print format").
						Options(
							huh.NewOption("Paperback", "paperback"),
							huh.NewOption("Hardcover", "hardcover"),
						).
						Value(&initiateFormat),
					huh.NewSelect[string]().
						Title("Trim Size").
						Description("Book layout dimensions").
						Options(
							huh.NewOption("8.5x8.5", "8.5x8.5"),
							huh.NewOption("6x9", "6x9"),
						).
						Value(&initiateTrimSize),
					huh.NewInput().
						Title("Pages").
						Description("Target page count").
						Value(&pagesStr).
						Validate(func(str string) error {
							val, err := strconv.Atoi(str)
							if err != nil || val <= 0 {
								return errors.New("page count must be a positive integer")
							}
							return nil
						}),
				),
			)
			form.WithAccessible(!isTTY())
			if err := form.Run(); err != nil {
				return err
			}
			val, err := strconv.Atoi(pagesStr)
			if err != nil {
				return fmt.Errorf("invalid page count: %w", err)
			}
			initiatePages = val
		}

		outputDir := resolveDirectoryFlag(cmd, initiateDir, initiateOutput)
		dirExplicitlySet := cmd.Flags().Changed("dir") || cmd.Flags().Changed("output")

		if !dirExplicitlySet {
			// User did not provide --dir or --output, resolve unique output dir under books/book
			resolvedBase := pipeline.ResolveBookPath(outputDir)
			outputDir = pipeline.GetUniqueOutputDir(resolvedBase)
		} else {
			// User explicitly provided --dir or --output. If it exists, ask for confirmation
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
			Title:           initiateTitle,
			Author:          initiateAuthor,
			Theme:           initiateTheme,
			Style:           initiateStyle,
			Format:          initiateFormat,
			TargetPageCount: initiatePages,
			TrimSize:        initiateTrimSize,
			NoBrainstorm:    initiateNoBrainstorm,
			Context:         cmd.Context(),
			DryRun:          rootDryRun,
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
	initiateCmd.Flags().StringVarP(&initiateDir, "dir", "d", "", "Target workspace directory path (alias for --output)")
	initiateCmd.Flags().StringVar(&initiateTitle, "title", "", "Title of the book (optional, will be generated if omitted)")
	initiateCmd.Flags().StringVar(&initiateAuthor, "author", "", "Author pseudonym for the book (optional, will be generated if omitted)")
	initiateCmd.Flags().StringVar(&initiateTheme, "theme", "", "Theme of the book")
	initiateCmd.Flags().StringVar(&initiateStyle, "style", "", "Style reference for illustrations (supporting Midjourney sref format)")
	initiateCmd.Flags().StringVar(&initiateFormat, "format", "paperback", "KDP print format (paperback or hardcover)")
	initiateCmd.Flags().IntVar(&initiatePages, "pages", 15, "Target page count of the book")
	initiateCmd.Flags().StringVar(&initiateTrimSize, "trim-size", "8.5x8.5", "Trim size of the book (e.g. 6x9, 8.5x8.5)")
	initiateCmd.Flags().BoolVar(&initiateNoBrainstorm, "no-brainstorm", false, "Disable automated visual style and character profile brainstorming")
	rootCmd.AddCommand(initiateCmd)
}
