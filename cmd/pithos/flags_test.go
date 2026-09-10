package main

import (
	"bytes"
	"strings"
	"testing"
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
	_ = previewCmd.Flags().Set("dir", "")
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
	_ = statusCmd.Flags().Set("dir", "")
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
	_ = cleanCmd.Flags().Set("dir", "")
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
	// Test initiate: both --output and --dir
	_ = initiateCmd.ParseFlags([]string{"--output", "legacy-out", "--dir", "precedence-dir"})
	outDir := initiateOutput
	if initiateCmd.Flags().Changed("dir") {
		outDir = initiateDir
	}
	if outDir != "precedence-dir" {
		t.Errorf("expected initiate outDir to be 'precedence-dir', got %q", outDir)
	}

	// Test brew: both --output and --dir
	_ = brewCmd.ParseFlags([]string{"--output", "legacy-brew", "--dir", "precedence-brew"})
	bDir := brewOutput
	if brewCmd.Flags().Changed("dir") {
		bDir = brewDir
	}
	if bDir != "precedence-brew" {
		t.Errorf("expected brew bDir to be 'precedence-brew', got %q", bDir)
	}

	// Test assemble: both --input and --dir
	_ = assembleCmd.ParseFlags([]string{"--input", "legacy-assemble", "--dir", "precedence-assemble"})
	aDir := assembleInput
	if assembleCmd.Flags().Changed("dir") {
		aDir = assembleDir
	}
	if aDir != "precedence-assemble" {
		t.Errorf("expected assemble aDir to be 'precedence-assemble', got %q", aDir)
	}

	// Test character: both --output and --dir
	_ = characterCmd.ParseFlags([]string{"--output", "legacy-char", "--dir", "precedence-char"})
	cDir := characterOutput
	if characterCmd.Flags().Changed("dir") {
		cDir = characterDir
	}
	if cDir != "precedence-char" {
		t.Errorf("expected character cDir to be 'precedence-char', got %q", cDir)
	}
}
