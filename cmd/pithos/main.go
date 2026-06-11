package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "pithos",
		Short: "Pithos is a minimalist publishing pipeline",
		Long:  `pithos is a Golang-powered automation pipeline designed for the rapid, low-lift production of niche-market "dark" children's book parodies.`,
	}

	var initiateCmd = &cobra.Command{
		Use:   "initiate",
		Short: "Triggers a Smoke Test by generating cover art and social media ad copy",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("initiate: Coming soon!")
		},
	}

	var brewCmd = &cobra.Command{
		Use:   "brew",
		Short: "Generates the manuscript and image prompts",
		Run: func(cmd *cobra.Command, args []string) {
			theme, _ := cmd.Flags().GetString("theme")
			style, _ := cmd.Flags().GetString("style")
			output, _ := cmd.Flags().GetString("output")
			fmt.Printf("brew: Generating manuscript for theme '%s' with style '%s' into '%s' (Coming soon!)\n", theme, style, output)
		},
	}
	brewCmd.Flags().String("theme", "", "Theme for the manuscript")
	brewCmd.Flags().String("style", "", "Style reference for images")
	brewCmd.Flags().String("output", "", "Output directory")

	var assembleCmd = &cobra.Command{
		Use:   "assemble",
		Short: "Combines assets into a print-ready PDF",
		Run: func(cmd *cobra.Command, args []string) {
			input, _ := cmd.Flags().GetString("input")
			format, _ := cmd.Flags().GetString("format")
			bleed, _ := cmd.Flags().GetBool("bleed")
			fmt.Printf("assemble: Assembling assets from '%s' into %s format (bleed: %t) (Coming soon!)\n", input, format, bleed)
		},
	}
	assembleCmd.Flags().String("input", "", "Input directory")
	assembleCmd.Flags().String("format", "paperback", "Format (e.g. hardcover, paperback)")
	assembleCmd.Flags().Bool("bleed", false, "Include bleed")

	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Packages the final assets and metadata for KDP upload",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("deploy: Coming soon!")
		},
	}

	rootCmd.AddCommand(initiateCmd, brewCmd, assembleCmd, deployCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
