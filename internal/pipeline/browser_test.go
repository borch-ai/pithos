package pipeline

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/powerword/pkg/telemetry"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOpenBrowserBehavior(t *testing.T) {
	// Backup the original openBrowserFunc and restore it after test
	origFunc := openBrowserFunc
	defer func() { openBrowserFunc = origFunc }()

	var calledURL string
	var callCount int
	openBrowserFunc = func(ctx context.Context, urlStr string) error {
		calledURL = urlStr
		callCount++
		return nil
	}

	err := openBrowser(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected call count 1, got %d", callCount)
	}
	if calledURL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %q", calledURL)
	}
}

func TestTriggerBrowserOpen_Error(t *testing.T) {
	origFunc := openBrowserFunc
	defer func() { openBrowserFunc = origFunc }()

	openBrowserFunc = func(ctx context.Context, urlStr string) error {
		return errors.New("mock browser launch error")
	}

	// This should run without panic or failure, logging to stderr
	triggerBrowserOpen(context.Background(), "https://example.com")
}

func TestDefaultOpenBrowser_AllPlatforms(t *testing.T) {
	origExec := execCommandContext
	origGOOS := goos
	defer func() {
		execCommandContext = origExec
		goos = origGOOS
	}()

	platforms := []struct {
		osName      string
		expectedCmd string
	}{
		{"darwin", "open"},
		{"windows", "rundll32"},
		{"linux", "xdg-open"},
		{"other", "xdg-open"},
	}

	for _, p := range platforms {
		t.Run(p.osName, func(t *testing.T) {
			goos = p.osName
			var calledName string
			execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				calledName = name
				// Execute the standard Unix "true" command to mock successful startup/termination
				return exec.CommandContext(ctx, "true")
			}

			err := defaultOpenBrowser(context.Background(), "https://example.com")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calledName != p.expectedCmd {
				t.Errorf("expected command %q for OS %q, got %q", p.expectedCmd, p.osName, calledName)
			}
		})
	}
}

//nolint:funlen // TestAssemble_BrowserOpen coordinates multiple subtests and mock servers
func TestAssemble_BrowserOpen(t *testing.T) {
	origFunc := openBrowserFunc
	defer func() { openBrowserFunc = origFunc }()

	tmpDir, err := os.MkdirTemp("", "pithos-assemble-browser-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Parody Theme",
		TargetPageCount: 80,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	m.Progress.Pages = make([]manifest.PageState, 80)
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	t.Run("Silent true does not call browser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
		_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
		defer cleanupKDP()

		clientTypst, serverTypst := mcpsdk.NewInMemoryTransports()
		_, cleanupTypst := setupMockTypstServer(t, ctx, serverTypst)
		defer cleanupTypst()

		clientPDFCheck, serverPDFCheck := mcpsdk.NewInMemoryTransports()
		_, cleanupPDFCheck := setupMockPDFCheckServer(t, ctx, serverPDFCheck, true, nil, nil)
		defer cleanupPDFCheck()

		called := false
		openBrowserFunc = func(ctx context.Context, urlStr string) error {
			called = true
			return nil
		}

		opts := AssembleOptions{
			InputDir:          tmpDir,
			Format:            "paperback",
			Bleed:             true,
			Silent:            true,
			KDPMathTransport:  clientKDP,
			TypstTransport:    clientTypst,
			PDFCheckTransport: clientPDFCheck,
		}
		_, err = Assemble(ctx, opts)
		if err != nil {
			t.Fatalf("Assemble failed: %v", err)
		}
		if called {
			t.Error("expected browser NOT to be opened when Silent is true")
		}
	})

	t.Run("Silent false calls browser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
		_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
		defer cleanupKDP()

		clientTypst, serverTypst := mcpsdk.NewInMemoryTransports()
		_, cleanupTypst := setupMockTypstServer(t, ctx, serverTypst)
		defer cleanupTypst()

		clientPDFCheck, serverPDFCheck := mcpsdk.NewInMemoryTransports()
		_, cleanupPDFCheck := setupMockPDFCheckServer(t, ctx, serverPDFCheck, true, nil, nil)
		defer cleanupPDFCheck()

		called := false
		var calledURL string
		openBrowserFunc = func(ctx context.Context, urlStr string) error {
			called = true
			calledURL = urlStr
			return nil
		}

		opts := AssembleOptions{
			InputDir:          tmpDir,
			Format:            "paperback",
			Bleed:             true,
			Silent:            false,
			KDPMathTransport:  clientKDP,
			TypstTransport:    clientTypst,
			PDFCheckTransport: clientPDFCheck,
		}
		_, err = Assemble(ctx, opts)
		if err != nil {
			t.Fatalf("Assemble failed: %v", err)
		}
		if !called {
			t.Error("expected browser to be opened when Silent is false")
		}
		expectedPrefix := "file:///"
		if !strings.HasPrefix(calledURL, expectedPrefix) {
			t.Errorf("expected URL to start with %q, got %q", expectedPrefix, calledURL)
		}
		if !strings.Contains(calledURL, "web_preview/preview.html") {
			t.Errorf("expected URL to contain 'web_preview/preview.html', got %q", calledURL)
		}
	})
}

//nolint:funlen,gocognit // TestBrew_BrowserOpen contains extensive mock setups and multiple subtests
func TestBrew_BrowserOpen(t *testing.T) {
	origFunc := openBrowserFunc
	defer func() { openBrowserFunc = origFunc }()

	tmpDir, err := os.MkdirTemp("", "pithos-brew-browser-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Write dummy image to be copied
	dummySourceImage := filepath.Join(tmpDir, "source.png")
	if writeErr := os.WriteFile(dummySourceImage, []byte("fake-image-bytes"), 0600); writeErr != nil {
		t.Fatalf("failed to write source image: %v", writeErr)
	}

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Original Theme",
		Style:           "original-style --sref http://example.com/original.png",
		TargetPageCount: 2,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	mockStanzas := []string{"Stanza 1 text", "Stanza 2 text"}

	t.Run("Silent true does not call browser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
		_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
		defer cleanupMCP()

		cloudClientTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/uploaded_character.png")
		defer cloudCleanup()

		mockLLMClient := &mockLLM{
			stanzas: mockStanzas,
			usage: telemetry.TokenUsage{
				InputTokens:  1000,
				OutputTokens: 2000,
				CachedTokens: 500,
			},
		}

		called := false
		openBrowserFunc = func(ctx context.Context, urlStr string) error {
			called = true
			return nil
		}

		optsBrew := BrewOptions{
			OutputDir:         tmpDir,
			Theme:             "Overridden Theme",
			Style:             "new-style",
			MCPTransport:      clientTransport,
			CloudMCPTransport: cloudClientTransport,
			LLM:               mockLLMClient,
			Silent:            true,
		}

		err = Brew(ctx, optsBrew)
		if err != nil {
			t.Fatalf("Brew failed: %v", err)
		}
		if called {
			t.Error("expected browser NOT to be opened when Silent is true")
		}
	})

	t.Run("Silent false calls browser", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
		_, cleanupMCP := setupMockImageGenServer(t, ctx, serverTransport, dummySourceImage)
		defer cleanupMCP()

		cloudClientTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/uploaded_character.png")
		defer cloudCleanup()

		mockLLMClient := &mockLLM{
			stanzas: mockStanzas,
			usage: telemetry.TokenUsage{
				InputTokens:  1000,
				OutputTokens: 2000,
				CachedTokens: 500,
			},
		}

		called := false
		var calledURL string
		openBrowserFunc = func(ctx context.Context, urlStr string) error {
			called = true
			calledURL = urlStr
			return nil
		}

		// Reset manifest pages statuses to pending for rerun
		mCheck, err := manifest.LoadManifest(filepath.Join(tmpDir, "manifest.json"))
		if err != nil {
			t.Fatalf("failed to load manifest: %v", err)
		}
		for i := range mCheck.Progress.Pages {
			mCheck.Progress.Pages[i].Status = manifest.StatusPending
			mCheck.Progress.Pages[i].ImagePath = ""
		}
		if err = mCheck.Save(); err != nil {
			t.Fatalf("failed to save manifest: %v", err)
		}

		optsBrew := BrewOptions{
			OutputDir:         tmpDir,
			Theme:             "Overridden Theme",
			Style:             "new-style",
			MCPTransport:      clientTransport,
			CloudMCPTransport: cloudClientTransport,
			LLM:               mockLLMClient,
			Silent:            false,
		}

		err = Brew(ctx, optsBrew)
		if err != nil {
			t.Fatalf("Brew failed: %v", err)
		}
		if !called {
			t.Error("expected browser to be opened when Silent is false")
		}
		expectedPrefix := "file:///"
		if !strings.HasPrefix(calledURL, expectedPrefix) {
			t.Errorf("expected URL to start with %q, got %q", expectedPrefix, calledURL)
		}
		if !strings.Contains(calledURL, "web_preview/preview.html") {
			t.Errorf("expected URL to contain 'web_preview/preview.html', got %q", calledURL)
		}
	})
}
