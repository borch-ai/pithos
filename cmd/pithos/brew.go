package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	brewTheme  string
	brewStyle  string
	brewOutput string
)

var brewCmd = &cobra.Command{
	Use:   "brew",
	Short: "Generates the manuscript and image prompts",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("brew: Generating manuscript for theme '%s' with style '%s' into '%s' (Coming soon!)\n", brewTheme, brewStyle, brewOutput)
	},
}

func init() {
	brewCmd.Flags().StringVar(&brewTheme, "theme", "", "Theme for the manuscript")
	brewCmd.Flags().StringVar(&brewStyle, "style", "", "Style reference for images")
	brewCmd.Flags().StringVar(&brewOutput, "output", "", "Output directory")
	rootCmd.AddCommand(brewCmd)
}
