package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/borch-ai/pithos/internal/manifest"
)

// GenerateWebPreview writes the static HTML, CSS, and JS visualizer files,
// along with a dynamic data.js file containing the serialized book manifest pages.
func GenerateWebPreview(outputDir string, m *manifest.Manifest) error {
	previewDir := filepath.Join(outputDir, "web_preview")
	if err := os.MkdirAll(previewDir, 0750); err != nil {
		return fmt.Errorf("failed to create web_preview directory: %w", err)
	}

	// 1. Write preview.html
	if err := os.WriteFile(filepath.Join(previewDir, "preview.html"), []byte(htmlTemplate), 0600); err != nil {
		return fmt.Errorf("failed to write preview.html: %w", err)
	}

	// 2. Write preview.css
	if err := os.WriteFile(filepath.Join(previewDir, "preview.css"), []byte(cssTemplate), 0600); err != nil {
		return fmt.Errorf("failed to write preview.css: %w", err)
	}

	// 3. Write preview.js
	if err := os.WriteFile(filepath.Join(previewDir, "preview.js"), []byte(jsTemplate), 0600); err != nil {
		return fmt.Errorf("failed to write preview.js: %w", err)
	}

	// 4. Generate data.js
	dataJS, err := generateDataJS(m)
	if err != nil {
		return fmt.Errorf("failed to generate book data JS: %w", err)
	}
	if err := os.WriteFile(filepath.Join(previewDir, "data.js"), []byte(dataJS), 0600); err != nil {
		return fmt.Errorf("failed to write data.js: %w", err)
	}

	return nil
}

type previewPage struct {
	PageIndex          int    `json:"pageIndex"`
	Text               string `json:"text"`
	ImagePath          string `json:"imagePath"`
	IllustrationPrompt string `json:"illustrationPrompt"`
	Layout             string `json:"layout,omitempty"`
}

type previewData struct {
	Theme            string             `json:"theme"`
	Style            string             `json:"style"`
	CharacterProfile string             `json:"characterProfile"`
	TrimSize         string             `json:"trimSize,omitempty"`
	Format           string             `json:"format,omitempty"`
	KDPLayout        manifest.KDPLayout `json:"kdpLayout,omitempty"`
	Pages            []previewPage      `json:"pages"`
}

func generateDataJS(m *manifest.Manifest) (string, error) {
	manifestPages := m.Progress.Pages

	pages := make([]previewPage, len(manifestPages))
	for i, p := range manifestPages {
		imgPath := p.ImagePath
		if imgPath != "" {
			imgPath = "../" + imgPath
		}
		pages[i] = previewPage{
			PageIndex:          p.PageIndex,
			Text:               p.Text,
			ImagePath:          imgPath,
			IllustrationPrompt: p.IllustrationPrompt,
			Layout:             p.Layout,
		}
	}

	data := previewData{
		Theme:            m.BookProperties.Theme,
		Style:            m.BookProperties.Style,
		CharacterProfile: m.BookProperties.CharacterProfile,
		TrimSize:         m.BookProperties.TrimSize,
		Format:           m.BookProperties.Format,
		KDPLayout:        m.KDPLayout,
		Pages:            pages,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("window.bookData = %s;\n", string(jsonData)), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Pithos Book Previewer</title>
  <link rel="stylesheet" href="preview.css">
</head>
<body>
  <div class="app-container">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <span class="logo">🏺 Pithos</span>
        <span class="badge">Previewer</span>
      </div>

      <div class="meta-section">
        <h3>Book Parameters</h3>
        <div class="meta-card">
          <label>Theme</label>
          <div id="meta-theme" class="meta-value">-</div>
        </div>
        <div class="meta-card-row">
          <div class="meta-card half">
            <label>Trim Size</label>
            <div id="meta-trim" class="meta-value">-</div>
          </div>
          <div class="meta-card half">
            <label>Format</label>
            <div id="meta-format" class="meta-value">-</div>
          </div>
        </div>
        <div class="meta-card">
          <label>Visual Style</label>
          <div id="meta-style" class="meta-value scrollable-content">-</div>
        </div>
        <div class="meta-card">
          <label>Character Profile</label>
          <div id="meta-character" class="meta-value scrollable-content">-</div>
        </div>
      </div>

      <div class="navigation-section">
        <h3>Pages List</h3>
        <ul id="pages-list" class="pages-list">
          <!-- Populated by JS -->
        </ul>
      </div>
    </aside>

    <!-- Main Workspace -->
    <main class="workspace">
      <!-- Top controls -->
      <header class="workspace-header">
        <div class="title-area">
          <h1 id="book-title">Interactive Book Preview</h1>
        </div>
        <div class="controls-area">
          <button id="toggle-guides" class="btn btn-secondary">
            <span class="icon">📐</span> Show Print Guides
          </button>
          <button id="toggle-prompts" class="btn btn-secondary">
            <span class="icon">👁️</span> Show Prompts
          </button>
        </div>
      </header>

      <!-- Center Stage (Page container) -->
      <div class="stage">
        <button id="prev-btn" class="nav-arrow nav-prev" aria-label="Previous page">‹</button>
        
        <div class="page-container" id="page-container">
          <!-- Rendered Pages will go here -->
        </div>

        <button id="next-btn" class="nav-arrow nav-next" aria-label="Next page">›</button>
      </div>

      <!-- Footer / Helper -->
      <footer class="workspace-footer">
        <div class="kbd-shortcuts">
          <span class="kbd">←</span> <span class="kbd">→</span> Arrow keys to flip pages
        </div>
        <div id="page-counter" class="page-counter">Page 0 of 0</div>
      </footer>
    </main>
  </div>

  <script src="data.js"></script>
  <script src="preview.js"></script>
</body>
</html>
`

const cssTemplate = `/* Reset & Base Styles */
* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background-color: #0d0e12;
  color: #c9d1d9;
  height: 100vh;
  overflow: hidden;
}

/* Layout */
.app-container {
  display: flex;
  height: 100vh;
  width: 100vw;
}

/* Sidebar Styling */
.sidebar {
  width: 330px;
  background-color: #12141c;
  border-right: 1px solid #21262d;
  display: flex;
  flex-direction: column;
  padding: 24px;
  flex-shrink: 0;
  overflow-y: hidden;
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 24px;
}

.logo {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 24px;
  font-weight: 700;
  color: #e6edf3;
  letter-spacing: -0.5px;
}

.badge {
  background-color: rgba(79, 70, 229, 0.15);
  color: #818cf8;
  border: 1px solid rgba(79, 70, 229, 0.3);
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 20px;
}

.meta-section h3, .navigation-section h3 {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 1.2px;
  color: #8b949e;
  margin-bottom: 10px;
}

.meta-section {
  margin-bottom: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.meta-card-row {
  display: flex;
  gap: 8px;
}

.meta-card {
  background-color: #161b22;
  border: 1px solid #21262d;
  border-radius: 8px;
  padding: 12px;
  max-height: 160px;
  display: flex;
  flex-direction: column;
}

.meta-card.half {
  flex: 1;
  min-width: 0;
}

.meta-card label {
  font-size: 9px;
  font-weight: 700;
  text-transform: uppercase;
  color: #8b949e;
  display: block;
  margin-bottom: 6px;
  letter-spacing: 0.5px;
}

.meta-value {
  font-size: 12px;
  color: #e6edf3;
  line-height: 1.4;
  word-break: break-word;
}

.scrollable-content {
  overflow-y: auto;
  padding-right: 4px;
  flex-grow: 1;
}

/* Custom Scrollbar */
.scrollable-content::-webkit-scrollbar, .pages-list::-webkit-scrollbar {
  width: 4px;
}
.scrollable-content::-webkit-scrollbar-track, .pages-list::-webkit-scrollbar-track {
  background: transparent;
}
.scrollable-content::-webkit-scrollbar-thumb, .pages-list::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 4px;
}

.navigation-section {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.pages-list {
  list-style: none;
  overflow-y: auto;
  flex-grow: 1;
  padding-right: 4px;
}

.pages-list li {
  padding: 10px 14px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #8b949e;
  transition: all 0.2s ease;
  margin-bottom: 4px;
  border: 1px solid transparent;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pages-list li:hover {
  background-color: #1f242c;
  color: #e6edf3;
}

.pages-list li.active {
  background-color: rgba(79, 70, 229, 0.1);
  color: #818cf8;
  border-color: rgba(79, 70, 229, 0.4);
  font-weight: 500;
}

/* Workspace Styling */
.workspace {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  background-color: #0b0c0e;
  position: relative;
}

.workspace-header {
  height: 70px;
  border-bottom: 1px solid #21262d;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
}

.workspace-header h1 {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 18px;
  font-weight: 600;
  color: #e6edf3;
}

.btn {
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  outline: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.btn-secondary {
  background-color: #161b22;
  color: #c9d1d9;
  border: 1px solid #30363d;
}

.btn-secondary:hover {
  background-color: #21262d;
  border-color: #8b949e;
  color: #f0f6fc;
}

.btn-secondary.active {
  background-color: rgba(129, 140, 248, 0.1);
  color: #818cf8;
  border-color: rgba(129, 140, 248, 0.4);
}

/* Stage & Flipbook Container */
.stage {
  flex-grow: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  position: relative;
}

.nav-arrow {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background-color: rgba(22, 27, 34, 0.8);
  border: 1px solid #30363d;
  color: #c9d1d9;
  font-size: 24px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  z-index: 100;
  line-height: 1;
  user-select: none;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
}

.nav-arrow:hover {
  background-color: #30363d;
  color: #f0f6fc;
  transform: scale(1.08);
}

.page-container {
  width: 700px;
  height: 700px;
  position: relative;
  perspective: 1500px;
  margin: 0 30px;
  transition: all 0.3s ease;
}

/* Individual Pages */
.page {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: #faf8f4; /* Authentic cream paper background */
  background-image: 
    radial-gradient(rgba(0, 0, 0, 0.02) 1px, transparent 0), 
    radial-gradient(rgba(0, 0, 0, 0.02) 1px, transparent 0);
  background-size: 8px 8px;
  background-position: 0 0, 4px 4px;
  color: #1f2328;
  border-radius: 4px; /* Crisp book cut edge */
  overflow: hidden;
  transition: transform 0.6s cubic-bezier(0.25, 1, 0.5, 1), opacity 0.6s ease;
  transform-style: preserve-3d;
  backface-visibility: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 15px 35px rgba(0, 0, 0, 0.4), 0 5px 15px rgba(0, 0, 0, 0.15);
}

/* Gutter shadow fold crease - recto (gutter left) */
.page.recto::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 30px;
  height: 100%;
  background: linear-gradient(to right, rgba(0, 0, 0, 0.14) 0%, rgba(0, 0, 0, 0.06) 30%, rgba(0, 0, 0, 0.02) 75%, rgba(0, 0, 0, 0) 100%);
  z-index: 10;
  pointer-events: none;
}

/* Gutter shadow fold crease - verso (gutter right) */
.page.verso::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 30px;
  height: 100%;
  background: linear-gradient(to left, rgba(0, 0, 0, 0.14) 0%, rgba(0, 0, 0, 0.06) 30%, rgba(0, 0, 0, 0.02) 75%, rgba(0, 0, 0, 0) 100%);
  z-index: 10;
  pointer-events: none;
}

/* Inside gutter margin alignments */
.page.recto .page-inner {
  padding: 30px 30px 30px 48px;
}

.page.verso .page-inner {
  padding: 30px 48px 30px 30px;
}

.page.recto .pending-layout {
  padding: 30px 30px 30px 48px;
}

.page.verso .pending-layout {
  padding: 30px 48px 30px 30px;
}

/* Print Guidelines */
.print-guide {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 20;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.show-guides .print-guide {
  opacity: 1;
}

.cut-line {
  position: absolute;
  border: 1px dashed rgba(255, 74, 74, 0.85);
  box-sizing: border-box;
}

.safety-line {
  position: absolute;
  border: 1px dashed rgba(74, 144, 226, 0.85);
  box-sizing: border-box;
}

.guide-label {
  position: absolute;
  font-size: 8px;
  font-family: monospace;
  color: #ffffff;
  padding: 1px 3px;
  border-radius: 2px;
  line-height: 1;
}

.guide-label.cut {
  background: rgba(255, 74, 74, 0.8);
}

.guide-label.safety {
  background: rgba(74, 144, 226, 0.8);
}

.page.hidden-left {
  transform: rotateY(-75deg) translateZ(-50px);
  opacity: 0;
  pointer-events: none;
}

.page.hidden-right {
  transform: rotateY(75deg) translateZ(-50px);
  opacity: 0;
  pointer-events: none;
}

.page.active {
  transform: rotateY(0deg);
  opacity: 1;
  z-index: 10;
}

/* Page Contents */
.page-background {
  width: 100%;
  height: 100%;
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
  position: absolute;
  top: 0;
  left: 0;
  z-index: 1;
}

.page-inner {
  position: relative;
  width: 100%;
  height: 100%;
  z-index: 2;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 30px;
}

/* Text Overlay Block */
.text-overlay {
  background: rgba(15, 17, 26, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 12px 16px;
  width: 100%;
  margin-top: auto;
  max-height: 45%;
  overflow-y: auto;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.4);
  text-align: center;
}

/* Custom Scrollbar for Text Overlay */
.text-overlay::-webkit-scrollbar {
  width: 3px;
}
.text-overlay::-webkit-scrollbar-track {
  background: transparent;
}
.text-overlay::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
}

.stanza-text {
  font-family: Georgia, serif;
  font-size: 14px;
  line-height: 1.45;
  font-weight: 500;
  color: #ffffff;
  white-space: pre-line;
}

/* Prompt Block */
.prompt-overlay {
  background: rgba(79, 70, 229, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  padding: 12px 16px;
  font-size: 12px;
  line-height: 1.5;
  color: #ffffff;
  position: absolute;
  top: 20px;
  left: 20px;
  right: 20px;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.35);
  z-index: 15;
  opacity: 0;
  transform: translateY(-10px);
  transition: all 0.3s cubic-bezier(0.25, 1, 0.5, 1);
  pointer-events: none;
}

.prompt-overlay.visible {
  opacity: 1;
  transform: translateY(0);
}

.prompt-title {
  font-weight: 700;
  text-transform: uppercase;
  font-size: 10px;
  letter-spacing: 0.8px;
  margin-bottom: 4px;
  opacity: 0.8;
}

/* Storyboarding Pending Page Layout */
.pending-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 30px;
  justify-content: space-between;
  background-color: #fdfbf7;
}

.placeholder-frame {
  flex-grow: 1;
  border: 2px dashed #d6cfc7;
  background-color: #f8f6f0;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
  margin-bottom: 24px;
  text-align: center;
}

.placeholder-badge {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  font-size: 11px;
  font-weight: 600;
  color: #8c7b6d;
  background-color: #ebdcd0;
  padding: 4px 12px;
  border-radius: 12px;
  margin-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.8px;
}

.placeholder-prompt {
  font-size: 12px;
  line-height: 1.5;
  color: #706356;
  max-width: 90%;
  font-style: italic;
  overflow-y: auto;
  max-height: 120px;
  padding-right: 4px;
}

.stanza-text-print {
  font-family: Georgia, serif;
  font-size: 15px;
  line-height: 1.45;
  text-align: center;
  color: #1a1a1a;
  white-space: pre-line;
  padding: 8px 0;
  margin-top: auto;
  max-height: 40%;
  overflow-y: auto;
}

/* Custom Scrollbar for Print Text */
.stanza-text-print::-webkit-scrollbar {
  width: 3px;
}
.stanza-text-print::-webkit-scrollbar-track {
  background: transparent;
}
.stanza-text-print::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.15);
  border-radius: 3px;
}

/* Footer Styling */
.workspace-footer {
  height: 50px;
  border-top: 1px solid #21262d;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  font-size: 12px;
  color: #8b949e;
}

.kbd {
  background-color: #21262d;
  border: 1px solid #30363d;
  border-radius: 4px;
  padding: 2px 6px;
  font-weight: 600;
  color: #c9d1d9;
}
`

const jsTemplate = `document.addEventListener('DOMContentLoaded', () => {
  if (typeof window.bookData === 'undefined') {
    console.error('window.bookData is not defined. Make sure data.js is loaded.');
    const pageContainer = document.getElementById('page-container');
    if (pageContainer) {
      pageContainer.innerHTML = '<div class="page"><div class="pending-layout"><div class="stanza-text-print">Error: window.bookData is not defined. data.js may have failed to load.</div></div></div>';
    }
    return;
  }

  const bookData = window.bookData;

  // 1. Initial configuration mapping
  const metaTheme = document.getElementById('meta-theme');
  const metaTrim = document.getElementById('meta-trim');
  const metaFormat = document.getElementById('meta-format');
  const metaStyle = document.getElementById('meta-style');
  const metaCharacter = document.getElementById('meta-character');
  const bookTitle = document.getElementById('book-title');

  metaTheme.textContent = bookData.theme || 'Not Set';
  metaTrim.textContent = bookData.trimSize || 'Not Set';
  metaFormat.textContent = bookData.format || 'Not Set';
  metaStyle.textContent = bookData.style || 'Not Set';
  metaCharacter.textContent = bookData.characterProfile || 'Not Set';
  bookTitle.textContent = (bookData.theme ? bookData.theme + ' - Preview' : 'Interactive Book Preview');

  // 2. Build pages and list elements
  const pageContainer = document.getElementById('page-container');
  const pagesList = document.getElementById('pages-list');

  let activeIndex = 0;
  let showPrompts = false;
  let showGuides = false;

  try {
    activeIndex = parseInt(sessionStorage.getItem('pithos_activeIndex'), 10) || 0;
    showPrompts = sessionStorage.getItem('pithos_showPrompts') === 'true';
    showGuides = sessionStorage.getItem('pithos_showGuides') === 'true';
  } catch (e) {}

  const pages = bookData.pages || [];

  if (pages.length === 0) {
    pageContainer.innerHTML = '<div class="page"><div class="pending-layout"><div class="stanza-text-print">No pages generated yet.</div></div></div>';
    return;
  }

  if (activeIndex >= pages.length) {
    activeIndex = 0;
  }

  // 3. Aspect Ratio and Sizing Calculation
  let trimWidth = 8.5;
  let trimHeight = 8.5;
  if (bookData.trimSize) {
    const parts = bookData.trimSize.split('x');
    if (parts.length === 2) {
      const w = parseFloat(parts[0]);
      const h = parseFloat(parts[1]);
      if (!isNaN(w) && !isNaN(h) && w > 0 && h > 0) {
        trimWidth = w;
        trimHeight = h;
      }
    }
  }

  // 3.1. Geometry parameters binding
  let bleedInches = 0.125;
  let safetyInside = 0.75;
  let safetyOutside = 0.375;

  if (bookData.kdpLayout && bookData.kdpLayout.margin_size > 0) {
    bleedInches = bookData.kdpLayout.bleed;
    safetyInside = bookData.kdpLayout.margin_size;
    safetyOutside = (bleedInches > 0) ? 0.375 : 0.25;
  }

  const hasBleed = bleedInches > 0;
  const pageW = trimWidth + (hasBleed ? bleedInches : 0);
  const pageH = trimHeight + (hasBleed ? (2 * bleedInches) : 0);

  const maxW = 700;
  const maxH = 700;
  let finalW = maxW;
  let finalH = maxH;
  const trimRatio = pageW / pageH;
  const maxRatio = maxW / maxH;

  if (trimRatio > maxRatio) {
    finalW = maxW;
    finalH = maxW / trimRatio;
  } else {
    finalH = maxH;
    finalW = maxH * trimRatio;
  }

  pageContainer.style.width = Math.round(finalW) + 'px';
  pageContainer.style.height = Math.round(finalH) + 'px';

  const pxPerInch = finalW / pageW;
  const padTopPx = Math.round(((hasBleed ? bleedInches : 0) + safetyOutside) * pxPerInch);
  const padBottomPx = Math.round(((hasBleed ? bleedInches : 0) + safetyOutside) * pxPerInch);
  const padOutsidePx = Math.round(((hasBleed ? bleedInches : 0) + safetyOutside) * pxPerInch);
  const padInsidePx = Math.round(safetyInside * pxPerInch);

  // Helper to generate print guides
  function createPrintGuides(pageEl, isRecto) {
    const guideContainer = document.createElement('div');
    guideContainer.className = 'print-guide';

    const topCut = hasBleed ? (bleedInches / pageH) * 100 : 0;
    const bottomCut = hasBleed ? (bleedInches / pageH) * 100 : 0;
    const leftCut = (hasBleed && !isRecto) ? (bleedInches / pageW) * 100 : 0;
    const rightCut = (hasBleed && isRecto) ? (bleedInches / pageW) * 100 : 0;

    // Cut box line
    const cutLine = document.createElement('div');
    cutLine.className = 'cut-line';
    cutLine.style.top = topCut.toFixed(2) + '%';
    cutLine.style.bottom = bottomCut.toFixed(2) + '%';
    cutLine.style.left = leftCut.toFixed(2) + '%';
    cutLine.style.right = rightCut.toFixed(2) + '%';

    const cutLabel = document.createElement('div');
    cutLabel.className = 'guide-label cut';
    cutLabel.textContent = 'CUT LINE';
    if (isRecto) {
      cutLabel.style.right = '4px';
      cutLabel.style.top = (topCut + 1).toFixed(2) + '%';
    } else {
      cutLabel.style.left = '4px';
      cutLabel.style.top = (topCut + 1).toFixed(2) + '%';
    }
    cutLine.appendChild(cutLabel);
    guideContainer.appendChild(cutLine);

    // Safety margins box
    const safetyTop = topCut + (safetyOutside / pageH) * 100;
    const safetyBottom = bottomCut + (safetyOutside / pageH) * 100;
    const safetyLeft = leftCut + ((isRecto ? safetyInside : safetyOutside) / pageW) * 100;
    const safetyRight = rightCut + ((isRecto ? safetyOutside : safetyInside) / pageW) * 100;

    const safetyLine = document.createElement('div');
    safetyLine.className = 'safety-line';
    safetyLine.style.top = safetyTop.toFixed(2) + '%';
    safetyLine.style.bottom = safetyBottom.toFixed(2) + '%';
    safetyLine.style.left = safetyLeft.toFixed(2) + '%';
    safetyLine.style.right = safetyRight.toFixed(2) + '%';

    const safetyLabel = document.createElement('div');
    safetyLabel.className = 'guide-label safety';
    safetyLabel.textContent = 'SAFE ZONE';
    safetyLabel.style.top = '4px';
    safetyLabel.style.left = '50%';
    safetyLabel.style.transform = 'translateX(-50%)';
    safetyLine.appendChild(safetyLabel);

    guideContainer.appendChild(safetyLine);
    pageEl.appendChild(guideContainer);
  }

  // Build Pages
  pages.forEach((page, idx) => {
    // Build Sidebar List Item
    const li = document.createElement('li');
    li.textContent = 'Page ' + page.pageIndex + ': ' + page.text.split('\n')[0];
    li.addEventListener('click', () => jumpToPage(idx));
    pagesList.appendChild(li);

    // Build Page DOM
    const pageEl = document.createElement('div');
    const isRecto = (idx % 2 === 0);
    pageEl.className = 'page ' + (isRecto ? 'recto' : 'verso');
    pageEl.id = 'page-' + idx;

    if (page.imagePath) {
      // Full Bleed Layout
      let mediaEl;
      const lowerPath = page.imagePath.toLowerCase();
      if (lowerPath.endsWith('.mp4') || lowerPath.endsWith('.webm')) {
        mediaEl = document.createElement('video');
        mediaEl.className = 'page-background';
        mediaEl.src = page.imagePath;
        mediaEl.autoplay = true;
        mediaEl.loop = true;
        mediaEl.muted = true;
        mediaEl.setAttribute('playsinline', '');
        mediaEl.style.objectFit = 'cover';
      } else {
        mediaEl = document.createElement('div');
        mediaEl.className = 'page-background';
        mediaEl.style.backgroundImage = "url('" + page.imagePath + "')";
      }
      pageEl.appendChild(mediaEl);

      const inner = document.createElement('div');
      inner.className = 'page-inner';
      if (isRecto) {
        inner.style.padding = padTopPx + 'px ' + padOutsidePx + 'px ' + padBottomPx + 'px ' + padInsidePx + 'px';
      } else {
        inner.style.padding = padTopPx + 'px ' + padInsidePx + 'px ' + padBottomPx + 'px ' + padOutsidePx + 'px';
      }

      // Prompt Overlay
      if (page.illustrationPrompt) {
        const promptEl = document.createElement('div');
        promptEl.className = 'prompt-overlay';
        const titleEl = document.createElement('div');
        titleEl.className = 'prompt-title';
        titleEl.textContent = 'Prompt Reference';
        promptEl.appendChild(titleEl);
        const textNode = document.createTextNode(page.illustrationPrompt);
        promptEl.appendChild(textNode);
        inner.appendChild(promptEl);
      }

      // Text Overlay
      const textOverlay = document.createElement('div');
      textOverlay.className = 'text-overlay';
      const stanzaText = document.createElement('div');
      stanzaText.className = 'stanza-text';
      stanzaText.textContent = page.text;
      textOverlay.appendChild(stanzaText);

      inner.appendChild(textOverlay);
      pageEl.appendChild(inner);
    } else {
      // Storyboard Page Layout
      const inner = document.createElement('div');
      inner.className = 'page-inner pending-layout';
      if (isRecto) {
        inner.style.padding = padTopPx + 'px ' + padOutsidePx + 'px ' + padBottomPx + 'px ' + padInsidePx + 'px';
      } else {
        inner.style.padding = padTopPx + 'px ' + padInsidePx + 'px ' + padBottomPx + 'px ' + padOutsidePx + 'px';
      }

      const placeholderFrame = document.createElement('div');
      placeholderFrame.className = 'placeholder-frame';

      const placeholderBadge = document.createElement('div');
      placeholderBadge.className = 'placeholder-badge';
      placeholderBadge.textContent = '🎨 Illustration Pending';
      placeholderFrame.appendChild(placeholderBadge);

      if (page.illustrationPrompt) {
        const placeholderPrompt = document.createElement('div');
        placeholderPrompt.className = 'placeholder-prompt scrollable-content';
        placeholderPrompt.textContent = page.illustrationPrompt;
        placeholderFrame.appendChild(placeholderPrompt);
      }

      inner.appendChild(placeholderFrame);

      const stanzaTextPrint = document.createElement('div');
      stanzaTextPrint.className = 'stanza-text-print';
      stanzaTextPrint.textContent = page.text;
      inner.appendChild(stanzaTextPrint);

      pageEl.appendChild(inner);
    }

    createPrintGuides(pageEl, isRecto);
    pageContainer.appendChild(pageEl);
  });

  // Navigation Logic
  function updateDOMState() {
    const listItems = pagesList.querySelectorAll('li');
    const pageElements = pageContainer.querySelectorAll('.page');

    pageElements.forEach((el, idx) => {
      const isRecto = (idx % 2 === 0);
      el.className = 'page ' + (isRecto ? 'recto' : 'verso');
      if (idx < activeIndex) {
        el.classList.add('hidden-left');
      } else if (idx > activeIndex) {
        el.classList.add('hidden-right');
      } else {
        el.classList.add('active');
      }

      // Update prompts visible state
      const prompt = el.querySelector('.prompt-overlay');
      if (prompt) {
        if (showPrompts) {
          prompt.classList.add('visible');
        } else {
          prompt.classList.remove('visible');
        }
      }
    });

    listItems.forEach((li, idx) => {
      if (idx === activeIndex) {
        li.classList.add('active');
      } else {
        li.classList.remove('active');
      }
    });

    // Page Counter
    const pageCounter = document.getElementById('page-counter');
    pageCounter.textContent = 'Page ' + (activeIndex + 1) + ' of ' + pages.length;
  }

  function nextPage() {
    if (activeIndex < pages.length - 1) {
      activeIndex++;
      try {
        sessionStorage.setItem('pithos_activeIndex', activeIndex);
      } catch (e) {}
      updateDOMState();
    }
  }

  function prevPage() {
    if (activeIndex > 0) {
      activeIndex--;
      try {
        sessionStorage.setItem('pithos_activeIndex', activeIndex);
      } catch (e) {}
      updateDOMState();
    }
  }

  // Jump to specific page
  function jumpToPage(index) {
    if (index >= 0 && index < pages.length) {
      activeIndex = index;
      try {
        sessionStorage.setItem('pithos_activeIndex', activeIndex);
      } catch (e) {}
      updateDOMState();
    }
  }

  // Bind Buttons
  document.getElementById('next-btn').addEventListener('click', nextPage);
  document.getElementById('prev-btn').addEventListener('click', prevPage);

  // Toggle Prompts
  const togglePromptsBtn = document.getElementById('toggle-prompts');
  togglePromptsBtn.addEventListener('click', () => {
    showPrompts = !showPrompts;
    try {
      sessionStorage.setItem('pithos_showPrompts', showPrompts);
    } catch (e) {}
    if (showPrompts) {
      togglePromptsBtn.classList.add('active');
    } else {
      togglePromptsBtn.classList.remove('active');
    }
    updateDOMState();
  });

  // Toggle Print Guides
  const toggleGuidesBtn = document.getElementById('toggle-guides');
  toggleGuidesBtn.addEventListener('click', () => {
    showGuides = !showGuides;
    try {
      sessionStorage.setItem('pithos_showGuides', showGuides);
    } catch (e) {}
    if (showGuides) {
      toggleGuidesBtn.classList.add('active');
      pageContainer.classList.add('show-guides');
    } else {
      toggleGuidesBtn.classList.remove('active');
      pageContainer.classList.remove('show-guides');
    }
  });

  // Bind Key Events
  document.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowRight') {
      nextPage();
    } else if (e.key === 'ArrowLeft') {
      prevPage();
    }
  });

  // Set initial UI classes/states for active prompts/guides buttons
  if (showPrompts) {
    togglePromptsBtn.classList.add('active');
  }
  if (showGuides) {
    toggleGuidesBtn.classList.add('active');
    pageContainer.classList.add('show-guides');
  }

  // Polling for hot-reload
  let currentDataStr = JSON.stringify(window.bookData);
  let pollPending = false;
  function pollForUpdates() {
    if (pollPending) {
      return;
    }
    pollPending = true;
    const script = document.createElement('script');
    script.className = 'pithos-poll-script';
    script.src = 'data.js?t=' + Date.now();
    script.onload = () => {
      script.remove();
      pollPending = false;
      const newDataStr = JSON.stringify(window.bookData);
      if (newDataStr !== currentDataStr) {
        console.log('Book data updated. Reloading...');
        window.location.reload();
      } else {
        currentDataStr = newDataStr;
      }
    };
    script.onerror = () => {
      script.remove();
      pollPending = false;
    };
    document.head.appendChild(script);
  }
  setInterval(pollForUpdates, 1500);

  // Initial State Run
  updateDOMState();
});`
