package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	completionInstall bool
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script for the specified shell",
	Long: `To load completions:

Bash:

  $ source <(pithos completion bash)

  # To load completions for each session, add this to your .bashrc:
  # source <(pithos completion bash)

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:

  $ echo "autoload -Uz compinit; compinit" >> ~/.zshrc

  # To load completions for each session, add this to your .zshrc:
  # source <(pithos completion zsh)

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ pithos completion fish | source

  # To load completions for each session, write the output to:
  # ~/.config/fish/completions/pithos.fish

PowerShell:

  PS> pithos completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  # pithos completion powershell > $PROFILE.CurrentUserAllHosts
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]

		if completionInstall {
			if shell != "zsh" {
				return fmt.Errorf("automatic installation is only supported for 'zsh'")
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}

			writeCompletionFunc := func(path string) error {
				file, fileErr := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) //nolint:gosec
				if fileErr != nil {
					return fileErr
				}
				defer func() {
					_ = file.Close()
				}()
				return cmd.Root().GenZshCompletion(file)
			}

			targetPath, instruction, err := installZshCompletion(homeDir, writeCompletionFunc)
			if err != nil {
				return err
			}

			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
			fmt.Printf("%s Successfully installed completions to %s\n", successStyle.Render("[✔]"), targetPath)
			if instruction != "" {
				fmt.Println()
				fmt.Println(instruction)
			}
			return nil
		}

		switch shell {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func installZshCompletion(homeDir string, writeCompletion func(path string) error) (string, string, error) {
	omzDir := filepath.Join(homeDir, ".oh-my-zsh")
	if fi, err := os.Stat(omzDir); err == nil && fi.IsDir() {
		omzCompletionsDir := filepath.Join(omzDir, "completions")
		if err := os.MkdirAll(omzCompletionsDir, 0750); err != nil {
			return "", "", fmt.Errorf("failed to create Oh-My-Zsh completions directory: %w", err)
		}
		targetPath := filepath.Join(omzCompletionsDir, "_pithos")
		if err := writeCompletion(targetPath); err != nil {
			return "", "", fmt.Errorf("failed to write completion file: %w", err)
		}
		return targetPath, "", nil
	}

	zshCompletionsDir := filepath.Join(homeDir, ".zsh", "completions")
	if err := os.MkdirAll(zshCompletionsDir, 0750); err != nil {
		return "", "", fmt.Errorf("failed to create Zsh completions directory: %w", err)
	}
	targetPath := filepath.Join(zshCompletionsDir, "_pithos")
	if err := writeCompletion(targetPath); err != nil {
		return "", "", fmt.Errorf("failed to write completion file: %w", err)
	}

	instruction := fmt.Sprintf("To enable the autocomplete, add the following to your ~/.zshrc:\n\n"+
		"    fpath=(%s $fpath)\n"+
		"    autoload -Uz compinit && compinit\n", zshCompletionsDir)
	return targetPath, instruction, nil
}

func init() {
	completionCmd.Flags().BoolVarP(&completionInstall, "install", "i", false, "Automatically install Zsh completions to standard directories")
	rootCmd.AddCommand(completionCmd)
}

// GenerateZshCompletionBytes is a helper function used for testing Zsh completion script generation.
func GenerateZshCompletionBytes(cmd *cobra.Command) ([]byte, error) {
	var buf bytes.Buffer
	if err := cmd.GenZshCompletion(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
