package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/registry"
)

func TestMain(m *testing.M) {
	// Setup package-wide temporary registry override to isolate tests
	// from the user's live config files.
	tmpDir, err := os.MkdirTemp("", "pithos_pipeline_test_registry")
	if err != nil {
		panic(err)
	}
	oldOverride := registry.SetRegistryPathOverride(filepath.Join(tmpDir, "registry.json"))

	code := m.Run()

	registry.SetRegistryPathOverride(oldOverride)
	_ = os.RemoveAll(tmpDir)

	os.Exit(code)
}
