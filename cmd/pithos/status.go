package main

import (
	"fmt"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/borch-ai/pithos/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <book>",
	Short: "Show detailed diagnostic status for a book workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}
		if config.Cfg.WorkspacesRoot == "" {
			return fmt.Errorf("workspaces root is empty in config")
		}

		bookName := args[0]
		ws, err := pipeline.GetWorkspaceStatus(config.Cfg.WorkspacesRoot, bookName)
		if err != nil {
			return err
		}

		var sb strings.Builder

		title := ui.HeaderStyle.Render(fmt.Sprintf("WORKSPACE STATUS: %s", ws.BookName))
		sb.WriteString(title + "\n\n")

		sb.WriteString(ui.KeyStyle.Render("Theme: ") + ui.ValStyle.Render(ws.Theme) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Format: ") + ui.ValStyle.Render(ws.Format) + "\n")
		targetStr := "-"
		if ws.TargetPageCount > 0 {
			targetStr = fmt.Sprintf("%d", ws.TargetPageCount)
		}
		sb.WriteString(ui.KeyStyle.Render("Pages: ") + ui.ValStyle.Render(fmt.Sprintf("%d / %s", ws.TotalPages, targetStr)) + "\n\n")

		sb.WriteString(ui.HighlightStyle.Render("PAGE DIAGNOSTICS") + "\n")
		sb.WriteString(ui.KeyStyle.Render("Completed: ") + ui.ValStyle.Render(fmt.Sprintf("%d", ws.Completed)) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Pending: ") + ui.ValStyle.Render(fmt.Sprintf("%d", ws.Pending)) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Generating: ") + ui.ValStyle.Render(fmt.Sprintf("%d", ws.Generating)) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Awaiting: ") + ui.ValStyle.Render(fmt.Sprintf("%d", ws.Awaiting)) + "\n")
		sb.WriteString(ui.KeyStyle.Render("Unknown: ") + ui.ValStyle.Render(fmt.Sprintf("%d", ws.Unknown)) + "\n\n")

		sb.WriteString(ui.HighlightStyle.Render("TELEMETRY") + "\n")
		sb.WriteString(ui.KeyStyle.Render("Total Cost: ") + ui.ValStyle.Render(fmt.Sprintf("$%.2f", ws.TotalCostUSD)) + "\n")

		box := ui.BoxStyle.Render(sb.String())
		fmt.Println(box)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
