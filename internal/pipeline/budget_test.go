package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/powerword/pkg/telemetry"
)

func createTestBook(t *testing.T, tmpDir string, pageCount int) {
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Test Book",
		TargetPageCount: pageCount,
	}
	mInit, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate book: %v", err)
	}
	mInit.BookProperties.Style = "mock-style"
	mInit.BookProperties.CharacterProfile = "mock-character-profile"
	mInit.BookProperties.CharacterReferenceURL = "http://example.com/character.png"
	if err = mInit.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}
}

func TestBudget_UnderBudget(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-under-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 3)

	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 40000.00}, // $0.04 per image
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 1.00, // plenty of budget
		},
	}

	mockStanzas := []string{"Stanza 1 text", "Stanza 2 text", "Stanza 3 text"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
		DryRun:    true,
		Silent:    true,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return true }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected no budget error, got: %v", err)
	}
}

func TestBudget_OverBudget_Headless(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-headless-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 5)

	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 40000.00}, // $0.04 per image
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 0.10, // low budget (estimated will be 5*0.04 + 0.01 LLM = 0.21)
		},
	}

	mockStanzas := []string{"S1", "S2", "S3", "S4", "S5"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
		DryRun:    true,
		Silent:    true,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Fatalf("expected budget exceeded error, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded error, got: %v", err)
	}
}

func TestBudget_OverBudget_TTY_Abort(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-tty-abort-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 3)

	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 40000.00},
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 0.05,
		},
	}

	mockStanzas := []string{"S1", "S2", "S3"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	var outBuf bytes.Buffer
	inBuf := strings.NewReader("n\n") // Abort confirm prompt

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
		DryRun:    true,
		Silent:    false,
		In:        inBuf,
		Out:       &outBuf,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return true }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Brew(ctx, optsBrew)
	if err == nil {
		t.Fatalf("expected run aborted error, got nil")
	}
	if !strings.Contains(err.Error(), "run aborted") {
		t.Errorf("expected run aborted error, got: %v", err)
	}
}

func TestBudget_OverBudget_TTY_Proceed(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-tty-proceed-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 3)

	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 40000.00},
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 0.05,
		},
	}

	mockStanzas := []string{"S1", "S2", "S3"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	var outBuf bytes.Buffer
	inBuf := strings.NewReader("y\n") // Proceed confirm prompt

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
		DryRun:    true,
		Silent:    false,
		In:        inBuf,
		Out:       &outBuf,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return true }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected proceed override, got error: %v", err)
	}
}

func TestBudget_LiveBudgetCheck(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-live-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 5)

	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 40000.00},
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 0.25,
		},
	}

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       &mockLLM{stanzas: []string{"S1", "S2", "S3", "S4", "S5"}},
		DryRun:    true,
		Silent:    true,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- Brew(ctx, optsBrew)
	}()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		mLoaded, readErr := manifest.LoadManifest(manifestPath)
		if readErr == nil && mLoaded.Progress.ManuscriptGenerated {
			break
		}
	}

	config.Cfg.SetMaxCostUSD(0.05)

	err = <-errChan
	if err == nil {
		t.Fatalf("expected budget exceeded during execution, got nil")
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded error, got: %v", err)
	}

	mLoaded, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	completedCount := 0
	pendingCount := 0
	for _, p := range mLoaded.Progress.Pages {
		switch p.Status {
		case manifest.StatusCompleted:
			completedCount++
		case manifest.StatusPending:
			pendingCount++
		}
	}

	if completedCount >= 5 {
		t.Errorf("expected execution to stop before all 5 pages completed, got completedCount: %d", completedCount)
	}
	t.Logf("Completed: %d, Pending: %d", completedCount, pendingCount)
}

func TestBudget_NilConfig(t *testing.T) {
	origCfg := config.Cfg
	config.Cfg = nil
	defer func() { config.Cfg = origCfg }()

	tmpDir, err := os.MkdirTemp("", "pithos-budget-nil-cfg-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	createTestBook(t, tmpDir, 1)

	mockStanzas := []string{"S1"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
		DryRun:    true,
		Silent:    true,
		Budget:    0.05,
	}

	oldIsTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = oldIsTTY }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("expected no budget error, got: %v", err)
	}
}

func TestCheckBudget_Direct(t *testing.T) {
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	// 1. Nil config
	config.Cfg = nil
	m := manifest.NewManifest("")
	m.BookProperties.TargetPageCount = 2
	opts := BrewOptions{
		Budget: 0.05,
		Silent: true,
	}
	// Estimated: 2 pages * 0.04 + 0.01 LLM = 0.09 > 0.05
	err := checkBudget(context.Background(), m, &opts)
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded, got: %v", err)
	}

	// 2. Under budget nil config
	opts.Budget = 0.15
	err = checkBudget(context.Background(), m, &opts)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// 3. Custom pricing, over budget
	config.Cfg = &config.Config{
		Pricing: map[string]telemetry.ModelPricing{
			"imagegen": {Input: 100000.00},
		},
		Budget: config.BudgetConfig{
			MaxCostUSD: 0.05,
		},
	}
	opts.Budget = 0.0
	// Estimated: 2 pages * 0.10 + 0.01 LLM = 0.21 > 0.05
	err = checkBudget(context.Background(), m, &opts)
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded, got: %v", err)
	}

	// 4. TargetPageCount <= 0 fallback
	m.BookProperties.TargetPageCount = 0
	config.Cfg.Budget.MaxCostUSD = 1.00
	err = checkBudget(context.Background(), m, &opts)
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded, got: %v", err)
	}

	// 5. Manuscript already generated
	m.Progress.ManuscriptGenerated = true
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 1, Status: manifest.StatusCompleted, ImagePath: "img1.png"},
		{PageIndex: 2, Status: manifest.StatusPending},
	}
	config.Cfg.Budget.MaxCostUSD = 0.05
	err = checkBudget(context.Background(), m, &opts)
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("expected budget exceeded, got: %v", err)
	}

	// 6. With specific pages allowed
	opts.Pages = []int{1}
	err = checkBudget(context.Background(), m, &opts)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestIsPageAllowed(t *testing.T) {
	if !isPageAllowed(1, nil) {
		t.Error("expected true for nil filter")
	}
	if !isPageAllowed(1, []int{}) {
		t.Error("expected true for empty filter")
	}
	if !isPageAllowed(1, []int{1, 2}) {
		t.Error("expected true for matching index")
	}
	if isPageAllowed(3, []int{1, 2}) {
		t.Error("expected false for non-matching index")
	}
}

func TestFormatFileURL(t *testing.T) {
	u := formatFileURL("/a/b/c")
	if !strings.HasPrefix(u, "file:///") {
		t.Errorf("expected file:///, got %s", u)
	}
}
