package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
)

func TestBrew_Integration_RealSubprocess(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := buildImageGenBinary(t, tempDir)

	imageBytes := []byte("integration-test-image-data")
	downloadServer, openaiServer := setupMockOpenAIServer(t, imageBytes)
	defer downloadServer.Close()
	defer openaiServer.Close()

	restoreConfig := configureTestEnvironment(t, tempDir, binaryPath, openaiServer.URL)
	defer restoreConfig()

	// 4. Initialize Pithos Workspace
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Integration Test Theme",
		TargetPageCount: 3,
	}
	_, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// Mock LLM Client to generate stanzas
	mockStanzas := []string{"Stanza 1", "Stanza 2", "Stanza 3"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	// 5. Run Brew with Concurrency 3
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	optsBrew := BrewOptions{
		OutputDir:   tempDir,
		LLM:         mockLLMClient,
		Concurrency: 3,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew integration failed: %v", err)
	}

	// 6. Verify outputs
	m, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if len(m.Progress.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(m.Progress.Pages))
	}

	for _, page := range m.Progress.Pages {
		if page.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d to be completed, got %q", page.PageIndex, page.Status)
		}
		if page.ImagePath != fmt.Sprintf("images/page_%d.png", page.PageIndex) {
			t.Errorf("expected image path 'images/page_%d.png', got %q", page.PageIndex, page.ImagePath)
		}

		// Verify image file actually exists and has the correct mock data
		fullPath := filepath.Join(tempDir, page.ImagePath)
		//nolint:gosec // G304: fullPath is constructed within the temp directory
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("failed to read generated image file %s: %v", fullPath, err)
		}
		if string(data) != "integration-test-image-data" {
			t.Errorf("expected image content 'integration-test-image-data', got %q", string(data))
		}
	}
}

func TestBrew_Integration_ReviewFlow(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := buildImageGenBinary(t, tempDir)

	imageBytes := []byte("real-subprocess-image-data")
	downloadServer, openaiServer := setupMockOpenAIServer(t, imageBytes)
	defer downloadServer.Close()
	defer openaiServer.Close()

	restoreConfig := configureTestEnvironment(t, tempDir, binaryPath, openaiServer.URL)
	defer restoreConfig()

	// 4. Initialize Pithos Workspace
	_, err := Initiate(InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Integration Review Theme",
		TargetPageCount: 2,
	})
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	mockStanzas := []string{"Stanza 1 original text", "Stanza 2 original text"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	// 5. Run Brew with Review = true (should generate stanzas, export manuscript.md and exit)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = Brew(ctx, BrewOptions{
		OutputDir: tempDir,
		LLM:       mockLLMClient,
		Review:    true,
	})
	if err == nil {
		t.Fatal("expected Brew to return review pause error, got nil")
	}
	if !strings.Contains(err.Error(), "review mode active") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify manuscript.md exists
	manuscriptPath := filepath.Join(tempDir, "manuscript.md")
	if _, statErr := os.Stat(manuscriptPath); os.IsNotExist(statErr) {
		t.Fatal("expected manuscript.md to exist")
	}

	// 6. Edit manuscript.md
	//nolint:gosec // manuscriptPath is constructed in temp test directory
	data, readErr := os.ReadFile(manuscriptPath)
	if readErr != nil {
		t.Fatalf("failed to read manuscript.md: %v", readErr)
	}
	editedContent := strings.Replace(string(data), "Stanza 2 original text", "Stanza 2 edited integration text", 1)
	//nolint:gosec // manuscriptPath is constructed in temp test directory
	if writeErr := os.WriteFile(manuscriptPath, []byte(editedContent), 0600); writeErr != nil {
		t.Fatalf("failed to write manuscript.md: %v", writeErr)
	}

	// 7. Run Brew with Review = false (should sync edits and run imagegen using real subprocess)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()

	err = Brew(ctx2, BrewOptions{
		OutputDir:   tempDir,
		LLM:         mockLLMClient,
		Review:      false,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatalf("expected Brew to succeed on resume/import run, got %v", err)
	}

	// 8. Verify final manifest and images
	m, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if m.Progress.Pages[1].Text != "Stanza 2 edited integration text" {
		t.Errorf("expected page 2 text to be updated, got %q", m.Progress.Pages[1].Text)
	}
	for _, page := range m.Progress.Pages {
		if page.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d to be completed, got %q", page.PageIndex, page.Status)
		}

		fullPath := filepath.Join(tempDir, page.ImagePath)
		//nolint:gosec // G304: fullPath is constructed within the temp directory
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("failed to read image file %s: %v", fullPath, err)
		}
		if string(data) != "real-subprocess-image-data" {
			t.Errorf("expected image content 'real-subprocess-image-data', got %q", string(data))
		}
	}
}

func buildImageGenBinary(t *testing.T, tempDir string) string {
	t.Helper()
	binaryPath := filepath.Join(tempDir, "pw-mcp-imagegen")

	siblingPath := "../../../powerword/cmd/pw-mcp-imagegen"
	//nolint:nestif // test binary compilation logic can have nested setup branches
	if _, statErr := os.Stat(siblingPath); statErr == nil {
		t.Logf("Building pw-mcp-imagegen from sibling repository: %s", siblingPath)
		absBinary, err := filepath.Abs(binaryPath)
		if err != nil {
			t.Fatalf("failed to get absolute binary path: %v", err)
		}
		//nolint:gosec // siblingPath and binaryPath are constructed inside test dir
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", absBinary, ".")
		cmd.Dir = siblingPath
		if buildErr := cmd.Run(); buildErr != nil {
			t.Fatalf("failed to build pw-mcp-imagegen: %v", buildErr)
		}
	} else {
		path, err := exec.LookPath("pw-mcp-imagegen")
		if err != nil {
			t.Skip("pw-mcp-imagegen binary not found and sibling powerword repo not found")
		}
		binaryPath = path
		t.Logf("Using existing system pw-mcp-imagegen binary: %s", binaryPath)
	}
	return binaryPath
}

func setupMockOpenAIServer(t *testing.T, imageBytes []byte) (downloadServer *httptest.Server, openaiServer *httptest.Server) {
	t.Helper()
	downloadServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))

	openaiServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Data []struct {
				URL string `json:"url"`
			} `json:"data"`
		}{
			Data: []struct {
				URL string `json:"url"`
			}{
				{URL: downloadServer.URL + "/image.png"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	return downloadServer, openaiServer
}

func configureTestEnvironment(t *testing.T, tempDir string, binaryPath string, openaiURL string) func() {
	t.Helper()
	t.Setenv("OPENAI_BASE_URL", openaiURL)
	t.Setenv("POWERWORD_WORKSPACE_ROOT", tempDir)

	//nolint:gosec // dummy key used for mock test configuration
	pwTOML := `
[api_keys]
openai = "dummy-key"
[plugins.imagegen]
backend = "openai"
`
	if err := os.WriteFile(filepath.Join(tempDir, "powerword.toml"), []byte(pwTOML), 0600); err != nil {
		t.Fatalf("failed to write powerword.toml: %v", err)
	}

	origCfg := config.Cfg
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			ImageGenPath: binaryPath,
		},
		API: config.APIConfig{
			GeminiKey: "mock-gemini-key",
		},
	}

	return func() {
		config.Cfg = origCfg
	}
}
