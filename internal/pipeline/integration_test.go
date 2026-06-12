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
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
)

//nolint:funlen // Integration test checks multiple pipeline setup steps and requires mock servers
func TestBrew_Integration_RealSubprocess(t *testing.T) {
	// 1. Locate/build pw-mcp-imagegen binary
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "pw-mcp-imagegen")

	// Try building from sibling powerword repository
	siblingPath := "../powerword/cmd/pw-mcp-imagegen"
	if _, err := os.Stat(siblingPath); err == nil {
		t.Logf("Building pw-mcp-imagegen from sibling repository: %s", siblingPath)
		//nolint:gosec // G204: siblingPath and binaryPath are constructed inside test dir
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, siblingPath)
		if buildErr := cmd.Run(); buildErr != nil {
			t.Fatalf("failed to build pw-mcp-imagegen: %v", buildErr)
		}
	} else {
		// Fallback to checking system PATH
		path, err := exec.LookPath("pw-mcp-imagegen")
		if err != nil {
			t.Skip("pw-mcp-imagegen binary not found and sibling powerword repo not found")
		}
		binaryPath = path
		t.Logf("Using existing system pw-mcp-imagegen binary: %s", binaryPath)
	}

	// 2. Setup mock OpenAI API server
	imageBytes := []byte("integration-test-image-data")
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer downloadServer.Close()

	// Mock OpenAI generation endpoint
	openaiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer openaiServer.Close()

	// 3. Configure environment and global config for Pithos
	t.Setenv("OPENAI_BASE_URL", openaiServer.URL)
	t.Setenv("POWERWORD_WORKSPACE_ROOT", tempDir) // set workspace root for pw-mcp-imagegen config loading

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
	defer func() { config.Cfg = origCfg }()

	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			ImageGenPath: binaryPath,
		},
		API: config.APIConfig{
			GeminiKey: "mock-gemini-key",
		},
	}

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
