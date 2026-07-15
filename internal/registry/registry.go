package registry

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Registry represents the structure of the registry JSON file.
type Registry struct {
	Workspaces []string `json:"workspaces"`
}

var registryPathOverride string

// SetRegistryPathOverride overrides the default registry file path for testing purposes.
func SetRegistryPathOverride(path string) {
	registryPathOverride = path
}

// GetRegistryPath resolves the absolute path to the registry JSON file.
// It defaults to ~/.config/pithos/registry.json, falling back to a local
// file in the current working directory if the home directory is inaccessible.
func GetRegistryPath() string {
	if registryPathOverride != "" {
		return registryPathOverride
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "pithos", "registry.json")
	}
	return ".pithos_registry.json"
}

// Load reads and unmarshals the workspace paths from the registry file.
// If the registry file does not exist, it returns an empty slice and no error.
func Load() ([]string, error) {
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
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPath = filepath.Clean(absPath)

	workspaces, err := Load()
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
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPath = filepath.Clean(absPath)

	workspaces, err := Load()
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
// on the local filesystem.
func Prune() error {
	workspaces, err := Load()
	if err != nil {
		return err
	}

	var active []string
	changed := false
	for _, ws := range workspaces {
		if _, err := os.Stat(ws); err == nil {
			active = append(active, ws)
		} else {
			changed = true
		}
	}

	if changed {
		return save(active)
	}
	return nil
}

// save marshals and writes workspace paths to the resolved registry path.
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
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
