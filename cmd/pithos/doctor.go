package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/borch-ai/pithos/internal/pipeline"
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
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")). // Hot pink
			MarginBottom(1)

		dividerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")) // Purple

		styleOk := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1B2A1C")). // dark forest green
			Background(lipgloss.Color("#A3E635")). // bright lime green
			Width(8).
			Align(lipgloss.Center)

		styleFail := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FEE2E2")). // light red
			Background(lipgloss.Color("#EF4444")). // bright red
			Width(8).
			Align(lipgloss.Center)

		styleWarn := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FEF3C7")). // light yellow/amber
			Background(lipgloss.Color("#F59E0B")). // bright amber
			Width(8).
			Align(lipgloss.Center)

		styleSkip := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F3F4F6")). // light gray
			Background(lipgloss.Color("#6B7280")). // slate gray
			Width(8).
			Align(lipgloss.Center)

		styleName := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

		styleMessage := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")) // muted gray

		successStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A3E635")).
			MarginTop(1)

		failStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444")).
			MarginTop(1)

		fmt.Println(headerStyle.Render("🏺 Running Pithos Preflight Diagnostics..."))
		divider := dividerStyle.Render("--------------------------------------------------------------------------------")
		fmt.Println(divider)

		results, hasFailure := pipeline.RunDiagnostics(ctx)

		for _, item := range results {
			var badge string
			switch item.Status {
			case pipeline.StatusOk:
				badge = styleOk.Render("OK")
			case pipeline.StatusFail:
				badge = styleFail.Render("FAIL")
			case pipeline.StatusWarning:
				badge = styleWarn.Render("WARN")
			case pipeline.StatusSkip:
				badge = styleSkip.Render("SKIP")
			default:
				badge = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#F3F4F6")).
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
			nameStr := styleName.Render(nameText)
			msgStr := styleMessage.Render(item.Message)

			// Print structured row: [BADGE] Name: Message
			fmt.Printf("%s %s %s\n", badge, nameStr, msgStr)
		}

		fmt.Println(divider)
		if hasFailure {
			fmt.Println(failStyle.Render("Diagnostics FAILED. Please resolve the errors above before running Pithos pipelines."))
			return fmt.Errorf("diagnostics failed")
		}

		fmt.Println(successStyle.Render("All checks passed successfully! Pithos is ready."))
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
