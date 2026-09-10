package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
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
	previewCmd.Flags().Lookup("dir").Changed = false
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
	statusCmd.Flags().Lookup("dir").Changed = false
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
	cleanCmd.Flags().Lookup("dir").Changed = false
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
		res, isPos, err := resolvePositionalOrDir(cmd, dir, []string{"my-book"})
		if err != nil || res != "my-book" || !isPos {
			t.Errorf("expected ('my-book', true), got (%q, %v), err: %v", res, isPos, err)
		}
	})

	t.Run("dir flag only", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{"--dir", "flag-book"}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		res, isPos, err := resolvePositionalOrDir(cmd, dir, nil)
		if err != nil || res != "flag-book" || isPos {
			t.Errorf("expected ('flag-book', false), got (%q, %v), err: %v", res, isPos, err)
		}
	})

	t.Run("neither positional nor dir flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var dir string
		cmd.Flags().StringVarP(&dir, "dir", "d", "", "test dir")
		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("ParseFlags failed: %v", err)
		}
		_, _, err := resolvePositionalOrDir(cmd, dir, nil)
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
		_, _, err := resolvePositionalOrDir(cmd, dir, []string{"positional-book"})
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

	_ = cleanCmd.Flags().Set("dir", "")
	cleanCmd.Flags().Lookup("dir").Changed = false
	_ = statusCmd.Flags().Set("dir", "")
	statusCmd.Flags().Lookup("dir").Changed = false
	_ = previewCmd.Flags().Set("dir", "")
	previewCmd.Flags().Lookup("dir").Changed = false
}

func TestIsBareSlug(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"bare-slug", true},
		{"books", true},
		{"my-book-123", true},
		{"~", false},
		{"~/book", false},
		{"./books", false},
		{"books/book", false},
		{"/abs/path", false},
		{"..", false},
		{"../books", false},
		{"dir\\book", false},
	}

	for _, tt := range tests {
		if got := isBareSlug(tt.input); got != tt.want {
			t.Errorf("isBareSlug(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolveWorkspaceAndBook(t *testing.T) {
	// Bare book name (positional)
	wsRoot, bName := resolveWorkspaceAndBook("bare-slug", true)
	if bName != "bare-slug" {
		t.Errorf("expected bookName 'bare-slug', got %q", bName)
	}
	if wsRoot == "" {
		t.Errorf("expected non-empty wsRoot for bare slug")
	}

	// Positional bare book name "books" must preserve bare-slug semantics under WorkspacesRoot
	wsRootBooksPos, bNameBooksPos := resolveWorkspaceAndBook("books", true)
	if bNameBooksPos != "books" {
		t.Errorf("expected bookName 'books', got %q", bNameBooksPos)
	}
	if wsRootBooksPos != wsRoot {
		t.Errorf("expected wsRoot for positional 'books' (%q) to match wsRoot for 'bare-slug' (%q)", wsRootBooksPos, wsRoot)
	}

	// Explicit --dir "books" (isPositional == false) must route through ResolveBookPath as a path
	wsRootBooksFlag, bNameBooksFlag := resolveWorkspaceAndBook("books", false)
	if bNameBooksFlag != "books" {
		t.Errorf("expected bookName 'books', got %q", bNameBooksFlag)
	}
	if wsRootBooksFlag != "." {
		t.Errorf("expected wsRoot for --dir 'books' to be '.', got %q", wsRootBooksFlag)
	}

	// Explicit path "./books" should NOT be treated as a bare slug in either mode
	wsRootDotBooks, bNameDotBooks := resolveWorkspaceAndBook("./books", true)
	if bNameDotBooks != "books" {
		t.Errorf("expected bookName 'books', got %q", bNameDotBooks)
	}
	if wsRootDotBooks != "." {
		t.Errorf("expected './books' to resolve relative to current dir '.', got %q", wsRootDotBooks)
	}

	// Absolute path
	absPath := "/tmp/test-workspaces/my-book"
	wsRoot, bName = resolveWorkspaceAndBook(absPath, false)
	if bName != "my-book" {
		t.Errorf("expected bookName 'my-book', got %q", bName)
	}
	if wsRoot != "/tmp/test-workspaces" {
		t.Errorf("expected wsRoot '/tmp/test-workspaces', got %q", wsRoot)
	}

	// Tilde path
	wsRoot, bName = resolveWorkspaceAndBook("~/tilde-book", false)
	if bName != "tilde-book" {
		t.Errorf("expected bookName 'tilde-book', got %q", bName)
	}
	if wsRoot == "" || strings.HasPrefix(wsRoot, "~") {
		t.Errorf("expected expanded wsRoot for tilde path, got %q", wsRoot)
	}

	// Books relative prefix
	wsRoot, bName = resolveWorkspaceAndBook("books/relative-book", false)
	if bName != "relative-book" {
		t.Errorf("expected bookName 'relative-book', got %q", bName)
	}
	if wsRoot != "books" {
		t.Errorf("expected wsRoot 'books', got %q", wsRoot)
	}
}

func TestStatusAndClean_EmptyWorkspacesRoot(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	statusDir = ""
	_ = statusCmd.Flags().Set("dir", "")
	statusCmd.Flags().Lookup("dir").Changed = false

	cleanDir = ""
	_ = cleanCmd.Flags().Set("dir", "")
	cleanCmd.Flags().Lookup("dir").Changed = false

	config.Cfg = &config.Config{
		WorkspacesRoot: "",
	}

	// Positional bare slug with empty WorkspacesRoot must fail with "workspaces root is empty in config"
	err := statusCmd.RunE(statusCmd, []string{"bare-book"})
	if err == nil || !strings.Contains(err.Error(), "workspaces root is empty in config") {
		t.Errorf("expected 'workspaces root is empty in config', got %v", err)
	}

	cleanAll = true
	err = cleanCmd.RunE(cleanCmd, []string{"bare-book"})
	if err == nil || !strings.Contains(err.Error(), "workspaces root is empty in config") {
		t.Errorf("expected 'workspaces root is empty in config', got %v", err)
	}
	cleanAll = false

	// Explicit --dir with absolute path must NOT fail with "workspaces root is empty in config"
	tmpDir := t.TempDir()
	bookDir := filepath.Join(tmpDir, "my-book")

	statusDir = bookDir
	err = statusCmd.RunE(statusCmd, []string{})
	// Error may be manifest not found, but must NOT be "workspaces root is empty in config"
	if err != nil && strings.Contains(err.Error(), "workspaces root is empty in config") {
		t.Errorf("status --dir should not fail with empty workspaces root, got %v", err)
	}
	statusDir = ""

	cleanDir = bookDir
	cleanAll = true
	err = cleanCmd.RunE(cleanCmd, []string{})
	if err != nil && strings.Contains(err.Error(), "workspaces root is empty in config") {
		t.Errorf("clean --dir should not fail with empty workspaces root, got %v", err)
	}
	cleanDir = ""
	cleanAll = false

	_ = statusCmd.Flags().Set("dir", "")
	statusCmd.Flags().Lookup("dir").Changed = false
	_ = cleanCmd.Flags().Set("dir", "")
	cleanCmd.Flags().Lookup("dir").Changed = false
}
