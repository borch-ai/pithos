package main

import (
	"fmt"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/spf13/cobra"
)

var (
	assembleInput     string
	assembleFormat    string
	assembleBleed     bool
	assembleTrimSize  string
	assemblePaperType string
	assembleSilent    bool
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
			Silent:    assembleSilent,
		}
		m, err := pipeline.Assemble(cmd.Context(), opts)
		if err != nil {
			return err
		}

		if !isTTY() {
			fmt.Printf("pithos assemble: Successfully generated layout manifest parameters for %s format.\n", m.BookProperties.Format)
			fmt.Printf("Calculated Dimensions (inches): Cover Width: %.3f, Cover Height: %.3f, Spine Width: %.3f\n",
				m.KDPLayout.CoverWidthInches, m.KDPLayout.CoverHeightInches, m.KDPLayout.SpineWidth)
			return nil
		}

		// Style the output using centralized Lipgloss styles
		titleStyle := ui.HeaderStyle.Padding(0, 1)
		boxStyle := ui.BoxStyle
		keyStyle := ui.KeyStyle
		valStyle := ui.ValStyle
		highlightStyle := ui.HighlightStyle

		var sb strings.Builder
		fmt.Fprintf(&sb, "%s %s\n", keyStyle.Render("Format:"), valStyle.Render(m.BookProperties.Format))
		fmt.Fprintf(&sb, "%s %s\n", keyStyle.Render("Trim Size:"), valStyle.Render(m.BookProperties.TrimSize))

		pageCount := len(m.Progress.Pages)
		if pageCount == 0 {
			pageCount = m.BookProperties.TargetPageCount
		}
		fmt.Fprintf(&sb, "%s %d\n", keyStyle.Render("Page Count:"), pageCount)
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%s\n", highlightStyle.Render("Calculated Print Dimensions (inches):"))
		fmt.Fprintf(&sb, "- %s %.3f\n", keyStyle.Render("Cover Width:"), m.KDPLayout.CoverWidthInches)
		fmt.Fprintf(&sb, "- %s %.3f\n", keyStyle.Render("Cover Height:"), m.KDPLayout.CoverHeightInches)
		fmt.Fprintf(&sb, "- %s %.3f", keyStyle.Render("Spine Width:"), m.KDPLayout.SpineWidth)

		titleStr := titleStyle.Render("ASSEMBLY SUMMARY")
		card := boxStyle.Render(titleStr + "\n\n" + sb.String())
		fmt.Println(card)

		return nil
	},
}

func init() {
	assembleCmd.Flags().StringVar(&assembleInput, "input", "book", "Input directory path")
	assembleCmd.Flags().StringVar(&assembleFormat, "format", "", "KDP print format (paperback or hardcover, defaults to format in manifest)")
	assembleCmd.Flags().BoolVar(&assembleBleed, "bleed", false, "Include bleed margins")
	assembleCmd.Flags().StringVar(&assembleTrimSize, "trim-size", "6x9", "Trim size of the book (e.g. 6x9, 5.5x8.5)")
	assembleCmd.Flags().StringVar(&assemblePaperType, "paper-type", "white", "Paper type of the book (white, cream, standard_color, premium_color)")
	assembleCmd.Flags().BoolVar(&assembleSilent, "silent", false, "Silence automatic opening of web preview in the browser")
	rootCmd.AddCommand(assembleCmd)
}
