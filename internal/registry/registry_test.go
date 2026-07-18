package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
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

	regularFile := filepath.Join(tempDir, "regular_file.txt")
	err = os.WriteFile(regularFile, []byte("data"), 0600)
	if err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}
	err = Add(regularFile)
	if err != nil {
		t.Fatalf("failed to add regular file path: %v", err)
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

	// Probe if traversal permission restriction was actually enforced
	_, statErr := os.Stat(filepath.Join(secretParent, "registry.json"))
	if statErr == nil || errors.Is(statErr, os.ErrNotExist) {
		t.Skip("skipping test: filesystem traversal was not blocked by chmod(0000)")
	}

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

	// Probe if traversal permission restriction was actually enforced
	_, statErr := os.Stat(wsDir)
	if statErr == nil || errors.Is(statErr, os.ErrNotExist) {
		t.Skip("skipping test: filesystem traversal was not blocked by chmod(0000)")
	}

	err = Prune()
	if err == nil {
		t.Error("expected error when Prune stats a directory with permission denied, got nil")
	}
}

func TestRegistryEdgeCases_SaveWindowsFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_windows_fallback")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Set destination path to a regular file
	registryFile := filepath.Join(tempDir, "registry.json")
	err = os.WriteFile(registryFile, []byte("{}"), 0600)
	if err != nil {
		t.Fatalf("failed to create registry file: %v", err)
	}

	oldOverride := SetRegistryPathOverride(registryFile)
	t.Cleanup(func() { SetRegistryPathOverride(oldOverride) })

	// Mock Windows platform
	oldIsWindows := isWindows
	isWindows = true
	defer func() { isWindows = oldIsWindows }()

	// Mock renameFunc to fail on first call and succeed on second call
	renameCallCount := 0
	oldRenameFunc := renameFunc
	renameFunc = func(oldpath, newpath string) error {
		renameCallCount++
		if renameCallCount == 1 {
			return os.ErrExist
		}
		return oldRenameFunc(oldpath, newpath)
	}
	defer func() { renameFunc = oldRenameFunc }()

	// Calling save should trigger the rename error, enter fallback, remove the file,
	// and successfully rename the temp file to registry.json.
	err = save([]string{"/test/path"})
	if err != nil {
		t.Fatalf("expected save to succeed via Windows fallback, got error: %v", err)
	}

	if renameCallCount != 2 {
		t.Errorf("expected rename to be called twice, got %d", renameCallCount)
	}

	// Verify it successfully wrote the registry using unlocked load
	workspaces, err := loadUnlocked()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(workspaces) != 1 || workspaces[0] != "/test/path" {
		t.Errorf("unexpected workspaces loaded: %v", workspaces)
	}
}

func TestRegistryEdgeCases_SaveWindowsFallbackDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_windows_fallback_dir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Set destination path to a directory
	registryDir := filepath.Join(tempDir, "registry.json")
	err = os.Mkdir(registryDir, 0750)
	if err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	oldOverride := SetRegistryPathOverride(registryDir)
	t.Cleanup(func() { SetRegistryPathOverride(oldOverride) })

	// Mock Windows platform
	oldIsWindows := isWindows
	isWindows = true
	defer func() { isWindows = oldIsWindows }()

	// Mock renameFunc to fail on first call
	renameCallCount := 0
	oldRenameFunc := renameFunc
	renameFunc = func(oldpath, newpath string) error {
		renameCallCount++
		return os.ErrExist
	}
	defer func() { renameFunc = oldRenameFunc }()

	// Calling save should trigger the rename error, see that destination is a directory,
	// and refuse to delete it, returning the original error.
	err = save([]string{"/test/path"})
	if err == nil {
		t.Fatal("expected save to fail when destination is a directory, got nil")
	}

	if renameCallCount != 1 {
		t.Errorf("expected rename to be called once, got %d", renameCallCount)
	}

	// Verify the directory still exists
	fi, statErr := os.Stat(registryDir)
	if statErr != nil {
		t.Fatalf("expected directory to still exist, got stat error: %v", statErr)
	}
	if !fi.IsDir() {
		t.Error("expected destination to remain a directory")
	}
}

func TestRegistryOperations_Concurrent(t *testing.T) {
	setupTestFile(t)

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				path := fmt.Sprintf("/some/path/%d/%d", workerID, j)
				if err := Add(path); err != nil {
					t.Errorf("Add failed concurrently: %v", err)
				}
				if _, err := Load(); err != nil {
					t.Errorf("Load failed concurrently: %v", err)
				}
				if err := Remove(path); err != nil {
					t.Errorf("Remove failed concurrently: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

type mockFileWriter struct {
	writeFunc func(p []byte) (n int, err error)
	syncFunc  func() error
	closeFunc func() error
	nameFunc  func() string
}

func (m *mockFileWriter) Write(p []byte) (n int, err error) {
	if m.writeFunc != nil {
		return m.writeFunc(p)
	}
	return len(p), nil
}

func (m *mockFileWriter) Sync() error {
	if m.syncFunc != nil {
		return m.syncFunc()
	}
	return nil
}

func (m *mockFileWriter) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockFileWriter) Name() string {
	if m.nameFunc != nil {
		return m.nameFunc()
	}
	return "mock.tmp"
}

func TestRegistryEdgeCases_SaveFileWriterErrors(t *testing.T) {
	setupTestFile(t)

	// Mock fileWriter to return Write error
	oldCreateTempFile := createTempFile
	defer func() { createTempFile = oldCreateTempFile }()

	// 1. Test Write Error
	createTempFile = func(dir, pattern string) (fileWriter, error) {
		return &mockFileWriter{
			writeFunc: func(p []byte) (n int, err error) {
				return 0, os.ErrPermission
			},
		}, nil
	}
	err := save([]string{"/test"})
	if err == nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected write permission error, got %v", err)
	}

	// 2. Test Short Write
	createTempFile = func(dir, pattern string) (fileWriter, error) {
		return &mockFileWriter{
			writeFunc: func(p []byte) (n int, err error) {
				return len(p) - 1, nil
			},
		}, nil
	}
	err = save([]string{"/test"})
	if err == nil || !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("expected short write error, got %v", err)
	}

	// 3. Test Sync Error
	createTempFile = func(dir, pattern string) (fileWriter, error) {
		return &mockFileWriter{
			syncFunc: func() error {
				return os.ErrInvalid
			},
		}, nil
	}
	err = save([]string{"/test"})
	if err == nil || !errors.Is(err, os.ErrInvalid) {
		t.Errorf("expected sync invalid error, got %v", err)
	}

	// 4. Test Close Error
	createTempFile = func(dir, pattern string) (fileWriter, error) {
		return &mockFileWriter{
			closeFunc: func() error {
				return os.ErrClosed
			},
		}, nil
	}
	err = save([]string{"/test"})
	if err == nil || !errors.Is(err, os.ErrClosed) {
		t.Errorf("expected close closed error, got %v", err)
	}
}

func TestRegistryEdgeCases_SaveWindowsFallback_OtherErrors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_windows_fallback_other")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	// Set destination path to a regular file
	registryFile := filepath.Join(tempDir, "registry.json")
	err = os.WriteFile(registryFile, []byte("existing"), 0600)
	if err != nil {
		t.Fatalf("failed to create registry file: %v", err)
	}

	oldOverride := SetRegistryPathOverride(registryFile)
	t.Cleanup(func() { SetRegistryPathOverride(oldOverride) })

	// Mock Windows platform
	oldIsWindows := isWindows
	isWindows = true
	defer func() { isWindows = oldIsWindows }()

	// Mock renameFunc to fail with permission error (not isExist error)
	oldRenameFunc := renameFunc
	renameFunc = func(oldpath, newpath string) error {
		return os.ErrPermission
	}
	defer func() { renameFunc = oldRenameFunc }()

	// Calling save should trigger the rename error, see that it is not os.IsExist,
	// and return the permission error without deleting registry.json.
	err = save([]string{"/test/path"})
	if err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected save to fail with permission error, got: %v", err)
	}

	// Verify registry.json still exists and was not deleted/modified
	// #nosec G304
	data, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatalf("expected registry file to still exist, got: %v", err)
	}
	if string(data) != "existing" {
		t.Errorf("expected registry file content to be unchanged, got %q", string(data))
	}
}

func TestRegistryOperations_RemovePaths(t *testing.T) {
	setupTestFile(t)

	err := Add("/path/1")
	if err != nil {
		t.Fatalf("failed to add path 1: %v", err)
	}
	err = Add("/path/2")
	if err != nil {
		t.Fatalf("failed to add path 2: %v", err)
	}
	err = Add("/path/3")
	if err != nil {
		t.Fatalf("failed to add path 3: %v", err)
	}

	// 1. Test empty paths slice
	err = RemovePaths([]string{})
	if err != nil {
		t.Fatalf("expected no-op on empty slice, got: %v", err)
	}

	// 2. Test batch removal
	err = RemovePaths([]string{"/path/1", "/path/3", "/non-existent"})
	if err != nil {
		t.Fatalf("failed to remove paths: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("failed to load workspaces: %v", err)
	}

	if len(workspaces) != 1 {
		t.Fatalf("expected 1 workspace remaining, got %d", len(workspaces))
	}
	absPath2, _ := filepath.Abs("/path/2")
	absPath2 = filepath.Clean(absPath2)
	if workspaces[0] != absPath2 {
		t.Errorf("expected path/2 to remain, got %q", workspaces[0])
	}
}

func TestRegistryOperations_EmptyPaths(t *testing.T) {
	setupTestFile(t)

	err := Add("")
	if err == nil {
		t.Error("expected Add(\"\") to fail, got nil")
	}

	err = Remove("")
	if err == nil {
		t.Error("expected Remove(\"\") to fail, got nil")
	}
}

func TestRegistryOperations_LoadNormalization(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pithos_registry_normalization")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	registryFile := filepath.Join(tempDir, "registry.json")
	oldOverride := SetRegistryPathOverride(registryFile)
	t.Cleanup(func() { SetRegistryPathOverride(oldOverride) })

	// Write non-canonical, relative, empty, and duplicate paths to registry.json
	regData := Registry{
		Workspaces: []string{
			"relative/path/1",
			"",
			"relative/path/1",
			"another/path",
		},
	}
	data, err := json.Marshal(regData)
	if err != nil {
		t.Fatalf("failed to marshal registry: %v", err)
	}
	err = os.WriteFile(registryFile, data, 0600)
	if err != nil {
		t.Fatalf("failed to write registry: %v", err)
	}

	workspaces, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(workspaces) != 2 {
		t.Fatalf("expected exactly 2 normalized workspaces, got %d", len(workspaces))
	}

	abs1, _ := filepath.Abs("relative/path/1")
	abs1 = filepath.Clean(abs1)
	abs2, _ := filepath.Abs("another/path")
	abs2 = filepath.Clean(abs2)

	if workspaces[0] != abs1 {
		t.Errorf("expected %s, got %s", abs1, workspaces[0])
	}
	if workspaces[1] != abs2 {
		t.Errorf("expected %s, got %s", abs2, workspaces[1])
	}
}
