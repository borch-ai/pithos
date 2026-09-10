package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/pipeline"
	"github.com/spf13/cobra"
)

// resolveDirectoryFlag returns dirVal if --dir was explicitly provided, otherwise legacyVal.
func resolveDirectoryFlag(cmd *cobra.Command, dirVal, legacyVal string) string {
	if cmd.Flags().Changed("dir") {
		return dirVal
	}
	return legacyVal
}

// resolvePositionalOrDir resolves the target workspace and indicates whether it was supplied via a positional argument.
// If both are specified, it returns an error to prevent accidental execution on the wrong target.
func resolvePositionalOrDir(cmd *cobra.Command, dirVal string, args []string) (target string, isPositional bool, err error) {
	if cmd.Flags().Changed("dir") && len(args) > 0 {
		return "", false, errors.New("cannot specify both book name as positional argument and --dir flag")
	}
	if dirVal != "" {
		return dirVal, false, nil
	}
	if len(args) == 0 {
		return "", false, errors.New("must specify book name as argument or via --dir flag")
	}
	return args[0], true, nil
}

// isBareSlug returns true if target is a bare book name without directory separators
// or special path prefixes (such as '~', '.', or absolute paths).
func isBareSlug(target string) bool {
	if target == "" {
		return false
	}
	if strings.ContainsAny(target, "/\\") || filepath.IsAbs(target) {
		return false
	}
	if strings.HasPrefix(target, "~") || strings.HasPrefix(target, ".") {
		return false
	}
	return true
}

// resolveWorkspaceAndBook resolves target path or slug using pipeline.ResolveBookPath
// and splits it into workspaceRoot and bookName.
// If isPositional is true and target is a bare slug (e.g. "books" or "my-book"),
// it preserves legacy bare-slug semantics under WorkspacesRoot.
// If target was explicitly supplied via --dir (isPositional == false),
// it routes through pipeline.ResolveBookPath, maintaining consistency with all other commands.
func resolveWorkspaceAndBook(target string, isPositional bool) (workspaceRoot string, bookName string) {
	if isPositional && isBareSlug(target) {
		root := ""
		if config.Cfg != nil && config.Cfg.WorkspacesRoot != "" {
			root = config.Cfg.WorkspacesRoot
		} else {
			root = pipeline.ResolveBookPath(".")
		}
		return root, target
	}
	resolved := pipeline.ResolveBookPath(target)
	return filepath.Dir(resolved), filepath.Base(resolved)
}
