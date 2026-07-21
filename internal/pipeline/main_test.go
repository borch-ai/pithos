package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/registry"
)

var TestPithosBinaryPath string
var TestPithosBinaryCleanup func()

func TestMain(m *testing.M) {
	// Setup package-wide temporary registry override to isolate tests
	// from the user's live config files.
	tmpDir, err := os.MkdirTemp("", "pithos_pipeline_test_registry")
	if err != nil {
		panic(err)
	}
	oldOverride := registry.SetRegistryPathOverride(filepath.Join(tmpDir, "registry.json"))

	code := m.Run()

	if TestPithosBinaryCleanup != nil {
		TestPithosBinaryCleanup()
	}

	registry.SetRegistryPathOverride(oldOverride)
	_ = os.RemoveAll(tmpDir)

	os.Exit(code)
}
