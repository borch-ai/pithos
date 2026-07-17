package registry

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Registry represents the structure of the registry JSON file.
type Registry struct {
	Workspaces []string `json:"workspaces"`
}

var userHomeDir = os.UserHomeDir

var isWindows = runtime.GOOS == "windows"

var mu sync.Mutex

var renameFunc = os.Rename

type fileWriter interface {
	Write(p []byte) (n int, err error)
	Sync() error
	Close() error
	Name() string
}

var createTempFile = func(dir, pattern string) (fileWriter, error) {
	return os.CreateTemp(dir, pattern)
}

var (
	registryPathOverride string
	overrideMu           sync.RWMutex
)

// SetRegistryPathOverride overrides the default registry file path for testing purposes.
// It returns the previous override value, allowing callers to restore it via defer.
func SetRegistryPathOverride(path string) string {
	overrideMu.Lock()
	defer overrideMu.Unlock()
	old := registryPathOverride
	registryPathOverride = path
	return old
}

// GetRegistryPath resolves the absolute path to the registry JSON file.
// It defaults to ~/.config/pithos/registry.json, falling back to a local
// file in the current working directory if the home directory is inaccessible.
func GetRegistryPath() string {
	overrideMu.RLock()
	path := registryPathOverride
	overrideMu.RUnlock()

	if path == "" {
		if home, err := userHomeDir(); err == nil {
			path = filepath.Join(home, ".config", "pithos", "registry.json")
		} else {
			path = ".pithos_registry.json"
		}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

// Load reads and unmarshals the workspace paths from the registry file.
// If the registry file does not exist, it returns an empty slice and no error.
func Load() ([]string, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadUnlocked()
}

func loadUnlocked() ([]string, error) {
	path := GetRegistryPath()
	// #nosec G304
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}

	if reg.Workspaces == nil {
		reg.Workspaces = []string{}
	}

	return reg.Workspaces, nil
}

// Add resolves the absolute path of a workspace and appends it to the registry
// if it is not already present.
func Add(path string) error {
	mu.Lock()
	defer mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPath = filepath.Clean(absPath)

	workspaces, err := loadUnlocked()
	if err != nil {
		return err
	}

	// Check if already registered
	for _, ws := range workspaces {
		if ws == absPath {
			return nil
		}
	}

	workspaces = append(workspaces, absPath)
	return save(workspaces)
}

// Remove cleans the given path, removes it from the registry, and saves the updates.
func Remove(path string) error {
	mu.Lock()
	defer mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPath = filepath.Clean(absPath)

	workspaces, err := loadUnlocked()
	if err != nil {
		return err
	}

	var updated []string
	for _, ws := range workspaces {
		if ws != absPath {
			updated = append(updated, ws)
		}
	}

	return save(updated)
}

// Prune validates all registered paths and removes any that no longer exist
// or are not directories on the local filesystem.
func Prune() error {
	mu.Lock()
	defer mu.Unlock()

	workspaces, err := loadUnlocked()
	if err != nil {
		return err
	}

	var active []string
	changed := false
	for _, ws := range workspaces {
		fi, err := os.Stat(ws)
		switch {
		case err == nil:
			if fi.IsDir() {
				active = append(active, ws)
			} else {
				changed = true
			}
		case errors.Is(err, os.ErrNotExist):
			changed = true
		default:
			return err
		}
	}

	if changed {
		return save(active)
	}
	return nil
}

// save marshals and writes workspace paths to the resolved registry path atomically.
func save(workspaces []string) error {
	if workspaces == nil {
		workspaces = []string{}
	}

	reg := Registry{Workspaces: workspaces}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	path := GetRegistryPath()
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return err
	}

	// Write atomically using temporary file in same directory
	tmpFile, err := createTempFile(dir, "registry-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	n, writeErr := tmpFile.Write(data)
	if writeErr != nil {
		return writeErr
	}
	if n < len(data) {
		return io.ErrShortWrite
	}
	if syncErr := tmpFile.Sync(); syncErr != nil {
		return syncErr
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		return closeErr
	}

	err = renameFunc(tmpPath, path)
	if err == nil || !isWindows {
		return err
	}

	if !os.IsExist(err) && !errors.Is(err, os.ErrExist) {
		return err
	}

	fi, statErr := os.Stat(path)
	if statErr != nil {
		return err
	}

	if fi.IsDir() {
		return err
	}

	// On Windows, os.Rename might fail if the destination file already exists.
	// We fallback to removing the destination and renaming.
	if removeErr := os.Remove(path); removeErr != nil {
		return removeErr
	}
	return renameFunc(tmpPath, path)
}
