package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	brewTheme       string
	brewStyle       string
	brewOutput      string
	brewDir         string
	brewConcurrency int
	brewReview      bool
	brewPagesStr    string
	brewSilent      bool
	brewSelect      bool
	brewBudget      float64
	brewTUI         bool
)

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Generates the manuscript and stanza illustrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		headless := IsHeadless()
		if headless {
			brewSilent = true
		}

		var pages []int
		if cmd.Flags().Changed("pages") {
			if brewPagesStr == "" {
				return errors.New("pages flag is empty; please specify comma-separated page numbers to regenerate")
			}
			parts := strings.Split(brewPagesStr, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				val, err := strconv.Atoi(part)
				if err != nil {
					return fmt.Errorf("invalid page number %q: %w", part, err)
				}
				if val <= 0 {
					return fmt.Errorf("page numbers must be positive: %d", val)
				}
				pages = append(pages, val)
			}
			if len(pages) == 0 {
				return errors.New("no valid page numbers parsed from pages flag")
			}
		} else if brewSelect {
			if headless {
				return errors.New("interactive page selection (--select) is not supported in headless mode; specify pages explicitly with --pages")
			}
		}

		outputDir := resolveDirectoryFlag(cmd, brewDir, brewOutput)

		opts := pipeline.BrewOptions{
			OutputDir:   outputDir,
			Theme:       brewTheme,
			Style:       brewStyle,
			Concurrency: brewConcurrency,
			Review:      brewReview && !headless,
			TUI:         brewTUI && !headless,
			Pages:       pages,
			Select:      brewSelect && !headless,
			Silent:      brewSilent || headless,
			Headless:    headless,
			DryRun:      rootDryRun,
			Budget:      brewBudget,
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		err := pipeline.Brew(ctx, opts)
		if err != nil {
			if errors.Is(err, pipeline.ErrReviewPause) {
				fmt.Println(err.Error())
				return nil
			}
			return err
		}
		return nil
	},
}

func init() {
	brewCmd.Flags().StringVar(&brewTheme, "theme", "", "Theme for the manuscript (optional override)")
	brewCmd.Flags().StringVar(&brewStyle, "style", "", "Style reference for images (optional override)")
	brewCmd.Flags().StringVar(&brewOutput, "output", "book", "Output directory path")
	brewCmd.Flags().StringVarP(&brewDir, "dir", "d", "", "Workspace directory path (alias for --output)")
	brewCmd.Flags().IntVar(&brewConcurrency, "concurrency", 0, "Number of concurrent image generation workers (defaults to config or 1)")
	brewCmd.Flags().BoolVar(&brewReview, "review", false, "Export manuscript for local markdown review and pause execution")
	brewCmd.Flags().StringVar(&brewPagesStr, "pages", "", "Comma-separated list of page numbers to regenerate (e.g. 2,4)")
	brewCmd.Flags().BoolVar(&brewSelect, "select", false, "Interactively select pages to regenerate")
	brewCmd.Flags().BoolVar(&brewSilent, "silent", false, "Silence automatic opening of web preview in the browser and disable interactive prompts (causing budget warnings to automatically abort)")
	brewCmd.Flags().Float64Var(&brewBudget, "budget", 0.0, "Budget limit in USD for this run (setting to 0.0 or omitting falls back to configured budget limits)")
	brewCmd.Flags().BoolVar(&brewTUI, "tui", false, "Launch interactive Bubbletea TUI dashboard for manuscript review")

	rootCmd.AddCommand(brewCmd)
}
