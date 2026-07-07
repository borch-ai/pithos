package main

import (
	"github.com/borch-ai/pithos/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	rootDryRun bool
	rootCmd    = &cobra.Command{
		Use:          "pithos",
		Short:        "Pithos is a minimalist publishing pipeline",
		Long:         `pithos is a Golang-powered automation pipeline designed for the rapid, low-lift production of niche-market "dark" children's book parodies.`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.LoadConfig(cfgFile)
			return err
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .pithos.toml or ~/.config/pithos/config.toml)")
	rootCmd.PersistentFlags().BoolVar(&rootDryRun, "dry-run", false, "dry run mode (bypass API keys and MCP invocations)")
}
