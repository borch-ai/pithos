package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/pipeline"
)

func TestIsTruthy(t *testing.T) {
	truthy := []string{"1", "true", "True", "TRUE", "yes", "YES", "on", "ON", "t", "T", "y", "Y"}
	for _, s := range truthy {
		if !isTruthy(s) {
			t.Errorf("expected %q to be truthy", s)
		}
	}

	falsy := []string{"0", "false", "no", "off", "", "random", "2"}
	for _, s := range falsy {
		if isTruthy(s) {
			t.Errorf("expected %q to be falsy", s)
		}
	}
}

//nolint:funlen,gocognit // TestIsHeadless_Detection coordinates multiple subtests verifying detection vectors
func TestIsHeadless_Detection(t *testing.T) {
	// Save and restore state
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origInteractive := rootInteractive
	origIsStdinTTY := isStdinTTY
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		rootInteractive = origInteractive
		isStdinTTY = origIsStdinTTY
	}()

	t.Run("Flag --headless", func(t *testing.T) {
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")
		t.Setenv("PITHOS_INTERACTIVE", "")
		isStdinTTY = func() bool { return true }
		rootHeadless = true
		rootNonInteractive = false
		rootInteractive = false

		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when rootHeadless is true")
		}
	})

	t.Run("Flag --non-interactive", func(t *testing.T) {
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")
		t.Setenv("PITHOS_INTERACTIVE", "")
		isStdinTTY = func() bool { return true }
		rootHeadless = false
		rootNonInteractive = true
		rootInteractive = false

		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when rootNonInteractive is true")
		}
	})

	t.Run("Env PITHOS_HEADLESS", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
		rootInteractive = false
		isStdinTTY = func() bool { return true }

		cases := []struct {
			val      string
			expected bool
		}{
			{"1", true},
			{"true", true},
			{"yes", true},
			{"on", true},
			{"0", false},
			{"false", false},
			{"", false},
		}

		for _, tc := range cases {
			t.Setenv("PITHOS_HEADLESS", tc.val)
			t.Setenv("CI", "")
			t.Setenv("PITHOS_INTERACTIVE", "")
			if got := IsHeadless(); got != tc.expected {
				t.Errorf("PITHOS_HEADLESS=%q: expected IsHeadless() == %v, got %v", tc.val, tc.expected, got)
			}
		}
	})

	t.Run("Env CI", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
		rootInteractive = false
		isStdinTTY = func() bool { return true }
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("PITHOS_INTERACTIVE", "")

		cases := []struct {
			val      string
			expected bool
		}{
			{"1", true},
			{"true", true},
			{"yes", true},
			{"0", false},
			{"false", false},
			{"", false},
		}

		for _, tc := range cases {
			t.Setenv("CI", tc.val)
			if got := IsHeadless(); got != tc.expected {
				t.Errorf("CI=%q: expected IsHeadless() == %v, got %v", tc.val, tc.expected, got)
			}
		}
	})

	t.Run("Non-TTY stdin and interactive override", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
		rootInteractive = false
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")
		t.Setenv("PITHOS_INTERACTIVE", "")

		isStdinTTY = func() bool { return false }
		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when stdin is non-TTY")
		}

		isStdinTTY = func() bool { return true }
		if IsHeadless() {
			t.Error("expected IsHeadless() == false when stdin is TTY")
		}

		// Test interactive override flags/env
		isStdinTTY = func() bool { return false }
		rootInteractive = true
		if IsHeadless() {
			t.Error("expected IsHeadless() == false when rootInteractive is true")
		}
		rootInteractive = false

		t.Setenv("PITHOS_INTERACTIVE", "1")
		if IsHeadless() {
			t.Error("expected IsHeadless() == false when PITHOS_INTERACTIVE is 1")
		}
	})
}

//nolint:funlen // TestInitiateCmd_HeadlessValidation tests multiple headless validation branches
func TestInitiateCmd_HeadlessValidation(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origInteractive := rootInteractive
	origIsStdinTTY := isStdinTTY
	origTheme := initiateTheme
	origDir := initiateDir
	origOutput := initiateOutput
	origNoBrainstorm := initiateNoBrainstorm
	origDryRun := rootDryRun
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		rootInteractive = origInteractive
		isStdinTTY = origIsStdinTTY
		initiateTheme = origTheme
		initiateDir = origDir
		initiateOutput = origOutput
		initiateNoBrainstorm = origNoBrainstorm
		rootDryRun = origDryRun
		_ = initiateCmd.Flags().Set("dir", "")
		initiateCmd.Flags().Lookup("dir").Changed = false
		_ = initiateCmd.Flags().Set("output", "book")
		initiateCmd.Flags().Lookup("output").Changed = false
	}()

	rootHeadless = true
	isStdinTTY = func() bool { return true }
	t.Setenv("PITHOS_HEADLESS", "1")

	// 1. Missing theme fails fast with clear guidance
	initiateTheme = ""
	err := initiateCmd.RunE(initiateCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "theme is required in headless mode") {
		t.Errorf("expected error mentioning theme is required in headless mode, got %v", err)
	}

	// 2. Brand new directory with theme succeeds in headless mode
	tmpDir := t.TempDir()
	newBookDir := filepath.Join(tmpDir, "brand-new-book")
	initiateTheme = "Valid Parody Theme"
	initiateDir = newBookDir
	_ = initiateCmd.Flags().Set("dir", newBookDir)
	initiateCmd.Flags().Lookup("dir").Changed = true
	initiateNoBrainstorm = true
	rootDryRun = true

	err = initiateCmd.RunE(initiateCmd, []string{})
	if err != nil {
		t.Errorf("expected initiate to succeed in headless mode with valid args, got %v", err)
	}

	// 3. Existing directory in headless mode fails fast without prompting
	existingDir := filepath.Join(tmpDir, "existing-book")
	if mkdirErr := os.MkdirAll(existingDir, 0750); mkdirErr != nil {
		t.Fatalf("failed to create existing dir: %v", mkdirErr)
	}
	initiateDir = existingDir
	_ = initiateCmd.Flags().Set("dir", existingDir)
	initiateCmd.Flags().Lookup("dir").Changed = true
	err = initiateCmd.RunE(initiateCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "cannot prompt for overwrite in headless mode") {
		t.Errorf("expected error mentioning cannot prompt for overwrite in headless mode, got %v", err)
	}

	// 4. Non-TTY stdin auto-detects headless mode and fails fast on missing theme
	rootHeadless = false
	rootNonInteractive = false
	rootInteractive = false
	t.Setenv("PITHOS_HEADLESS", "")
	t.Setenv("CI", "")
	t.Setenv("PITHOS_INTERACTIVE", "")
	isStdinTTY = func() bool { return false }
	initiateTheme = ""

	err = initiateCmd.RunE(initiateCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "theme is required in headless mode") {
		t.Errorf("expected non-TTY stdin to trigger headless theme validation error, got %v", err)
	}
}

func TestBrewCmd_HeadlessValidation(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origSelect := brewSelect
	origPages := brewPagesStr
	origReview := brewReview
	origTUI := brewTUI
	origSilent := brewSilent
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		brewSelect = origSelect
		brewPagesStr = origPages
		brewReview = origReview
		brewTUI = origTUI
		brewSilent = origSilent
		_ = brewCmd.Flags().Set("pages", "")
		brewCmd.Flags().Lookup("pages").Changed = false
	}()

	rootHeadless = true
	t.Setenv("PITHOS_HEADLESS", "1")

	// 1. --select in headless mode fails fast with clear guidance
	brewSelect = true
	_ = brewCmd.Flags().Set("pages", "")
	brewCmd.Flags().Lookup("pages").Changed = false

	err := brewCmd.RunE(brewCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "interactive page selection (--select) is not supported in headless mode") {
		t.Errorf("expected --select error in headless mode, got %v", err)
	}

	// 2. --select together with --pages in headless mode still fails fast on --select
	brewSelect = true
	brewPagesStr = "2"
	_ = brewCmd.Flags().Set("pages", "2")
	brewCmd.Flags().Lookup("pages").Changed = true

	err = brewCmd.RunE(brewCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "interactive page selection (--select) is not supported in headless mode") {
		t.Errorf("expected --select error in headless mode when pages is specified, got %v", err)
	}
}

func TestPreviewCmd_HeadlessSuppression(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origCfg := config.Cfg
	origDir := previewDir
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		config.Cfg = origCfg
		previewDir = origDir
		_ = previewCmd.Flags().Set("dir", "")
		previewCmd.Flags().Lookup("dir").Changed = false
	}()

	config.Cfg = &config.Config{
		WorkspacesRoot: t.TempDir(),
	}

	tmpDir := t.TempDir()
	bookDir := filepath.Join(tmpDir, "preview-book")
	m, err := pipeline.Initiate(pipeline.InitiateOptions{
		OutputDir:       bookDir,
		Theme:           "Preview Theme",
		TargetPageCount: 2,
		NoBrainstorm:    true,
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	_ = m

	rootHeadless = true
	t.Setenv("PITHOS_HEADLESS", "1")

	previewDir = bookDir
	_ = previewCmd.Flags().Set("dir", bookDir)
	previewCmd.Flags().Lookup("dir").Changed = true

	// preview should generate assets and succeed without opening browser
	err = previewCmd.RunE(previewCmd, []string{})
	if err != nil {
		t.Fatalf("previewCmd failed: %v", err)
	}

	previewHTML := filepath.Join(bookDir, "web_preview", "preview.html")
	if _, statErr := os.Stat(previewHTML); statErr != nil {
		t.Errorf("expected preview.html to exist, got stat error: %v", statErr)
	}
}

func TestAssembleCmd_Headless(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origDir := assembleDir
	origSilent := assembleSilent
	origDryRun := rootDryRun
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		assembleDir = origDir
		assembleSilent = origSilent
		rootDryRun = origDryRun
		_ = assembleCmd.Flags().Set("dir", "")
		assembleCmd.Flags().Lookup("dir").Changed = false
	}()

	tmpDir := t.TempDir()
	bookDir := filepath.Join(tmpDir, "assemble-book")
	m, err := pipeline.Initiate(pipeline.InitiateOptions{
		OutputDir:       bookDir,
		Theme:           "Assemble Theme",
		TargetPageCount: 80,
		NoBrainstorm:    true,
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	m.Progress.Pages = make([]manifest.PageState, 80)
	if err = m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	rootHeadless = true
	rootDryRun = true
	t.Setenv("PITHOS_HEADLESS", "1")

	assembleDir = bookDir
	_ = assembleCmd.Flags().Set("dir", bookDir)
	assembleCmd.Flags().Lookup("dir").Changed = true

	err = assembleCmd.RunE(assembleCmd, []string{})
	if err != nil {
		t.Fatalf("assembleCmd failed in headless mode: %v", err)
	}
}

func TestPersistentPreRunE_InitializesPipelineHeadlessMode(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origInteractive := rootInteractive
	origPipelineHeadless := pipeline.HeadlessMode
	origPipelineInteractive := pipeline.InteractiveMode
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		rootInteractive = origInteractive
		pipeline.HeadlessMode = origPipelineHeadless
		pipeline.InteractiveMode = origPipelineInteractive
	}()

	rootHeadless = true
	rootNonInteractive = false
	rootInteractive = false
	t.Setenv("PITHOS_HEADLESS", "1")

	err := rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	if !pipeline.HeadlessMode {
		t.Error("expected pipeline.HeadlessMode to be true after PersistentPreRunE")
	}
	if pipeline.InteractiveMode {
		t.Error("expected pipeline.InteractiveMode to be false when headless")
	}
}

func TestPersistentPreRunE_ConflictingFlags(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origInteractive := rootInteractive
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		rootInteractive = origInteractive
	}()

	rootHeadless = true
	rootInteractive = true
	err := rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --headless/--non-interactive and --interactive") {
		t.Errorf("expected error for conflicting flags, got %v", err)
	}

	rootHeadless = false
	rootNonInteractive = true
	rootInteractive = true
	err = rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "cannot specify both --headless/--non-interactive and --interactive") {
		t.Errorf("expected error for conflicting flags, got %v", err)
	}
}

func TestPersistentPreRunE_FlagPrecedenceOverEnv(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origInteractive := rootInteractive
	origPipelineHeadless := pipeline.HeadlessMode
	origPipelineInteractive := pipeline.InteractiveMode
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		rootInteractive = origInteractive
		pipeline.HeadlessMode = origPipelineHeadless
		pipeline.InteractiveMode = origPipelineInteractive
	}()

	// 1. --interactive flag overrides PITHOS_HEADLESS=1 env var
	rootHeadless = false
	rootNonInteractive = false
	rootInteractive = true
	t.Setenv("PITHOS_HEADLESS", "1")
	t.Setenv("PITHOS_INTERACTIVE", "")

	err := rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}
	if pipeline.HeadlessMode {
		t.Error("expected pipeline.HeadlessMode == false when --interactive is passed despite PITHOS_HEADLESS=1")
	}
	if !pipeline.InteractiveMode {
		t.Error("expected pipeline.InteractiveMode == true when --interactive is passed")
	}

	// 2. --headless flag overrides PITHOS_INTERACTIVE=1 env var
	rootHeadless = true
	rootNonInteractive = false
	rootInteractive = false
	t.Setenv("PITHOS_HEADLESS", "")
	t.Setenv("PITHOS_INTERACTIVE", "1")

	err = rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}
	if !pipeline.HeadlessMode {
		t.Error("expected pipeline.HeadlessMode == true when --headless is passed despite PITHOS_INTERACTIVE=1")
	}
	if pipeline.InteractiveMode {
		t.Error("expected pipeline.InteractiveMode == false when --headless is passed")
	}
}
