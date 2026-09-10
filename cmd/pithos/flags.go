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
// For bare slugs (e.g. "my-book" or "books"), it preserves bare-slug semantics under WorkspacesRoot.
func resolveWorkspaceAndBook(target string) (workspaceRoot string, bookName string) {
	if isBareSlug(target) {
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
