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
	old := SetRegistryPathOverride(regFile)
	t.Cleanup(func() {
		SetRegistryPathOverride(old)
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

	err = Add(ws1)
	if err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	err = Add(ws2)
	if err != nil {
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

	err = Add(ws1)
	if err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	err = Add(ws1)
	if err != nil {
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

	err = Add(ws1)
	if err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}
	err = Add(ws2)
	if err != nil {
		t.Fatalf("failed to add ws2: %v", err)
	}

	err = Remove(ws1)
	if err != nil {
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

	err = Add(ws1)
	if err != nil {
		t.Fatalf("failed to add ws1: %v", err)
	}

	fakePath := filepath.Join(tempDir, "non_existent_dir")
	err = Add(fakePath)
	if err != nil {
		t.Fatalf("failed to add fake path: %v", err)
	}

	err = Prune()
	if err != nil {
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
	old := SetRegistryPathOverride("")
	defer SetRegistryPathOverride(old)
	path := GetRegistryPath()
	if path == "" {
		t.Error("expected non-empty path from GetRegistryPath")
	}

	oldUserHomeDir := userHomeDir
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}
	defer func() { userHomeDir = oldUserHomeDir }()

	fallbackPath := GetRegistryPath()
	expectedPath, _ := filepath.Abs(".pithos_registry.json")
	if fallbackPath != filepath.Clean(expectedPath) {
		t.Errorf("expected fallback path %s, got %s", expectedPath, fallbackPath)
	}
}

func TestRegistryEdgeCases_LoadFailurePropagation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	badFile := filepath.Join(tempDir, "bad_registry.json")
	err = os.WriteFile(badFile, []byte("invalid json{"), 0600)
	if err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	old := SetRegistryPathOverride(badFile)
	t.Cleanup(func() { SetRegistryPathOverride(old) })

	_, err = Load()
	if err == nil {
		t.Error("expected error when loading invalid json registry, got nil")
	}

	err = Add(tempDir)
	if err == nil {
		t.Error("expected Add to fail when Load fails, got nil")
	}
	err = Remove(tempDir)
	if err == nil {
		t.Error("expected Remove to fail when Load fails, got nil")
	}
	err = Prune()
	if err == nil {
		t.Error("expected Prune to fail when Load fails, got nil")
	}

	// Test Load failure (os.Open permission denied)
	secretParent := filepath.Join(tempDir, "secret_parent_load")
	err = os.Mkdir(secretParent, 0750)
	if err != nil {
		t.Fatalf("failed to create secret parent: %v", err)
	}
	SetRegistryPathOverride(filepath.Join(secretParent, "registry.json"))
	err = os.Chmod(secretParent, 0000)
	if err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(secretParent, 0750) }()

	_, err = Load()
	if err == nil {
		t.Error("expected error when Load fails due to permission denied on opening, got nil")
	}
}

func TestRegistryEdgeCases_NullWorkspaces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	nullFile := filepath.Join(tempDir, "null_registry.json")
	err = os.WriteFile(nullFile, []byte(`{"workspaces": null}`), 0600)
	if err != nil {
		t.Fatalf("failed to write null workspaces file: %v", err)
	}

	old := SetRegistryPathOverride(nullFile)
	t.Cleanup(func() { SetRegistryPathOverride(old) })

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
	err = os.WriteFile(conflictFile, []byte("plain file"), 0600)
	if err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}

	old := SetRegistryPathOverride(filepath.Join(conflictFile, "registry.json"))
	t.Cleanup(func() { SetRegistryPathOverride(old) })

	err = Add(tempDir)
	if err == nil {
		t.Error("expected error when directory creation fails due to file conflict, got nil")
	}

	// Test os.CreateTemp failure by using a readonly directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err = os.Mkdir(readOnlyDir, 0750)
	if err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	// #nosec G302
	err = os.Chmod(readOnlyDir, 0500)
	if err != nil {
		t.Fatalf("failed to chmod readonly dir: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(readOnlyDir, 0750) }()

	oldReadOnly := SetRegistryPathOverride(filepath.Join(readOnlyDir, "registry.json"))
	defer SetRegistryPathOverride(oldReadOnly)
	err = Add(tempDir)
	if err == nil {
		t.Error("expected error when CreateTemp fails inside readonly dir, got nil")
	}

	oldTemp := SetRegistryPathOverride(tempDir)
	defer SetRegistryPathOverride(oldTemp)
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

	old := SetRegistryPathOverride(filepath.Join(tempDir, "non_existent_registry.json"))
	t.Cleanup(func() { SetRegistryPathOverride(old) })

	err = Remove("/some/path/that/is/not/there")
	if err != nil {
		t.Fatalf("expected Remove on non-existent registry to succeed, got error: %v", err)
	}
}

func TestRegistryEdgeCases_PruneFailures(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_edge_cases")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Create secret parent and nested workspace directory
	secretParent := filepath.Join(tempDir, "secret_parent")
	err = os.Mkdir(secretParent, 0750)
	if err != nil {
		t.Fatalf("failed to create secret parent dir: %v", err)
	}
	wsDir := filepath.Join(secretParent, "ws")
	err = os.Mkdir(wsDir, 0750)
	if err != nil {
		t.Fatalf("failed to create nested ws dir: %v", err)
	}

	regFile := filepath.Join(tempDir, "registry.json")
	old := SetRegistryPathOverride(regFile)
	t.Cleanup(func() { SetRegistryPathOverride(old) })

	// Add nested workspace to registry while parent is accessible
	err = Add(wsDir)
	if err != nil {
		t.Fatalf("failed to add ws dir: %v", err)
	}

	// Make parent inaccessible to trigger Stat permission error on the nested path
	err = os.Chmod(secretParent, 0000)
	if err != nil {
		t.Fatalf("failed to chmod secret parent dir: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(secretParent, 0750) }()

	err = Prune()
	if err == nil {
		t.Error("expected error when Prune stats a directory with permission denied, got nil")
	}
}
