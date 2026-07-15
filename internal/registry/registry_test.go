package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryOperations(t *testing.T) {
	// Create a temporary file to act as our registry.json
	tempDir, err := os.MkdirTemp("", "pithos_registry_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	regFile := filepath.Join(tempDir, "registry.json")
	SetRegistryPathOverride(regFile)
	defer func() { SetRegistryPathOverride("") }()

	// 1. Load empty registry
	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load empty registry: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected empty registry, got %d items", len(workspaces))
	}

	// Create some temp directories to use as workspaces
	ws1, err := os.MkdirTemp("", "ws1")
	if err != nil {
		t.Fatalf("failed to create ws1: %v", err)
	}
	defer os.RemoveAll(ws1)

	ws2, err := os.MkdirTemp("", "ws2")
	if err != nil {
		t.Fatalf("failed to create ws2: %v", err)
	}
	defer os.RemoveAll(ws2)

	absWS1, _ := filepath.Abs(ws1)
	absWS1 = filepath.Clean(absWS1)
	absWS2, _ := filepath.Abs(ws2)
	absWS2 = filepath.Clean(absWS2)

	// 2. Add workspaces
	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	if err := Add(ws2); err != nil {
		t.Fatalf("failed to add ws2: %v", err)
	}

	// Verify they are added
	workspaces, err = Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0] != absWS1 || workspaces[1] != absWS2 {
		t.Errorf("unexpected registered paths: %v", workspaces)
	}

	// Try adding duplicate
	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add duplicate: %v", err)
	}
	workspaces, err = Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 2 {
		t.Errorf("expected duplicate to be ignored, got %d workspaces", len(workspaces))
	}

	// 3. Remove workspace
	if err := Remove(ws1); err != nil {
		t.Fatalf("failed to remove ws1: %v", err)
	}
	workspaces, err = Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace left, got %d", len(workspaces))
	}
	if workspaces[0] != absWS2 {
		t.Errorf("expected ws2 to remain, got %s", workspaces[0])
	}

	// 4. Prune workspaces
	// Add a non-existent directory path
	fakePath := filepath.Join(tempDir, "non_existent_dir")
	// Add it to registry
	if err := Add(fakePath); err != nil {
		t.Fatalf("failed to add fake path: %v", err)
	}

	// Prune
	if err := Prune(); err != nil {
		t.Fatalf("failed to prune: %v", err)
	}

	// Verify only ws2 remains since fakePath doesn't exist
	workspaces, err = Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace after pruning, got %d", len(workspaces))
	}
	if workspaces[0] != absWS2 {
		t.Errorf("expected only ws2 to remain after pruning, got %s", workspaces[0])
	}
}
