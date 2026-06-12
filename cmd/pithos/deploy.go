package main

import (
	"errors"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	deployInput string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Packages the final assets and metadata for KDP upload",
	RunE: func(cmd *cobra.Command, args []string) error {
		if deployInput == "" {
			return errors.New("input directory is required")
		}
		opts := pipeline.DeployOptions{
			InputDir: deployInput,
		}
		return pipeline.Deploy(cmd.Context(), opts)
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployInput, "input", "book", "Input directory path")
	rootCmd.AddCommand(deployCmd)
}
