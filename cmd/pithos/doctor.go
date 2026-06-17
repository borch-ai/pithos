package main

import (
	"fmt"
	"os"

	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Runs preflight connection diagnostics for MCP plugins and LLM credentials",
	Long:  `doctor executes a series of validation tests to ensure configuration files are loaded, API keys are valid, and native MCP plugin executables can be resolved and connected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		useColor := isTTY()

		var (
			green  = ""
			red    = ""
			yellow = ""
			gray   = ""
			bold   = ""
			reset  = ""
		)
		if useColor {
			green = "\033[32m"
			red = "\033[31m"
			yellow = "\033[33m"
			gray = "\033[90m"
			bold = "\033[1m"
			reset = "\033[0m"
		}

		fmt.Println("🏺 Running Pithos Preflight Diagnostics...")
		fmt.Println("--------------------------------------------------")

		results, hasFailure := pipeline.RunDiagnostics(ctx)

		for _, item := range results {
			var statusStr string
			switch item.Status {
			case pipeline.StatusOk:
				statusStr = fmt.Sprintf("%s[✔] OK     %s", green, reset)
			case pipeline.StatusFail:
				statusStr = fmt.Sprintf("%s[✘] FAIL   %s", red, reset)
			case pipeline.StatusWarning:
				statusStr = fmt.Sprintf("%s[!] WARN   %s", yellow, reset)
			case pipeline.StatusSkip:
				statusStr = fmt.Sprintf("%s[-] SKIP   %s", gray, reset)
			default:
				statusStr = fmt.Sprintf("[%s]", item.Status)
			}

			fmt.Printf("%s %s%-40s%s %s\n", statusStr, bold, item.Name+":", reset, item.Message)
		}

		fmt.Println("--------------------------------------------------")
		if hasFailure {
			fmt.Printf("%s%sDiagnostics FAILED. Please resolve the errors above before running Pithos pipelines.%s\n", red, bold, reset)
			return fmt.Errorf("diagnostics failed")
		}

		fmt.Printf("%s%sAll checks passed successfully! Pithos is ready.%s\n", green, bold, reset)
		return nil
	},
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
