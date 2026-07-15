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

func TestRegistryEdgeCases(t *testing.T) {
	// 1. Test GetRegistryPath without override
	SetRegistryPathOverride("")
	path := GetRegistryPath()
	if path == "" {
		t.Error("expected non-empty path from GetRegistryPath")
	}

	// Test GetRegistryPath when home dir is inaccessible/unset
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("USERPROFILE", "")

	fallbackPath := GetRegistryPath()
	if fallbackPath != ".pithos_registry.json" {
		t.Errorf("expected fallback path .pithos_registry.json, got %s", fallbackPath)
	}

	// Restore env variables
	os.Setenv("HOME", oldHome)
	os.Setenv("USERPROFILE", oldUserProfile)

	// 2. Test Load with invalid JSON
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	badFile := filepath.Join(tempDir, "bad_registry.json")
	if err := os.WriteFile(badFile, []byte("invalid json{"), 0600); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	SetRegistryPathOverride(badFile)
	defer func() { SetRegistryPathOverride("") }()

	_, err = Load()
	if err == nil {
		t.Error("expected error when loading invalid json registry, got nil")
	}

	// Test Load failure propagation in Add, Remove, Prune
	if err := Add(tempDir); err == nil {
		t.Error("expected Add to fail when Load fails, got nil")
	}
	if err := Remove(tempDir); err == nil {
		t.Error("expected Remove to fail when Load fails, got nil")
	}
	if err := Prune(); err == nil {
		t.Error("expected Prune to fail when Load fails, got nil")
	}

	// 3. Test Load with null workspaces array
	nullFile := filepath.Join(tempDir, "null_registry.json")
	if err := os.WriteFile(nullFile, []byte(`{"workspaces": null}`), 0600); err != nil {
		t.Fatalf("failed to write null workspaces file: %v", err)
	}

	SetRegistryPathOverride(nullFile)
	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry with null workspaces: %v", err)
	}
	if workspaces == nil || len(workspaces) != 0 {
		t.Errorf("expected empty non-nil workspaces slice, got: %v", workspaces)
	}

	// 4. Test save directory creation error (file/directory conflict)
	conflictFile := filepath.Join(tempDir, "conflict")
	if err := os.WriteFile(conflictFile, []byte("plain file"), 0600); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	// Set override path inside the conflict file path (conflict/registry.json)
	SetRegistryPathOverride(filepath.Join(conflictFile, "registry.json"))
	
	err = Add(tempDir)
	if err == nil {
		t.Error("expected error when directory creation fails due to file conflict, got nil")
	}

	// Test save write error (write to directory)
	// Set override path to a directory (tempDir) so os.WriteFile fails
	SetRegistryPathOverride(tempDir)
	err = save([]string{"/some/path"})
	if err == nil {
		t.Error("expected error when writing to a directory path, got nil")
	}

	// 5. Test path resolution errors (invalid path)
	// Passing an empty directory path to filepath.Abs is handled, but let's test absolute path errors if any.
	// We can check if calling Remove on non-existent registry behaves gracefully.
	SetRegistryPathOverride(filepath.Join(tempDir, "non_existent_registry.json"))
	err = Remove("/some/path/that/is/not/there")
	if err != nil {
		t.Fatalf("expected Remove on non-existent registry to succeed, got error: %v", err)
	}
}

