package main

import (
	"fmt"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all book workspaces",
	Long:  `ls scans the workspaces directory, reading manifest.json for each book and listing status, page count, and costs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Cfg == nil {
			return fmt.Errorf("configuration is not loaded")
		}

		root := config.Cfg.WorkspacesRoot
		if root == "" {
			return fmt.Errorf("workspaces root is empty in config")
		}

		summaries, err := pipeline.ListWorkspaces(root)
		if err != nil {
			return fmt.Errorf("failed to list workspaces: %w", err)
		}

		if len(summaries) == 0 {
			fmt.Println("No workspaces found in:", root)
			return nil
		}

		// Style header and table using lipgloss
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")). // Hot pink
			Align(lipgloss.Center)

		borderStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")) // Purple

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(borderStyle).
			Headers("DIRNAME", "THEME", "FORMAT", "PAGES", "MILESTONES", "COST").
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == 0 {
					return headerStyle
				}
				// Default styling for rows
				return lipgloss.NewStyle().Padding(0, 1)
			})

		for _, s := range summaries {
			theme := s.Theme
			if theme == "" {
				theme = "-"
			}
			format := s.Format
			if format == "" {
				format = "-"
			}
			pagesStr := fmt.Sprintf("%d/%d", s.PageCount, s.TargetPageCount)
			milestonesStr := strings.Join(s.Milestones, ", ")
			if milestonesStr == "" {
				milestonesStr = "-"
			}
			costStr := fmt.Sprintf("$%.2f", s.TotalCost)

			t.Row(s.DirName, theme, format, pagesStr, milestonesStr, costStr)
		}

		fmt.Println(t.Render())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
