package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFlagRegistration_DirFlags(t *testing.T) {
	cmds := []*struct {
		name string
		flag string
	}{
		{"initiate", "dir"},
		{"brew", "dir"},
		{"assemble", "dir"},
		{"preview", "dir"},
		{"status", "dir"},
		{"clean", "dir"},
		{"character", "dir"},
	}

	for _, tc := range cmds {
		cmd, _, err := rootCmd.Find([]string{tc.name})
		if err != nil {
			t.Fatalf("command %s not found: %v", tc.name, err)
		}

		f := cmd.Flags().Lookup(tc.flag)
		if f == nil {
			t.Errorf("command %s is missing flag --%s", tc.name, tc.flag)
			continue
		}

		if f.Shorthand != "d" {
			t.Errorf("command %s flag --%s shorthand is %q, expected 'd'", tc.name, tc.flag, f.Shorthand)
		}

		short := cmd.Flags().ShorthandLookup("d")
		if short == nil || short.Name != tc.flag {
			t.Errorf("command %s shorthand -d did not resolve to --%s", tc.name, tc.flag)
		}
	}
}

func TestInitiateCmd_DirFlagParsing(t *testing.T) {
	if err := initiateCmd.Flags().Set("dir", "my-custom-book"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if initiateDir != "my-custom-book" {
		t.Errorf("expected initiateDir to be 'my-custom-book', got %q", initiateDir)
	}

	if err := initiateCmd.ParseFlags([]string{"-d", "shorthand-book"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if initiateDir != "shorthand-book" {
		t.Errorf("expected initiateDir to be 'shorthand-book', got %q", initiateDir)
	}
}

func TestBrewCmd_DirFlagParsing(t *testing.T) {
	if err := brewCmd.Flags().Set("dir", "brew-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if brewDir != "brew-book-dir" {
		t.Errorf("expected brewDir to be 'brew-book-dir', got %q", brewDir)
	}

	if err := brewCmd.ParseFlags([]string{"-d", "brew-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if brewDir != "brew-short-dir" {
		t.Errorf("expected brewDir to be 'brew-short-dir', got %q", brewDir)
	}
}

func TestAssembleCmd_DirFlagParsing(t *testing.T) {
	if err := assembleCmd.Flags().Set("dir", "assemble-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if assembleDir != "assemble-book-dir" {
		t.Errorf("expected assembleDir to be 'assemble-book-dir', got %q", assembleDir)
	}

	if err := assembleCmd.ParseFlags([]string{"-d", "assemble-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if assembleDir != "assemble-short-dir" {
		t.Errorf("expected assembleDir to be 'assemble-short-dir', got %q", assembleDir)
	}
}

func TestPreviewCmd_DirFlagParsing(t *testing.T) {
	if err := previewCmd.Flags().Set("dir", "preview-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if previewDir != "preview-book-dir" {
		t.Errorf("expected previewDir to be 'preview-book-dir', got %q", previewDir)
	}

	if err := previewCmd.ParseFlags([]string{"-d", "preview-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if previewDir != "preview-short-dir" {
		t.Errorf("expected previewDir to be 'preview-short-dir', got %q", previewDir)
	}
}

func TestStatusCmd_DirFlagParsing(t *testing.T) {
	if err := statusCmd.Flags().Set("dir", "status-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if statusDir != "status-book-dir" {
		t.Errorf("expected statusDir to be 'status-book-dir', got %q", statusDir)
	}

	if err := statusCmd.ParseFlags([]string{"-d", "status-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if statusDir != "status-short-dir" {
		t.Errorf("expected statusDir to be 'status-short-dir', got %q", statusDir)
	}
}

func TestCleanCmd_DirFlagParsing(t *testing.T) {
	if err := cleanCmd.Flags().Set("dir", "clean-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if cleanDir != "clean-book-dir" {
		t.Errorf("expected cleanDir to be 'clean-book-dir', got %q", cleanDir)
	}

	if err := cleanCmd.ParseFlags([]string{"-d", "clean-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if cleanDir != "clean-short-dir" {
		t.Errorf("expected cleanDir to be 'clean-short-dir', got %q", cleanDir)
	}
}

func TestCharacterCmd_DirFlagParsing(t *testing.T) {
	if err := characterCmd.Flags().Set("dir", "character-book-dir"); err != nil {
		t.Fatalf("failed to set --dir flag: %v", err)
	}
	if characterDir != "character-book-dir" {
		t.Errorf("expected characterDir to be 'character-book-dir', got %q", characterDir)
	}

	if err := characterCmd.ParseFlags([]string{"-d", "character-short-dir"}); err != nil {
		t.Fatalf("failed to parse -d shorthand: %v", err)
	}
	if characterDir != "character-short-dir" {
		t.Errorf("expected characterDir to be 'character-short-dir', got %q", characterDir)
	}
}

func TestExecution_PreviewMissingArgs(t *testing.T) {
	previewDir = ""
	if err := previewCmd.Flags().Set("dir", ""); err != nil {
		t.Fatalf("failed to reset dir flag on previewCmd: %v", err)
	}
	rootCmd.SetArgs([]string{"preview"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when preview is executed without args or --dir")
	}
	if !strings.Contains(err.Error(), "must specify book name") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExecution_StatusMissingArgs(t *testing.T) {
	statusDir = ""
	if err := statusCmd.Flags().Set("dir", ""); err != nil {
		t.Fatalf("failed to reset dir flag on statusCmd: %v", err)
	}
	rootCmd.SetArgs([]string{"status"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when status is executed without args or --dir")
	}
	if !strings.Contains(err.Error(), "must specify book name") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExecution_CleanMissingArgs(t *testing.T) {
	cleanDir = ""
	if err := cleanCmd.Flags().Set("dir", ""); err != nil {
		t.Fatalf("failed to reset dir flag on cleanCmd: %v", err)
	}
	rootCmd.SetArgs([]string{"clean", "--orphans"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when clean is executed without args or --dir")
	}
	if !strings.Contains(err.Error(), "must specify book name") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPrecedence_DirOverLegacyFlags(t *testing.T) {
	// 1. Test initiateCmd
	// Reset dir flag state first
	initiateCmd.Flags().Lookup("dir").Changed = false
	initiateDir = ""
	initiateOutput = ""

	// Only legacy flag set
	if err := initiateCmd.ParseFlags([]string{"--output", "legacy-out"}); err != nil {
		t.Fatalf("failed to parse legacy flag for initiateCmd: %v", err)
	}
	if got := resolveDirectoryFlag(initiateCmd, initiateDir, initiateOutput); got != "legacy-out" {
		t.Errorf("expected initiate outDir to be 'legacy-out', got %q", got)
	}
	// Both legacy and --dir set (precedence)
	if err := initiateCmd.ParseFlags([]string{"--output", "legacy-out", "--dir", "precedence-dir"}); err != nil {
		t.Fatalf("failed to parse flags for initiateCmd: %v", err)
	}
	if got := resolveDirectoryFlag(initiateCmd, initiateDir, initiateOutput); got != "precedence-dir" {
		t.Errorf("expected initiate outDir to be 'precedence-dir', got %q", got)
	}

	// 2. Test brewCmd
	brewCmd.Flags().Lookup("dir").Changed = false
	brewDir = ""
	brewOutput = ""

	// Only legacy flag set
	if err := brewCmd.ParseFlags([]string{"--output", "legacy-brew"}); err != nil {
		t.Fatalf("failed to parse legacy flag for brewCmd: %v", err)
	}
	if got := resolveDirectoryFlag(brewCmd, brewDir, brewOutput); got != "legacy-brew" {
		t.Errorf("expected brew outDir to be 'legacy-brew', got %q", got)
	}
	// Both legacy and --dir set (precedence)
	if err := brewCmd.ParseFlags([]string{"--output", "legacy-brew", "--dir", "precedence-brew"}); err != nil {
		t.Fatalf("failed to parse flags for brewCmd: %v", err)
	}
	if got := resolveDirectoryFlag(brewCmd, brewDir, brewOutput); got != "precedence-brew" {
		t.Errorf("expected brew outDir to be 'precedence-brew', got %q", got)
	}

	// 3. Test assembleCmd
	assembleCmd.Flags().Lookup("dir").Changed = false
	assembleDir = ""
	assembleInput = ""

	// Only legacy flag set
	if err := assembleCmd.ParseFlags([]string{"--input", "legacy-assemble"}); err != nil {
		t.Fatalf("failed to parse legacy flag for assembleCmd: %v", err)
	}
	if got := resolveDirectoryFlag(assembleCmd, assembleDir, assembleInput); got != "legacy-assemble" {
		t.Errorf("expected assemble inputDir to be 'legacy-assemble', got %q", got)
	}
	// Both legacy and --dir set (precedence)
	if err := assembleCmd.ParseFlags([]string{"--input", "legacy-assemble", "--dir", "precedence-assemble"}); err != nil {
		t.Fatalf("failed to parse flags for assembleCmd: %v", err)
	}
	if got := resolveDirectoryFlag(assembleCmd, assembleDir, assembleInput); got != "precedence-assemble" {
		t.Errorf("expected assemble inputDir to be 'precedence-assemble', got %q", got)
	}

	// 4. Test characterCmd
	characterCmd.Flags().Lookup("dir").Changed = false
	characterDir = ""
	characterOutput = ""

	// Only legacy flag set
	if err := characterCmd.ParseFlags([]string{"--output", "legacy-char"}); err != nil {
		t.Fatalf("failed to parse legacy flag for characterCmd: %v", err)
	}
	if got := resolveDirectoryFlag(characterCmd, characterDir, characterOutput); got != "legacy-char" {
		t.Errorf("expected character outDir to be 'legacy-char', got %q", got)
	}
	// Both legacy and --dir set (precedence)
	if err := characterCmd.ParseFlags([]string{"--output", "legacy-char", "--dir", "precedence-char"}); err != nil {
		t.Fatalf("failed to parse flags for characterCmd: %v", err)
	}
	if got := resolveDirectoryFlag(characterCmd, characterDir, characterOutput); got != "precedence-char" {
		t.Errorf("expected character outDir to be 'precedence-char', got %q", got)
	}
}

func TestResolvePositionalOrDir(t *testing.T) {
	t.Run("positional only", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		res, err := resolvePositionalOrDir(cmd, dir, []string{"my-book"})
		if err != nil || res != "my-book" {
			t.Errorf("expected 'my-book', got %q, err: %v", res, err)
		}
	})

	t.Run("dir flag only", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{"--dir", "flag-book"}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		res, err := resolvePositionalOrDir(cmd, dir, nil)
		if err != nil || res != "flag-book" {
			t.Errorf("expected 'flag-book', got %q, err: %v", res, err)
		}
	})

	t.Run("neither positional nor dir flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		_, err := resolvePositionalOrDir(cmd, dir, nil)
		if err == nil || !strings.Contains(err.Error(), "must specify book name") {
			t.Errorf("expected missing book name error, got %v", err)
		}
	})

	t.Run("both positional and dir flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{"--dir", "flag-book"}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		_, err := resolvePositionalOrDir(cmd, dir, []string{"positional-book"})
		if err == nil || !strings.Contains(err.Error(), "cannot specify both") {
			t.Errorf("expected mutual exclusion error, got %v", err)
		}
	})
}

func TestExecution_RejectBothPositionalAndDir(t *testing.T) {
	// 1. Clean command rejects both positional arg and --dir
	rootCmd.SetArgs([]string{"clean", "--dir", "dir-a", "arg-b", "--orphans"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected clean to reject both positional arg and --dir flag")
	}
	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 2. Status command rejects both positional arg and --dir
	rootCmd.SetArgs([]string{"status", "--dir", "dir-a", "arg-b"})
	buf.Reset()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected status to reject both positional arg and --dir flag")
	}
	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("unexpected error message: %v", err)
	}

	// 3. Preview command rejects both positional arg and --dir
	rootCmd.SetArgs([]string{"preview", "--dir", "dir-a", "arg-b"})
	buf.Reset()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected preview to reject both positional arg and --dir flag")
	}
	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveWorkspaceAndBook(t *testing.T) {
	// Bare book name
	wsRoot, bName := resolveWorkspaceAndBook("bare-slug")
	if bName != "bare-slug" {
		t.Errorf("expected bookName 'bare-slug', got %q", bName)
	}
	if wsRoot == "" {
		t.Errorf("expected non-empty wsRoot for bare slug")
	}

	// Absolute path
	absPath := "/tmp/test-workspaces/my-book"
	wsRoot, bName = resolveWorkspaceAndBook(absPath)
	if bName != "my-book" {
		t.Errorf("expected bookName 'my-book', got %q", bName)
	}
	if wsRoot != "/tmp/test-workspaces" {
		t.Errorf("expected wsRoot '/tmp/test-workspaces', got %q", wsRoot)
	}

	// Tilde path
	wsRoot, bName = resolveWorkspaceAndBook("~/tilde-book")
	if bName != "tilde-book" {
		t.Errorf("expected bookName 'tilde-book', got %q", bName)
	}
	if wsRoot == "" || strings.HasPrefix(wsRoot, "~") {
		t.Errorf("expected expanded wsRoot for tilde path, got %q", wsRoot)
	}

	// Books relative prefix
	wsRoot, bName = resolveWorkspaceAndBook("books/relative-book")
	if bName != "relative-book" {
		t.Errorf("expected bookName 'relative-book', got %q", bName)
	}
	if wsRoot != "books" {
		t.Errorf("expected wsRoot 'books', got %q", wsRoot)
	}
}
