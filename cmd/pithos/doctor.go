package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Runs preflight connection diagnostics for MCP plugins and LLM credentials",
	Long:  `doctor executes a series of validation tests to ensure configuration files are loaded, API keys are valid, and native MCP plugin executables can be resolved and connected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		useColor := isTTY()

		if !useColor {
			lipgloss.SetColorProfile(termenv.Ascii)
		} else {
			lipgloss.SetColorProfile(termenv.ColorProfile())
		}

		// Define Lipgloss Styles
		headerStyle := ui.HeaderStyle.MarginBottom(1)
		dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPurple))

		fmt.Println(headerStyle.Render("🏺 Running Pithos Preflight Diagnostics..."))
		divider := dividerStyle.Render("--------------------------------------------------------------------------------")
		fmt.Println(divider)

		results, hasFailure := pipeline.RunDiagnostics(ctx)

		for _, item := range results {
			var badge string
			switch item.Status {
			case pipeline.StatusOk:
				badge = ui.BadgeOk.Render("OK")
			case pipeline.StatusFail:
				badge = ui.BadgeFail.Render("FAIL")
			case pipeline.StatusWarning:
				badge = ui.BadgeWarn.Render("WARN")
			case pipeline.StatusSkip:
				badge = ui.BadgeSkip.Render("SKIP")
			default:
				badge = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color(ui.ColorSkipText)).
					Background(lipgloss.Color("#374151")).
					Width(8).
					Align(lipgloss.Center).
					Render(string(item.Status))
			}

			nameText := item.Name + ":"
			visualWidth := lipgloss.Width(nameText)
			if visualWidth < 52 {
				nameText += strings.Repeat(" ", 52-visualWidth)
			}
			nameStr := ui.KeyStyle.Render(nameText)
			msgStr := ui.ValStyle.Render(item.Message)

			// Print structured row: [BADGE] Name: Message
			fmt.Printf("%s %s %s\n", badge, nameStr, msgStr)
		}

		fmt.Println(divider)
		if hasFailure {
			fmt.Println(ui.FailStyle.Render("Diagnostics FAILED. Please resolve the errors above before running Pithos pipelines."))
			return fmt.Errorf("diagnostics failed")
		}

		fmt.Println(ui.SuccessStyle.Render("All checks passed successfully! Pithos is ready."))
		return nil
	},
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
