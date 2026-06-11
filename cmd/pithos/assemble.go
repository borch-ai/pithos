package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	assembleInput  string
	assembleFormat string
	assembleBleed  bool
)

var assembleCmd = &cobra.Command{
	Use:   "assemble",
	Short: "Combines assets into a print-ready PDF",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("assemble: Assembling assets from '%s' into %s format (bleed: %t) (Coming soon!)\n", assembleInput, assembleFormat, assembleBleed)
	},
}

func init() {
	assembleCmd.Flags().StringVar(&assembleInput, "input", "", "Input directory")
	assembleCmd.Flags().StringVar(&assembleFormat, "format", "paperback", "Format (e.g. hardcover, paperback)")
	assembleCmd.Flags().BoolVar(&assembleBleed, "bleed", false, "Include bleed")
	rootCmd.AddCommand(assembleCmd)
}
