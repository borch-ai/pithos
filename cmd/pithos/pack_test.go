package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/pipeline"
)

func setupTestWorkspaceForCLI(t *testing.T, preflightPassed bool) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "pithos-cli-pack-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Existential Parody"
	m.BookProperties.Format = "6x9"

	interiorContent := []byte("%PDF-1.4 Mock Interior PDF Content for CLI")
	coverContent := []byte("%PDF-1.4 Mock Cover PDF Content for CLI")

	if err := os.WriteFile(filepath.Join(tmpDir, "interior.pdf"), interiorContent, 0600); err != nil {
		t.Fatalf("failed to write interior.pdf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "cover.pdf"), coverContent, 0600); err != nil {
		t.Fatalf("failed to write cover.pdf: %v", err)
	}

	m.AssetRegistry["interior_pdf"] = filepath.Join(tmpDir, "interior.pdf")
	m.AssetRegistry["cover_pdf"] = filepath.Join(tmpDir, "cover.pdf")

	if preflightPassed {
		m.Progress.PreflightPassed = true
		m.Progress.PreflightHashes = map[string]string{
			"interior.pdf": pipeline.ComputeBytesSHA256(interiorContent),
			"cover.pdf":    pipeline.ComputeBytesSHA256(coverContent),
		}
	}

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return tmpDir
}

func resetPackFlags() {
	packDir = ""
	packForce = false
	packOutput = ""
	if packCmd.Flags().Lookup("dir") != nil {
		_ = packCmd.Flags().Set("dir", "")
		packCmd.Flags().Lookup("dir").Changed = false
	}
	if packCmd.Flags().Lookup("force") != nil {
		_ = packCmd.Flags().Set("force", "false")
		packCmd.Flags().Lookup("force").Changed = false
	}
	if packCmd.Flags().Lookup("output") != nil {
		_ = packCmd.Flags().Set("output", "")
		packCmd.Flags().Lookup("output").Changed = false
	}
}

func TestPackCmd_FlagRegistration(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	dirFlag := packCmd.Flags().Lookup("dir")
	if dirFlag == nil || dirFlag.Shorthand != "d" {
		t.Errorf("expected --dir flag with -d shorthand, got %v", dirFlag)
	}

	forceFlag := packCmd.Flags().Lookup("force")
	if forceFlag == nil || forceFlag.Shorthand != "f" {
		t.Errorf("expected --force flag with -f shorthand, got %v", forceFlag)
	}

	outFlag := packCmd.Flags().Lookup("output")
	if outFlag == nil || outFlag.Shorthand != "o" {
		t.Errorf("expected --output flag with -o shorthand, got %v", outFlag)
	}
}

func TestPackCmd_ConfigNil(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	oldCfg := config.Cfg
	config.Cfg = nil
	defer func() { config.Cfg = oldCfg }()

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)
	err := packCmd.RunE(packCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "configuration is not loaded") {
		t.Fatalf("expected config not loaded error, got: %v", err)
	}
}

func TestPackCmd_MissingTarget(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: "/tmp"}
	defer func() { config.Cfg = oldCfg }()

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)
	err := packCmd.RunE(packCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "must specify book name as argument or via --dir flag") {
		t.Fatalf("expected missing book name error, got: %v", err)
	}
}

func TestPackCmd_Execution_RejectBothPositionalAndDir(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: "/tmp"}
	defer func() { config.Cfg = oldCfg }()

	packDir = "dir-a"
	_ = packCmd.Flags().Set("dir", "dir-a")
	packCmd.Flags().Lookup("dir").Changed = true

	err := packCmd.RunE(packCmd, []string{"book-b"})
	if err == nil || !strings.Contains(err.Error(), "cannot specify both") {
		t.Fatalf("expected mutual exclusion error, got: %v", err)
	}
}

func TestPackCmd_Execution_EmptyWorkspacesRoot(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: ""}
	defer func() { config.Cfg = oldCfg }()

	err := packCmd.RunE(packCmd, []string{"book-a"})
	if err == nil || !strings.Contains(err.Error(), "workspaces root is empty in config") {
		t.Fatalf("expected empty workspaces root error, got: %v", err)
	}
}

func TestPackCmd_Execution_DirFlag(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, true)
	defer func() { _ = os.RemoveAll(wsDir) }()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: filepath.Dir(wsDir)}
	defer func() { config.Cfg = oldCfg }()

	packDir = wsDir
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{})
	if err != nil {
		t.Fatalf("pack command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "pithos pack: Successfully packaged") {
		t.Errorf("expected plain text success output, got: %s", output)
	}
	if !strings.Contains(output, "interior.pdf") {
		t.Errorf("expected checksums for interior.pdf in output, got: %s", output)
	}
}

func TestPackCmd_Execution_Positional(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, true)
	defer func() { _ = os.RemoveAll(wsDir) }()

	bookName := filepath.Base(wsDir)
	workspacesRoot := filepath.Dir(wsDir)

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: workspacesRoot}
	defer func() { config.Cfg = oldCfg }()

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{bookName})
	if err != nil {
		t.Fatalf("pack command with positional arg failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "pithos pack: Successfully packaged") {
		t.Errorf("expected success output, got: %s", output)
	}
}

func TestPackCmd_Execution_PreflightErrorAndForce(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, false)
	defer func() { _ = os.RemoveAll(wsDir) }()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: filepath.Dir(wsDir)}
	defer func() { config.Cfg = oldCfg }()

	// 1. Without --force, should fail
	packDir = wsDir
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "preflight validation has not passed") {
		t.Fatalf("expected preflight validation error, got: %v", err)
	}

	resetPackFlags()

	// 2. With --force, should succeed
	packDir = wsDir
	packForce = true
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true
	_ = packCmd.Flags().Set("force", "true")
	packCmd.Flags().Lookup("force").Changed = true

	buf.Reset()
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err = packCmd.RunE(packCmd, []string{})
	if err != nil {
		t.Fatalf("expected pack --force to succeed, got error: %v", err)
	}
}

func TestPackCmd_Execution_CustomOutput(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, true)
	defer func() { _ = os.RemoveAll(wsDir) }()

	customZip := filepath.Join(wsDir, "my-custom-export.zip")

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: filepath.Dir(wsDir)}
	defer func() { config.Cfg = oldCfg }()

	packDir = wsDir
	packOutput = customZip
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true
	_ = packCmd.Flags().Set("output", customZip)
	packCmd.Flags().Lookup("output").Changed = true

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{})
	if err != nil {
		t.Fatalf("pack with custom -o failed: %v", err)
	}

	if _, statErr := os.Stat(customZip); statErr != nil {
		t.Fatalf("expected custom zip %q to exist: %v", customZip, statErr)
	}
}

func TestPackCmd_Execution_HeadlessMode(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, true)
	defer func() { _ = os.RemoveAll(wsDir) }()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: filepath.Dir(wsDir)}
	defer func() { config.Cfg = oldCfg }()

	pipeline.HeadlessMode = true
	defer func() { pipeline.HeadlessMode = false }()

	packDir = wsDir
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{})
	if err != nil {
		t.Fatalf("headless pack failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "pithos pack: Successfully packaged") {
		t.Errorf("expected plain text headless output, got: %s", output)
	}
}

func TestPackCmd_Execution_TTY(t *testing.T) {
	resetPackFlags()
	defer resetPackFlags()

	wsDir := setupTestWorkspaceForCLI(t, true)
	defer func() { _ = os.RemoveAll(wsDir) }()

	oldCfg := config.Cfg
	config.Cfg = &config.Config{WorkspacesRoot: filepath.Dir(wsDir)}
	defer func() { config.Cfg = oldCfg }()

	oldInteractive := rootInteractive
	rootInteractive = true
	defer func() { rootInteractive = oldInteractive }()
	t.Setenv("CI", "")

	oldIsTTY := isTTY
	isTTY = func() bool { return true }
	defer func() { isTTY = oldIsTTY }()

	packDir = wsDir
	_ = packCmd.Flags().Set("dir", wsDir)
	packCmd.Flags().Lookup("dir").Changed = true

	var buf bytes.Buffer
	packCmd.SetOut(&buf)
	packCmd.SetErr(&buf)

	err := packCmd.RunE(packCmd, []string{})
	if err != nil {
		t.Fatalf("TTY pack failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PRINT-READY ARTIFACT PACKAGE") {
		t.Errorf("expected styled TTY card output, got: %s", output)
	}
	if !strings.Contains(output, "SHA-256 CHECKSUMS") {
		t.Errorf("expected checksums header in styled output, got: %s", output)
	}
}
