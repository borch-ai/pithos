package main

import (
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
	brewConcurrency int
	brewReview      bool
	brewPagesStr    string
)

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Generates the manuscript and stanza illustrations",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		}

		opts := pipeline.BrewOptions{
			OutputDir:   brewOutput,
			Theme:       brewTheme,
			Style:       brewStyle,
			Concurrency: brewConcurrency,
			Review:      brewReview,
			Pages:       pages,
		}
		err := pipeline.Brew(cmd.Context(), opts)
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
	brewCmd.Flags().IntVar(&brewConcurrency, "concurrency", 0, "Number of concurrent image generation workers (defaults to config or 1)")
	brewCmd.Flags().BoolVar(&brewReview, "review", false, "Export manuscript for local markdown review and pause execution")
	brewCmd.Flags().StringVar(&brewPagesStr, "pages", "", "Comma-separated list of page numbers to regenerate (e.g. 2,4)")
	rootCmd.AddCommand(brewCmd)
}
