package main

import (
	"fmt"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Runs preflight connection diagnostics for MCP plugins and LLM credentials",
	Long:  `doctor executes a series of validation tests to ensure configuration files are loaded, API keys are valid, and native MCP plugin executables can be resolved and connected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		fmt.Println("🏺 Running Pithos Preflight Diagnostics...")
		fmt.Println("--------------------------------------------------")

		results, hasFailure := pipeline.RunDiagnostics(ctx)

		for _, item := range results {
			var statusStr string
			switch item.Status {
			case pipeline.StatusOk:
				statusStr = "\033[32m[✔] OK     \033[0m"
			case pipeline.StatusFail:
				statusStr = "\033[31m[✘] FAIL   \033[0m"
			case pipeline.StatusWarning:
				statusStr = "\033[33m[!] WARN   \033[0m"
			case pipeline.StatusSkip:
				statusStr = "\033[90m[-] SKIP   \033[0m"
			default:
				statusStr = fmt.Sprintf("[%s]", item.Status)
			}

			fmt.Printf("%s \033[1m%-40s\033[0m %s\n", statusStr, item.Name+":", item.Message)
		}

		fmt.Println("--------------------------------------------------")
		if hasFailure {
			return fmt.Errorf("\033[31m\033[1mDiagnostics FAILED. Please resolve the errors above before running Pithos pipelines.\033[0m")
		}

		fmt.Println("\033[32m\033[1mAll checks passed successfully! Pithos is ready.\033[0m")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
