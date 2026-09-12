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

//nolint:funlen // TestIsHeadless_Detection coordinates multiple subtests verifying detection vectors
func TestIsHeadless_Detection(t *testing.T) {
	// Save and restore state
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origIsStdinTTY := isStdinTTY
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
	}()

	t.Run("Flag --headless", func(t *testing.T) {
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")
		isStdinTTY = func() bool { return true }
		rootHeadless = true
		rootNonInteractive = false

		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when rootHeadless is true")
		}
	})

	t.Run("Flag --non-interactive", func(t *testing.T) {
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")
		isStdinTTY = func() bool { return true }
		rootHeadless = false
		rootNonInteractive = true

		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when rootNonInteractive is true")
		}
	})

	t.Run("Env PITHOS_HEADLESS", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
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
			if got := IsHeadless(); got != tc.expected {
				t.Errorf("PITHOS_HEADLESS=%q: expected IsHeadless() == %v, got %v", tc.val, tc.expected, got)
			}
		}
	})

	t.Run("Env CI", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
		isStdinTTY = func() bool { return true }
		t.Setenv("PITHOS_HEADLESS", "")

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

	t.Run("Non-TTY stdin", func(t *testing.T) {
		rootHeadless = false
		rootNonInteractive = false
		t.Setenv("PITHOS_HEADLESS", "")
		t.Setenv("CI", "")

		isStdinTTY = func() bool { return false }
		if !IsHeadless() {
			t.Error("expected IsHeadless() == true when isStdinTTY returns false")
		}

		isStdinTTY = func() bool { return true }
		if IsHeadless() {
			t.Error("expected IsHeadless() == false when isStdinTTY returns true and no env/flags set")
		}
	})
}

func TestInitiateCmd_HeadlessValidation(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origIsStdinTTY := isStdinTTY
	origTheme := initiateTheme
	origDir := initiateDir
	origOutput := initiateOutput
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
		initiateTheme = origTheme
		initiateDir = origDir
		initiateOutput = origOutput
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
	initiateNoBrainstorm = true
	rootDryRun = true

	err = initiateCmd.RunE(initiateCmd, []string{})
	if err != nil {
		t.Errorf("expected initiate to succeed in headless mode with valid args, got %v", err)
	}
}

func TestBrewCmd_HeadlessValidation(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origIsStdinTTY := isStdinTTY
	origSelect := brewSelect
	origPages := brewPagesStr
	origReview := brewReview
	origTUI := brewTUI
	origSilent := brewSilent
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
		brewSelect = origSelect
		brewPagesStr = origPages
		brewReview = origReview
		brewTUI = origTUI
		brewSilent = origSilent
		_ = brewCmd.Flags().Set("pages", "")
		brewCmd.Flags().Lookup("pages").Changed = false
	}()

	rootHeadless = true
	isStdinTTY = func() bool { return true }
	t.Setenv("PITHOS_HEADLESS", "1")

	// 1. --select in headless mode fails fast with clear guidance
	brewSelect = true
	_ = brewCmd.Flags().Set("pages", "")
	brewCmd.Flags().Lookup("pages").Changed = false

	err := brewCmd.RunE(brewCmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "interactive page selection (--select) is not supported in headless mode") {
		t.Errorf("expected --select error in headless mode, got %v", err)
	}
}

func TestPreviewCmd_HeadlessSuppression(t *testing.T) {
	origHeadless := rootHeadless
	origNonInteractive := rootNonInteractive
	origIsStdinTTY := isStdinTTY
	origCfg := config.Cfg
	origDir := previewDir
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
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
	isStdinTTY = func() bool { return true }
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
	origIsStdinTTY := isStdinTTY
	origDir := assembleDir
	origSilent := assembleSilent
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
		assembleDir = origDir
		assembleSilent = origSilent
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
	isStdinTTY = func() bool { return false }
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
	origIsStdinTTY := isStdinTTY
	origPipelineHeadless := pipeline.HeadlessMode
	defer func() {
		rootHeadless = origHeadless
		rootNonInteractive = origNonInteractive
		isStdinTTY = origIsStdinTTY
		pipeline.HeadlessMode = origPipelineHeadless
	}()

	rootHeadless = true
	isStdinTTY = func() bool { return true }
	t.Setenv("PITHOS_HEADLESS", "1")

	err := rootCmd.PersistentPreRunE(rootCmd, []string{})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	if !pipeline.HeadlessMode {
		t.Error("expected pipeline.HeadlessMode to be true after PersistentPreRunE")
	}
}
