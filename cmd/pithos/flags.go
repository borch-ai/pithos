package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// resolveDirectoryFlag returns dirVal if --dir was explicitly provided, otherwise legacyVal.
func resolveDirectoryFlag(cmd *cobra.Command, dirVal, legacyVal string) string {
	if cmd.Flags().Changed("dir") {
		return dirVal
	}
	return legacyVal
}

// resolvePositionalOrDir resolves the target workspace from either a positional argument or --dir flag.
// If both are specified, it returns an error to prevent accidental execution on the wrong target.
func resolvePositionalOrDir(cmd *cobra.Command, dirVal string, args []string) (string, error) {
	if cmd.Flags().Changed("dir") && len(args) > 0 {
		return "", errors.New("cannot specify both book name as positional argument and --dir flag")
	}
	if dirVal != "" {
		return dirVal, nil
	}
	if len(args) == 0 {
		return "", errors.New("must specify book name as argument or via --dir flag")
	}
	return args[0], nil
}
