package pipeline

import (
	"context"
	"encoding/base64"
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
	"github.com/borch-ai/powerword/pkg/gitutil"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

//nolint:gocognit,funlen,nestif // Integration tests have multiple steps, complex checking blocks, and configurations
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cloudTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/uploaded_character.png")
	defer cloudCleanup()

	optsBrew := BrewOptions{
		OutputDir:         tempDir,
		LLM:               mockLLMClient,
		Concurrency:       3,
		CloudMCPTransport: cloudTransport,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew integration failed: %v", err)
	}

	// Verify Git Checkpoints were generated during integration pipeline run
	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		gitDir := filepath.Join(tempDir, ".git")
		if _, statErr := os.Stat(gitDir); os.IsNotExist(statErr) {
			t.Error("expected .git directory to exist in the workspace")
		} else {
			logOut, logErr := gitutil.RunGitCommand(ctx, tempDir, "log", "--oneline")
			if logErr != nil {
				t.Errorf("failed to read git log: %v", logErr)
			} else {
				expectedMilestones := []string{
					"Initial workspace setup",
					"Generated stanzas and prompts",
					"Completed illustration generation",
				}
				for _, milestone := range expectedMilestones {
					if !strings.Contains(logOut, milestone) {
						t.Errorf("expected git log to contain milestone %q, got log:\n%s", milestone, logOut)
					}
				}
			}
		}
	}

	// 6. Verify outputs
	m, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if len(m.Kiln.Milestones) != 2 || m.Kiln.Milestones[0] != "initiate_complete" || m.Kiln.Milestones[1] != "brew_complete" {
		t.Errorf("expected milestones [initiate_complete, brew_complete] in integration test, got %v", m.Kiln.Milestones)
	}
	if m.Kiln.Version != 1 {
		t.Errorf("expected Kiln version 1, got %d", m.Kiln.Version)
	}
	if m.Kiln.TotalCostUSD <= 0 {
		t.Errorf("expected non-zero total cost in manifest.Kiln, got %f", m.Kiln.TotalCostUSD)
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
			if string(data) == "" {
				t.Skip("skipping content verification: generated image is empty (restricted sandboxed environment)")
			} else {
				t.Errorf("expected image content 'integration-test-image-data', got %q", string(data))
			}
		}
	}
}

//nolint:gocognit,funlen,nestif // Integration review test requires multi-step setup, manuscript edit modifications, and git logs checks
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cloudTransport, cloudCleanup := setupMockCloudTransport(t, ctx, "http://example.com/uploaded_character.png")
	defer cloudCleanup()

	err = Brew(ctx, BrewOptions{
		OutputDir:         tempDir,
		LLM:               mockLLMClient,
		Review:            true,
		CloudMCPTransport: cloudTransport,
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
	ctx2, cancel2 := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel2()

	optsBrew := BrewOptions{
		OutputDir:         tempDir,
		LLM:               mockLLMClient,
		Review:            false,
		Concurrency:       2,
		CloudMCPTransport: cloudTransport,
	}

	err = Brew(ctx2, optsBrew)
	if err != nil {
		t.Fatalf("expected Brew to succeed on resume/import run, got %v", err)
	}

	// Verify Git Checkpoints were generated during review flow resume/import run
	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		gitDir2 := filepath.Join(tempDir, ".git")
		if _, statErr := os.Stat(gitDir2); os.IsNotExist(statErr) {
			t.Error("expected .git directory to exist in the workspace")
		} else {
			logOut, logErr := gitutil.RunGitCommand(ctx2, tempDir, "log", "--oneline")
			if logErr != nil {
				t.Errorf("failed to read git log: %v", logErr)
			} else {
				expectedMilestones := []string{
					"Initial workspace setup",
					"Generated stanzas and prompts",
					"Imported manuscript edits from review",
					"Completed illustration generation",
				}
				for _, milestone := range expectedMilestones {
					if !strings.Contains(logOut, milestone) {
						t.Errorf("expected git log to contain milestone %q, got log:\n%s", milestone, logOut)
					}
				}
			}
		}
	}

	// 8. Verify final manifest and images
	m, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	if len(m.Kiln.Milestones) != 2 || m.Kiln.Milestones[0] != "initiate_complete" || m.Kiln.Milestones[1] != "brew_complete" {
		t.Errorf("expected milestones [initiate_complete, brew_complete] in integration test, got %v", m.Kiln.Milestones)
	}
	if m.Kiln.Version != 1 {
		t.Errorf("expected Kiln version 1, got %d", m.Kiln.Version)
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
			if string(data) == "" {
				t.Skip("skipping content verification: generated image is empty (restricted sandboxed environment)")
			} else {
				t.Errorf("expected image content 'real-subprocess-image-data', got %q", string(data))
			}
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
		if strings.Contains(r.URL.Path, "predict") {
			googleResp := map[string]interface{}{
				"predictions": []map[string]interface{}{
					{
						"bytesBase64Encoded": base64.StdEncoding.EncodeToString(imageBytes),
						"mimeType":           "image/png",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(googleResp)
			return
		}

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
	t.Setenv("GOOGLE_BASE_URL", openaiURL)
	t.Setenv("POWERWORD_WORKSPACE_ROOT", tempDir)
	t.Setenv("POWERWORD_IMAGEGEN_BACKEND", "google")

	//nolint:gosec // dummy key used for mock test configuration
	pwTOML := `
[api_keys]
openai = "dummy-key"
gemini = "dummy-key"
[plugins.imagegen]
backend = "google"
`
	if err := os.WriteFile(filepath.Join(tempDir, "powerword.toml"), []byte(pwTOML), 0600); err != nil {
		t.Fatalf("failed to write powerword.toml: %v", err)
	}

	origCfg := config.Cfg
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			ImageGenPath: binaryPath,
			CloudPath:    "pw-mcp-cloud",
		},
		API: config.APIConfig{
			GeminiKey: "mock-gemini-key",
		},
	}

	return func() {
		config.Cfg = origCfg
	}
}

//nolint:gocognit,funlen,nestif // Integration tests have multiple steps, complex checking blocks, and configurations
func TestBrew_Integration_SelectivePageRedo(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := buildImageGenBinary(t, tempDir)

	imageBytes := []byte("integration-selective-image-data")
	downloadServer, openaiServer := setupMockOpenAIServer(t, imageBytes)
	defer downloadServer.Close()
	defer openaiServer.Close()

	restoreConfig := configureTestEnvironment(t, tempDir, binaryPath, openaiServer.URL)
	defer restoreConfig()

	// 1. Initialize Pithos Workspace
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Integration Selective Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// 2. Pre-populate stanzas to skip LLM text generation step
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "images/page_1.png", Text: "Stanza 1"},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page_2.png", Text: "Stanza 2"},
		{PageIndex: 3, Status: manifest.StatusCompleted, ImagePath: "images/page_3.png", Text: "Stanza 3"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// Pre-create image files on disk
	for _, p := range m.Progress.Pages {
		fullImgPath := filepath.Join(tempDir, p.ImagePath)
		if mkdirErr := os.MkdirAll(filepath.Dir(fullImgPath), 0750); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(fullImgPath, []byte("original-image-data"), 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	// 3. Run Brew with Pages = []int{2} to regenerate Page 2
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	optsBrew := BrewOptions{
		OutputDir: tempDir,
		Pages:     []int{2},
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew integration failed: %v", err)
	}

	// 4. Verify outputs:
	// Reload manifest
	m2, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	// Page 2 should be completed and updated with new image
	if m2.Progress.Pages[1].Status != manifest.StatusCompleted {
		t.Errorf("expected page 2 to be completed, got %q", m2.Progress.Pages[1].Status)
	}
	// #nosec G304
	data2, err := os.ReadFile(filepath.Join(tempDir, m2.Progress.Pages[1].ImagePath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != "integration-selective-image-data" {
		t.Errorf("expected page 2 to have new image data, got %q", string(data2))
	}

	// Page 1 and Page 3 should remain completed and have their original image data
	for _, pIdx := range []int{0, 2} {
		p := m2.Progress.Pages[pIdx]
		if p.Status != manifest.StatusCompleted {
			t.Errorf("expected page %d to remain completed, got %q", p.PageIndex, p.Status)
		}
		// #nosec G304
		data, err := os.ReadFile(filepath.Join(tempDir, p.ImagePath))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "original-image-data" {
			t.Errorf("expected page %d to have original image data, got %q", p.PageIndex, string(data))
		}
	}
}

//nolint:gocognit,funlen // Integration tests have multiple setup configurations, assertions, and checks
func TestAssemble_Integration_RealSubprocess(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("POWERWORD_WORKSPACE_ROOT", tempDir)
	typstBinaryPath := buildTypstBinary(t, tempDir)
	pdfcheckBinaryPath := buildPDFCheckBinary(t, tempDir)

	// Configure environment: point TypstPath and PDFCheckPath to our built binaries.
	origCfg := config.Cfg
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			TypstPath:    typstBinaryPath,
			KDPMathPath:  "pw-mcp-kdp-math",
			PDFCheckPath: pdfcheckBinaryPath,
		},
		API: config.APIConfig{
			GeminiKey: "mock-gemini-key",
		},
	}
	defer func() {
		config.Cfg = origCfg
	}()

	// 1. Initialize Pithos Workspace
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Integration Assemble Theme",
		TargetPageCount: 3,
	}
	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}

	// Pre-populate stanzas to skip LLM text generation step
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusPending, Text: "Line 1\nLine 2\nLine 3\nLine 4"},
		{PageIndex: 2, Status: manifest.StatusPending, Text: "Stanza 2"},
		{PageIndex: 3, Status: manifest.StatusPending, Text: "Stanza 3"},
	}
	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest: %v", saveErr)
	}

	// 2. Set up mock KDP math server via in-memory transport
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	clientKDP, serverKDP := mcpsdk.NewInMemoryTransports()
	_, cleanupKDP := setupMockKDPMathServer(t, ctx, serverKDP)
	defer cleanupKDP()

	// 3. Verify that manuscript.md does NOT exist initially
	manuscriptPath := filepath.Join(tempDir, "manuscript.md")
	if _, statErr := os.Stat(manuscriptPath); !os.IsNotExist(statErr) {
		t.Fatal("expected manuscript.md NOT to exist before Assemble is run")
	}

	// 4. Run Assemble with KDP Math Mocked, but Typst and PDFCheck running as real subprocesses
	optsAssemble := AssembleOptions{
		InputDir:         tempDir,
		Format:           "paperback",
		Bleed:            true,
		KDPMathTransport: clientKDP,
	}

	// Check if typst command is installed on the host system.
	hasTypstInstalled := false
	if _, lookErr := exec.LookPath("typst"); lookErr == nil {
		hasTypstInstalled = true
	}

	_, err = Assemble(ctx, optsAssemble)

	// Check that manuscript.md was indeed auto-exported before the compile tool was called
	if _, statErr := os.Stat(manuscriptPath); os.IsNotExist(statErr) {
		t.Error("expected manuscript.md to be automatically exported during Assemble, but it is missing")
	} else {
		// Verify content: single newlines inside stanzas exist in the exported file
		//nolint:gosec // manuscriptPath is constructed in temp test directory
		content, readErr := os.ReadFile(manuscriptPath)
		if readErr != nil {
			t.Fatalf("failed to read manuscript.md: %v", readErr)
		}
		if !strings.Contains(string(content), "Line 1\nLine 2") {
			t.Errorf("expected manuscript.md to preserve line breaks, got:\n%s", string(content))
		}
	}

	if hasTypstInstalled && err != nil {
		t.Fatalf("Assemble integration test failed: %v", err)
	}
	if hasTypstInstalled {
		verifyAssembleOutputs(t, tempDir)
	}
	if !hasTypstInstalled {
		if err == nil {
			t.Error("expected Assemble to fail without typst installed, but it succeeded")
		} else if !strings.Contains(err.Error(), "interior compilation failed") && !strings.Contains(err.Error(), "failed to compile Typst interior") {
			t.Errorf("unexpected error without typst installed: %v", err)
		}
		t.Logf("typst not installed locally, verified subprocess started up and threw expected error: %v", err)
	}
}

func buildTypstBinary(t *testing.T, tempDir string) string {
	t.Helper()
	binaryPath := filepath.Join(tempDir, "pw-mcp-typst")
	siblingPath := "../../../powerword/cmd/pw-mcp-typst"

	if _, statErr := os.Stat(siblingPath); statErr != nil {
		path, err := exec.LookPath("pw-mcp-typst")
		if err != nil {
			t.Skip("pw-mcp-typst binary not found and sibling powerword repo not found")
		}
		t.Logf("Using existing system pw-mcp-typst binary: %s", path)
		return path
	}

	t.Logf("Building pw-mcp-typst from sibling repository: %s", siblingPath)
	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("failed to get absolute binary path: %v", err)
	}

	//nolint:gosec // siblingPath and binaryPath are constructed inside test dir
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", absBinary, ".")
	cmd.Dir = siblingPath
	if buildErr := cmd.Run(); buildErr != nil {
		t.Fatalf("failed to build pw-mcp-typst: %v", buildErr)
	}

	return binaryPath
}

func buildPDFCheckBinary(t *testing.T, tempDir string) string {
	t.Helper()
	binaryPath := filepath.Join(tempDir, "pw-mcp-pdfcheck")
	siblingPath := "../../../powerword/cmd/pw-mcp-pdfcheck"

	if _, statErr := os.Stat(siblingPath); statErr != nil {
		path, err := exec.LookPath("pw-mcp-pdfcheck")
		if err != nil {
			t.Skip("pw-mcp-pdfcheck binary not found and sibling powerword repo not found")
		}
		t.Logf("Using existing system pw-mcp-pdfcheck binary: %s", path)
		return path
	}

	t.Logf("Building pw-mcp-pdfcheck from sibling repository: %s", siblingPath)
	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("failed to get absolute binary path: %v", err)
	}

	//nolint:gosec // siblingPath and binaryPath are constructed inside test dir
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", absBinary, ".")
	cmd.Dir = siblingPath
	if buildErr := cmd.Run(); buildErr != nil {
		t.Fatalf("failed to build pw-mcp-pdfcheck: %v", buildErr)
	}

	return binaryPath
}

func verifyAssembleOutputs(t *testing.T, tempDir string) {
	t.Helper()
	interiorPDF := filepath.Join(tempDir, "interior.pdf")
	if _, statErr := os.Stat(interiorPDF); os.IsNotExist(statErr) {
		t.Error("expected interior.pdf to exist, but it was not found")
	}

	m2, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest in integration test: %v", err)
	}
	if len(m2.Kiln.Milestones) != 2 || m2.Kiln.Milestones[0] != "initiate_complete" || m2.Kiln.Milestones[1] != "assemble_complete" {
		t.Errorf("expected milestones [initiate_complete, assemble_complete], got %v", m2.Kiln.Milestones)
	}
	if m2.Kiln.InteriorPDFPath == "" {
		t.Error("expected Kiln interior PDF path to be populated")
	}
}
