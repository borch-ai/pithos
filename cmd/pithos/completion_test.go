package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInstallZshCompletion_OhMyZsh(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock .oh-my-zsh directory
	omzDir := filepath.Join(tempDir, ".oh-my-zsh")
	if mkErr := os.MkdirAll(omzDir, 0700); mkErr != nil {
		t.Fatalf("failed to create mock omz dir: %v", mkErr)
	}

	writeCalled := false
	writeFunc := func(path string) error {
		writeCalled = true
		expectedPath := filepath.Join(omzDir, "completions", "_pithos")
		if path != expectedPath {
			t.Errorf("expected write path %q, got %q", expectedPath, path)
		}
		return os.WriteFile(path, []byte("# mock completion"), 0600)
	}

	targetPath, instruction, err := installZshCompletion(tempDir, writeFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !writeCalled {
		t.Error("expected write completion func to be called")
	}

	expectedTargetPath := filepath.Join(omzDir, "completions", "_pithos")
	if targetPath != expectedTargetPath {
		t.Errorf("expected target path %q, got %q", expectedTargetPath, targetPath)
	}

	if instruction != "" {
		t.Errorf("expected empty instruction for OMZ, got %q", instruction)
	}

	// Verify content
	content, err := os.ReadFile(filepath.Clean(targetPath))
	if err != nil {
		t.Fatalf("failed to read completion file: %v", err)
	}
	if string(content) != "# mock completion" {
		t.Errorf("expected file content '# mock completion', got %q", string(content))
	}
}

func TestInstallZshCompletion_StandardZsh(t *testing.T) {
	tempDir := t.TempDir()

	writeCalled := false
	writeFunc := func(path string) error {
		writeCalled = true
		expectedPath := filepath.Join(tempDir, ".zsh", "completions", "_pithos")
		if path != expectedPath {
			t.Errorf("expected write path %q, got %q", expectedPath, path)
		}
		return os.WriteFile(path, []byte("# mock completion"), 0600)
	}

	targetPath, instruction, err := installZshCompletion(tempDir, writeFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !writeCalled {
		t.Error("expected write completion func to be called")
	}

	expectedTargetPath := filepath.Join(tempDir, ".zsh", "completions", "_pithos")
	if targetPath != expectedTargetPath {
		t.Errorf("expected target path %q, got %q", expectedTargetPath, targetPath)
	}

	if !strings.Contains(instruction, "To enable the autocomplete") {
		t.Errorf("expected standard Zsh instructions, got %q", instruction)
	}
}

func TestCompletionCmd_Generate(t *testing.T) {
	testRoot := rootCmd

	oldOut := testRoot.OutOrStdout()
	var buf bytes.Buffer
	testRoot.SetOut(&buf)
	defer testRoot.SetOut(oldOut)

	zshBytes, err := generateZshCompletionBytes(testRoot)
	if err != nil {
		t.Fatalf("failed to generate zsh completion bytes: %v", err)
	}

	if !strings.Contains(string(zshBytes), "#compdef pithos") {
		t.Errorf("expected zsh completion script to contain '#compdef pithos'")
	}
}

func generateZshCompletionBytes(cmd *cobra.Command) ([]byte, error) {
	var buf bytes.Buffer
	if err := cmd.GenZshCompletion(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
