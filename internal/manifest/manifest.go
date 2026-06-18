package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/borch-ai/powerword/pkg/telemetry"
)

// PageStatus represents the processing state of a page.
type PageStatus string

const (
	StatusPending          PageStatus = "pending"
	StatusGeneratingImages PageStatus = "generating_images"
	StatusAwaitingApproval PageStatus = "awaiting_approval"
	StatusCompleted        PageStatus = "completed"
)

// PageState holds the progress details of a single page/stanza.
type PageState struct {
	PageIndex          int        `json:"page_index"`
	Status             PageStatus `json:"status"`
	ImagePath          string     `json:"image_path,omitempty"`
	Text               string     `json:"text,omitempty"`
	IllustrationPrompt string     `json:"illustration_prompt,omitempty"`
	CharacterWeight    *int       `json:"character_weight,omitempty"`
}

// BookProperties holds high-level configurations of the book.
type BookProperties struct {
	Theme                 string `json:"theme"`
	Style                 string `json:"style"`
	CharacterProfile      string `json:"character_profile"`
	Format                string `json:"format"`
	TargetPageCount       int    `json:"target_page_count"`
	TrimSize              string `json:"trim_size,omitempty"`
	CharacterReferenceURL string `json:"character_reference_url,omitempty"`
	CharacterWeight       int    `json:"character_weight,omitempty"`
}

// Progress tracks the completion state of various pipeline stages.
type Progress struct {
	ManuscriptGenerated bool        `json:"manuscript_generated"`
	CoverImageGenerated bool        `json:"cover_image_generated"`
	CoverImagePath      string      `json:"cover_image_path,omitempty"`
	Pages               []PageState `json:"pages"`
}

// LayoutGuide represents a logical bounding box in the cover coordinate system.
type LayoutGuide struct {
	Label        string  `json:"label"`
	XInches      float64 `json:"x_inches"`
	YInches      float64 `json:"y_inches"`
	WidthInches  float64 `json:"width_inches"`
	HeightInches float64 `json:"height_inches"`
}

// KDPLayout defines the spacing and dimensional attributes for Amazon KDP.
type KDPLayout struct {
	SpineWidth           float64       `json:"spine_width"`
	MarginSize           float64       `json:"margin_size"`
	Bleed                float64       `json:"bleed"`
	CoverWidthInches     float64       `json:"cover_width_inches,omitempty"`
	CoverHeightInches    float64       `json:"cover_height_inches,omitempty"`
	CoverWidthPoints     float64       `json:"cover_width_points,omitempty"`
	CoverHeightPoints    float64       `json:"cover_height_points,omitempty"`
	HingeWidthInches     float64       `json:"hinge_width_inches,omitempty"`
	WrapWidthInches      float64       `json:"wrap_width_inches,omitempty"`
	OverhangHeightInches float64       `json:"overhang_height_inches,omitempty"`
	SpineTextEligible    bool          `json:"spine_text_eligible"`
	Guides               []LayoutGuide `json:"guides,omitempty"`
}

// TelemetryMetrics tracks token usage and book generation costs.
type TelemetryMetrics struct {
	ModelUsages      map[string]*telemetry.ModelUsage `json:"model_usages"`
	ImageGenerations int64                            `json:"image_generations"`
	TotalCostUSD     float64                          `json:"total_cost_usd"`
}

// KilnSync contains fields written by Pithos for consumption by Kiln.
// Kiln reads these fields; it never writes to this struct.
type KilnSync struct {
	Version         int      `json:"kiln_sync_version"`
	Milestones      []string `json:"kiln_milestones"`
	TotalCostUSD    float64  `json:"total_cost_usd"`
	InteriorPDFPath string   `json:"interior_pdf_path,omitempty"`
	CoverPDFPath    string   `json:"cover_pdf_path,omitempty"`
}

// Manifest is the root structure serving as the checkpoint state file.
type Manifest struct {
	mu sync.RWMutex

	filePath string

	BookProperties BookProperties    `json:"book_properties"`
	Progress       Progress          `json:"progress"`
	AssetRegistry  map[string]string `json:"asset_registry"`
	KDPLayout      KDPLayout         `json:"kdp_layout"`
	Telemetry      TelemetryMetrics  `json:"telemetry"`
	Kiln           KilnSync          `json:"kiln"`
}

// NewManifest instantiates a new Manifest with initialized fields.
func NewManifest(path string) *Manifest {
	m := &Manifest{
		filePath:      path,
		AssetRegistry: make(map[string]string),
		Progress: Progress{
			Pages: make([]PageState, 0),
		},
		Telemetry: TelemetryMetrics{
			ModelUsages: make(map[string]*telemetry.ModelUsage),
		},
		Kiln: KilnSync{
			Version:    1,
			Milestones: make([]string, 0),
		},
	}
	m.BookProperties.CharacterWeight = 100
	return m
}

// LoadManifest reads a manifest from a file, parses it, and returns the Manifest pointer.
func LoadManifest(path string) (*Manifest, error) {
	//nolint:gosec // ReadFile path is constructed in local CLI environment for state management
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest json: %w", err)
	}

	m.filePath = path
	if m.AssetRegistry == nil {
		m.AssetRegistry = make(map[string]string)
	}
	if m.Progress.Pages == nil {
		m.Progress.Pages = make([]PageState, 0)
	}
	if m.Telemetry.ModelUsages == nil {
		m.Telemetry.ModelUsages = make(map[string]*telemetry.ModelUsage)
	}
	if m.Kiln.Milestones == nil {
		m.Kiln.Milestones = make([]string, 0)
	}
	if m.Kiln.Version == 0 {
		m.Kiln.Version = 1
	}
	if m.BookProperties.CharacterWeight == 0 {
		m.BookProperties.CharacterWeight = 100
	}

	return &m, nil
}

// AddMilestone appends a pipeline milestone to the Kiln status tracking if not already present.
func (m *Manifest) AddMilestone(milestone string) error {
	m.mu.Lock()
	if m.Kiln.Milestones == nil {
		m.Kiln.Milestones = make([]string, 0)
	}
	exists := false
	for _, ms := range m.Kiln.Milestones {
		if ms == milestone {
			exists = true
			break
		}
	}
	if !exists {
		m.Kiln.Milestones = append(m.Kiln.Milestones, milestone)
	}
	m.mu.Unlock()
	return m.Save()
}

// UpdatePDFPaths updates the interior and cover PDF absolute paths in Kiln status.
// Paths are converted to absolute paths using filepath.Abs if they are relative.
func (m *Manifest) UpdatePDFPaths(interiorPath, coverPath string) error {
	m.mu.Lock()
	if interiorPath != "" {
		if abs, err := filepath.Abs(interiorPath); err == nil {
			interiorPath = abs
		}
		m.Kiln.InteriorPDFPath = interiorPath
	}
	if coverPath != "" {
		if abs, err := filepath.Abs(coverPath); err == nil {
			coverPath = abs
		}
		m.Kiln.CoverPDFPath = coverPath
	}
	m.mu.Unlock()
	return m.Save()
}

// FilePath returns the file path of the manifest.
func (m *Manifest) FilePath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filePath
}

// SetFilePath updates the file path of the manifest.
func (m *Manifest) SetFilePath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filePath = path
}

// Save serializes and writes the manifest to its configured file path atomically.
func (m *Manifest) Save() error {
	m.mu.RLock()
	path := m.filePath
	m.mu.RUnlock()

	if path == "" {
		return errors.New("manifest file path is not set")
	}

	return m.SaveTo(path)
}

// SaveTo serializes and writes the manifest to the specified file path atomically.
func (m *Manifest) SaveTo(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	dir := filepath.Dir(path)
	// Create directory if it doesn't exist
	if mkdirErr := os.MkdirAll(dir, 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create manifest directory: %w", mkdirErr)
	}

	// Create a temporary file in the target directory to ensure it is on the same filesystem
	tmpFile, err := os.CreateTemp(dir, "manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpName := tmpFile.Name()

	// Ensure cleanup in case of panic or early return
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
		}
		_ = os.Remove(tmpName)
	}()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data to temporary file: %w", err)
	}

	// Flush to storage
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Close before renaming
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	tmpFile = nil // Prevent double close in defer

	// Atomic replacement
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace manifest file atomically: %w", err)
	}

	return nil
}

// UpdatePageStatus updates the progress state of a specific page and saves the manifest.
func (m *Manifest) UpdatePageStatus(pageIndex int, status PageStatus, imagePath string) error {
	m.mu.Lock()

	// Check if page already exists in m.Progress.Pages
	found := false
	for i, p := range m.Progress.Pages {
		if p.PageIndex == pageIndex {
			m.Progress.Pages[i].Status = status
			if imagePath != "" {
				m.Progress.Pages[i].ImagePath = imagePath
			}
			found = true
			break
		}
	}

	// If page was not found, create it
	if !found {
		m.Progress.Pages = append(m.Progress.Pages, PageState{
			PageIndex: pageIndex,
			Status:    status,
			ImagePath: imagePath,
		})
	}

	m.mu.Unlock()

	return m.Save()
}

// UpdateManuscriptStatus updates the manuscript generation status and saves the manifest.
func (m *Manifest) UpdateManuscriptStatus(generated bool) error {
	m.mu.Lock()
	m.Progress.ManuscriptGenerated = generated
	m.mu.Unlock()
	return m.Save()
}

// UpdateCoverImage updates the cover image generation status and file path and saves the manifest.
func (m *Manifest) UpdateCoverImage(generated bool, path string) error {
	m.mu.Lock()
	m.Progress.CoverImageGenerated = generated
	m.Progress.CoverImagePath = path
	m.mu.Unlock()
	return m.Save()
}

// RegisterAsset associates a key with a file path and saves the manifest.
func (m *Manifest) RegisterAsset(key, path string) error {
	m.mu.Lock()
	if m.AssetRegistry == nil {
		m.AssetRegistry = make(map[string]string)
	}
	m.AssetRegistry[key] = path
	m.mu.Unlock()
	return m.Save()
}

// PaperType represents KDP paper type.
type PaperType string

const (
	PaperWhite PaperType = "white"
	PaperCream PaperType = "cream"
	PaperColor PaperType = "color"
)

// CalculateSpineWidth calculates spine width in inches based on page count and paper type.
func CalculateSpineWidth(pageCount int, paperType PaperType) float64 {
	switch paperType {
	case PaperCream:
		return float64(pageCount) * 0.0025
	case PaperColor:
		return float64(pageCount) * 0.002347
	case PaperWhite:
		fallthrough
	default:
		return float64(pageCount) * 0.002252
	}
}

// CalculateTrimWithBleed calculates dimensions with bleed.
// Returns (widthWithBleed, heightWithBleed) in inches.
func CalculateTrimWithBleed(trimWidth, trimHeight float64) (float64, float64) {
	return trimWidth + 0.125, trimHeight + 0.25
}

// RecordLLMUsage accumulates LLM token usage and updates total cost under a single lock.
func (m *Manifest) RecordLLMUsage(model string, usage telemetry.TokenUsage, pricing map[string]telemetry.ModelPricing) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Telemetry.ModelUsages == nil {
		m.Telemetry.ModelUsages = make(map[string]*telemetry.ModelUsage)
	}

	mu, exists := m.Telemetry.ModelUsages[model]
	if !exists || mu == nil {
		mu = &telemetry.ModelUsage{}
		m.Telemetry.ModelUsages[model] = mu
	}

	mu.InputTokens += usage.InputTokens
	mu.OutputTokens += usage.OutputTokens
	mu.CachedTokens += usage.CachedTokens

	m.updateTotalCostLocked(pricing)
}

// RecordImageGeneration increments the image generation counter and updates total cost under a single lock.
func (m *Manifest) RecordImageGeneration(pricing map[string]telemetry.ModelPricing) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Telemetry.ImageGenerations++

	m.updateTotalCostLocked(pricing)
}

// UpdateTotalCost computes the total cost USD based on model usage and image generations.
func (m *Manifest) UpdateTotalCost(pricing map[string]telemetry.ModelPricing) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateTotalCostLocked(pricing)
}

// updateTotalCostLocked computes the total cost USD without acquiring the lock (expects lock to be held).
func (m *Manifest) updateTotalCostLocked(pricing map[string]telemetry.ModelPricing) {
	tracker := telemetry.NewUsageTracker()
	for model, usage := range m.Telemetry.ModelUsages {
		if usage != nil {
			tracker.RecordUsage(model, telemetry.TokenUsage{
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				CachedTokens: usage.CachedTokens,
			})
		}
	}

	llmCost := tracker.EstimatedCost(pricing)

	// Default image generation cost is $0.04 unless defined in pricing
	imageCost := 0.04
	if pricing != nil {
		if p, ok := pricing["imagegen"]; ok {
			imageCost = p.Input / 1_000_000.0
		}
	}

	m.Telemetry.TotalCostUSD = llmCost + float64(m.Telemetry.ImageGenerations)*imageCost
	m.Kiln.TotalCostUSD = m.Telemetry.TotalCostUSD
}
