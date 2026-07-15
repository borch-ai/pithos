package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestFile(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "pithos_registry_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	regFile := filepath.Join(tempDir, "registry.json")
	SetRegistryPathOverride(regFile)
	t.Cleanup(func() {
		SetRegistryPathOverride("")
	})

	return tempDir
}

func TestRegistryOperations_LoadEmpty(t *testing.T) {
	setupTestFile(t)

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load empty registry: %v", err)
	}
	if len(workspaces) != 0 {
		t.Errorf("expected empty registry, got %d items", len(workspaces))
	}
}

func TestRegistryOperations_AddAndLoad(t *testing.T) {
	setupTestFile(t)

	ws1, err := os.MkdirTemp("", "ws1")
	if err != nil {
		t.Fatalf("failed to create ws1: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws1) })

	ws2, err := os.MkdirTemp("", "ws2")
	if err != nil {
		t.Fatalf("failed to create ws2: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws2) })

	absWS1, _ := filepath.Abs(ws1)
	absWS1 = filepath.Clean(absWS1)
	absWS2, _ := filepath.Abs(ws2)
	absWS2 = filepath.Clean(absWS2)

	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	if err := Add(ws2); err != nil {
		t.Fatalf("failed to add ws2: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0] != absWS1 || workspaces[1] != absWS2 {
		t.Errorf("unexpected registered paths: %v", workspaces)
	}
}

func TestRegistryOperations_AddDuplicate(t *testing.T) {
	setupTestFile(t)

	ws1, err := os.MkdirTemp("", "ws1")
	if err != nil {
		t.Fatalf("failed to create ws1: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws1) })

	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add duplicate: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 {
		t.Errorf("expected duplicate to be ignored, got %d workspaces", len(workspaces))
	}
}

func TestRegistryOperations_Remove(t *testing.T) {
	setupTestFile(t)

	ws1, err := os.MkdirTemp("", "ws1")
	if err != nil {
		t.Fatalf("failed to create ws1: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws1) })

	ws2, err := os.MkdirTemp("", "ws2")
	if err != nil {
		t.Fatalf("failed to create ws2: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws2) })

	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	if err := Add(ws2); err != nil {
		t.Fatalf("failed to add ws2: %v", err)
	}

	if err := Remove(ws1); err != nil {
		t.Fatalf("failed to remove ws1: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace left, got %d", len(workspaces))
	}

	absWS2, _ := filepath.Abs(ws2)
	absWS2 = filepath.Clean(absWS2)
	if workspaces[0] != absWS2 {
		t.Errorf("expected ws2 to remain, got %s", workspaces[0])
	}
}

func TestRegistryOperations_Prune(t *testing.T) {
	tempDir := setupTestFile(t)

	ws1, err := os.MkdirTemp("", "ws1")
	if err != nil {
		t.Fatalf("failed to create ws1: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(ws1) })

	if err := Add(ws1); err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}

	fakePath := filepath.Join(tempDir, "non_existent_dir")
	if err := Add(fakePath); err != nil {
		t.Fatalf("failed to add fake path: %v", err)
	}

	if err := Prune(); err != nil {
		t.Fatalf("failed to prune: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace after pruning, got %d", len(workspaces))
	}

	absWS1, _ := filepath.Abs(ws1)
	absWS1 = filepath.Clean(absWS1)
	if workspaces[0] != absWS1 {
		t.Errorf("expected only ws1 to remain after pruning, got %s", workspaces[0])
	}
}

func TestRegistryEdgeCases_GetRegistryPath(t *testing.T) {
	SetRegistryPathOverride("")
	path := GetRegistryPath()
	if path == "" {
		t.Error("expected non-empty path from GetRegistryPath")
	}

	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", "")
	oldUserProfile := os.Getenv("USERPROFILE")
	_ = os.Setenv("USERPROFILE", "")

	fallbackPath := GetRegistryPath()
	if fallbackPath != ".pithos_registry.json" {
		t.Errorf("expected fallback path .pithos_registry.json, got %s", fallbackPath)
	}

	_ = os.Setenv("HOME", oldHome)
	_ = os.Setenv("USERPROFILE", oldUserProfile)
}

func TestRegistryEdgeCases_LoadFailurePropagation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	badFile := filepath.Join(tempDir, "bad_registry.json")
	if err := os.WriteFile(badFile, []byte("invalid json{"), 0600); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	SetRegistryPathOverride(badFile)
	t.Cleanup(func() { SetRegistryPathOverride("") })

	_, err = Load()
	if err == nil {
		t.Error("expected error when loading invalid json registry, got nil")
	}

	if err := Add(tempDir); err == nil {
		t.Error("expected Add to fail when Load fails, got nil")
	}
	if err := Remove(tempDir); err == nil {
		t.Error("expected Remove to fail when Load fails, got nil")
	}
	if err := Prune(); err == nil {
		t.Error("expected Prune to fail when Load fails, got nil")
	}
}

func TestRegistryEdgeCases_NullWorkspaces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	nullFile := filepath.Join(tempDir, "null_registry.json")
	if err := os.WriteFile(nullFile, []byte(`{"workspaces": null}`), 0600); err != nil {
		t.Fatalf("failed to write null workspaces file: %v", err)
	}

	SetRegistryPathOverride(nullFile)
	t.Cleanup(func() { SetRegistryPathOverride("") })

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load registry with null workspaces: %v", err)
	}
	if workspaces == nil || len(workspaces) != 0 {
		t.Errorf("expected empty non-nil workspaces slice, got: %v", workspaces)
	}
}

func TestRegistryEdgeCases_SaveFailures(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	conflictFile := filepath.Join(tempDir, "conflict")
	if err := os.WriteFile(conflictFile, []byte("plain file"), 0600); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	SetRegistryPathOverride(filepath.Join(conflictFile, "registry.json"))
	t.Cleanup(func() { SetRegistryPathOverride("") })

	err = Add(tempDir)
	if err == nil {
		t.Error("expected error when directory creation fails due to file conflict, got nil")
	}

	SetRegistryPathOverride(tempDir)
	err = save([]string{"/some/path"})
	if err == nil {
		t.Error("expected error when writing to a directory path, got nil")
	}
}

func TestRegistryEdgeCases_RemoveNonExistent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	SetRegistryPathOverride(filepath.Join(tempDir, "non_existent_registry.json"))
	t.Cleanup(func() { SetRegistryPathOverride("") })

	err = Remove("/some/path/that/is/not/there")
	if err != nil {
		t.Fatalf("expected Remove on non-existent registry to succeed, got error: %v", err)
	}
}
