package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	deployInput string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Packages the final assets and metadata for KDP upload",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("deploy: Coming soon! (Input: %s)\n", deployInput)
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployInput, "input", "", "Input directory")
	rootCmd.AddCommand(deployCmd)
}
