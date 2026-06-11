package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	initiateOutput string
)

var initiateCmd = &cobra.Command{
	Use:   "initiate",
	Short: "Triggers a Smoke Test by generating cover art and social media ad copy",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("initiate: Coming soon! (Output: %s)\n", initiateOutput)
	},
}

func init() {
	initiateCmd.Flags().StringVar(&initiateOutput, "output", "", "Output directory")
	rootCmd.AddCommand(initiateCmd)
}
