package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
)

//nolint:gocognit,funlen // Test function performs multiple JSON validations and structural checks
func TestGenerateWebPreview(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-preview-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a mock manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Test Theme"
	m.BookProperties.Style = "minimalist"
	m.BookProperties.CharacterProfile = "A lonely robot"
	m.BookProperties.TrimSize = "6x9"
	m.BookProperties.Format = "paperback"
	m.KDPLayout = manifest.KDPLayout{
		Bleed:      0.125,
		MarginSize: 0.75,
	}
	m.Progress.Pages = []manifest.PageState{
		{
			PageIndex:          1,
			Status:             manifest.StatusCompleted,
			Text:               "Line 1\nLine 2",
			ImagePath:          "images/page_1.png",
			IllustrationPrompt: "Robot sitting under a tree",
		},
		{
			PageIndex:          2,
			Status:             manifest.StatusCompleted,
			Text:               "Line 3\nLine 4",
			ImagePath:          "",
			IllustrationPrompt: "",
		},
	}

	if saveErr := m.Save(); saveErr != nil {
		t.Fatalf("failed to save mock manifest: %v", saveErr)
	}

	// Generate preview
	if genErr := GenerateWebPreview(tmpDir, m); genErr != nil {
		t.Fatalf("GenerateWebPreview failed: %v", genErr)
	}

	// Assert files exist
	previewDir := filepath.Join(tmpDir, "web_preview")
	files := []string{"preview.html", "preview.css", "preview.js", "data.js"}
	for _, file := range files {
		filePath := filepath.Join(previewDir, file)
		if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
			t.Errorf("expected file %s to be generated, but it does not exist", file)
		}
	}

	// Read and validate data.js contents
	//nolint:gosec // ReadFile path is constructed inside test temp directory
	dataJSBytes, err := os.ReadFile(filepath.Join(previewDir, "data.js"))
	if err != nil {
		t.Fatalf("failed to read generated data.js: %v", err)
	}
	dataJSStr := string(dataJSBytes)

	prefix := "const bookData = "
	if !strings.HasPrefix(dataJSStr, prefix) {
		t.Fatalf("expected data.js to start with %q", prefix)
	}
	if !strings.HasSuffix(dataJSStr, ";\n") && !strings.HasSuffix(dataJSStr, ";") {
		t.Fatalf("expected data.js to end with a semicolon and newline")
	}

	jsonStr := strings.TrimSuffix(strings.TrimPrefix(dataJSStr, prefix), ";\n")
	jsonStr = strings.TrimSuffix(jsonStr, ";")

	var parsedData previewData
	if err := json.Unmarshal([]byte(jsonStr), &parsedData); err != nil {
		t.Fatalf("failed to unmarshal JSON from data.js: %v (raw: %q)", err, jsonStr)
	}

	if parsedData.Theme != "Test Theme" {
		t.Errorf("expected theme 'Test Theme', got %q", parsedData.Theme)
	}
	if parsedData.Style != "minimalist" {
		t.Errorf("expected style 'minimalist', got %q", parsedData.Style)
	}
	if parsedData.CharacterProfile != "A lonely robot" {
		t.Errorf("expected characterProfile 'A lonely robot', got %q", parsedData.CharacterProfile)
	}
	if parsedData.TrimSize != "6x9" {
		t.Errorf("expected trimSize '6x9', got %q", parsedData.TrimSize)
	}
	if parsedData.Format != "paperback" {
		t.Errorf("expected format 'paperback', got %q", parsedData.Format)
	}
	if parsedData.KDPLayout.Bleed != 0.125 {
		t.Errorf("expected bleed 0.125, got %f", parsedData.KDPLayout.Bleed)
	}
	if parsedData.KDPLayout.MarginSize != 0.75 {
		t.Errorf("expected marginSize 0.75, got %f", parsedData.KDPLayout.MarginSize)
	}

	if len(parsedData.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(parsedData.Pages))
	}

	p1 := parsedData.Pages[0]
	if p1.PageIndex != 1 {
		t.Errorf("expected page index 1, got %d", p1.PageIndex)
	}
	if p1.Text != "Line 1\nLine 2" {
		t.Errorf("expected page text 'Line 1\\nLine 2', got %q", p1.Text)
	}
	if p1.ImagePath != "../images/page_1.png" {
		t.Errorf("expected imagePath '../images/page_1.png', got %q", p1.ImagePath)
	}
	if p1.IllustrationPrompt != "Robot sitting under a tree" {
		t.Errorf("expected prompt 'Robot sitting under a tree', got %q", p1.IllustrationPrompt)
	}

	p2 := parsedData.Pages[1]
	if p2.PageIndex != 2 {
		t.Errorf("expected page index 2, got %d", p2.PageIndex)
	}
	if p2.ImagePath != "" {
		t.Errorf("expected page 2 imagePath empty, got %q", p2.ImagePath)
	}
}

func TestGenerateWebPreview_Errors(t *testing.T) {
	m := manifest.NewManifest("")
	m.BookProperties.Theme = "Test Theme"

	// 1. MkdirAll error: create a file where web_preview folder should be created
	tmpDir1, err := os.MkdirTemp("", "pithos-preview-err1-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir1) }()

	// Write a file named web_preview to block directory creation
	blockingFile := filepath.Join(tmpDir1, "web_preview")
	if writeErr := os.WriteFile(blockingFile, []byte(""), 0600); writeErr != nil {
		t.Fatalf("failed to write blocking file: %v", writeErr)
	}

	err = GenerateWebPreview(tmpDir1, m)
	if err == nil {
		t.Error("expected error when MkdirAll fails, got nil")
	}

	// 2a. WriteFile error for html
	tmpHtml, err := os.MkdirTemp("", "pithos-preview-html-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpHtml) }()
	_ = os.MkdirAll(filepath.Join(tmpHtml, "web_preview", "preview.html"), 0750)
	if err = GenerateWebPreview(tmpHtml, m); err == nil {
		t.Error("expected error when writing html fails, got nil")
	}

	// 2b. WriteFile error for css
	tmpCss, err := os.MkdirTemp("", "pithos-preview-css-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpCss) }()
	_ = os.MkdirAll(filepath.Join(tmpCss, "web_preview", "preview.css"), 0750)
	if err = GenerateWebPreview(tmpCss, m); err == nil {
		t.Error("expected error when writing css fails, got nil")
	}

	// 2c. WriteFile error for js
	tmpJs, err := os.MkdirTemp("", "pithos-preview-js-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpJs) }()
	_ = os.MkdirAll(filepath.Join(tmpJs, "web_preview", "preview.js"), 0750)
	if err = GenerateWebPreview(tmpJs, m); err == nil {
		t.Error("expected error when writing js fails, got nil")
	}

	// 2d. WriteFile error for data.js
	tmpData, err := os.MkdirTemp("", "pithos-preview-data-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpData) }()
	_ = os.MkdirAll(filepath.Join(tmpData, "web_preview", "data.js"), 0750)
	if err = GenerateWebPreview(tmpData, m); err == nil {
		t.Error("expected error when writing data.js fails, got nil")
	}
}
