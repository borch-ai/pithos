package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/powerword/pkg/gitutil"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCheckpoint_RealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	// 1. Create a temp directory
	tmpDir := t.TempDir()

	ctx := context.Background()

	// Write a test file to verify that staging and committing works and creates a HEAD commit.
	initialFile := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(initialFile, []byte(`{"status":"initiated"}`), 0600); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	// 2. Call Checkpoint in the temp directory (should initialize git, config user, and commit)
	err := Checkpoint(ctx, tmpDir, "Initial checkpoint")
	if err != nil {
		t.Fatalf("expected Checkpoint to succeed, got %v", err)
	}

	// 3. Verify that git repository was initialized and is inside work tree
	inside, err := gitutil.IsInsideWorkTree(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to check if workspace is inside worktree: %v", err)
	}
	if !inside {
		t.Error("expected directory to be inside git worktree after checkpoint")
	}

	// 4. Verify that HEAD commit exists
	commit1, err := gitutil.GetHeadCommit(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to get head commit: %v", err)
	}
	if len(commit1) == 0 {
		t.Error("expected non-empty commit hash")
	}

	// 5. Check git log output to make sure message matches
	logOut, err := gitutil.RunGitCommand(ctx, tmpDir, "log", "--oneline")
	if err != nil {
		t.Fatalf("failed to run git log: %v", err)
	}
	if !strings.Contains(logOut, "Initial checkpoint") {
		t.Errorf("expected log output to contain 'Initial checkpoint', got: %q", logOut)
	}

	// 6. Write a file and create a second checkpoint
	testFile := filepath.Join(tmpDir, "test.txt")
	if writeErr := os.WriteFile(testFile, []byte("updated content"), 0600); writeErr != nil {
		t.Fatalf("failed to write test file: %v", writeErr)
	}

	err = Checkpoint(ctx, tmpDir, "Second checkpoint")
	if err != nil {
		t.Fatalf("expected second Checkpoint to succeed, got %v", err)
	}

	// 7. Verify git log has both commits
	logOut2, err := gitutil.RunGitCommand(ctx, tmpDir, "log", "--oneline")
	if err != nil {
		t.Fatalf("failed to run git log: %v", err)
	}
	if !strings.Contains(logOut2, "Second checkpoint") || !strings.Contains(logOut2, "Initial checkpoint") {
		t.Errorf("expected log output to contain both checkpoints, got: %q", logOut2)
	}

	// 8. Call checkpoint again with no changes (should ignore "nothing to commit" and succeed)
	err = Checkpoint(ctx, tmpDir, "Third checkpoint")
	if err != nil {
		t.Fatalf("expected third Checkpoint with no changes to succeed, got %v", err)
	}
}

func TestCheckpoint_NoGitInPath(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", origPath) }()

	// Temporarily break PATH so git cannot be found
	_ = os.Setenv("PATH", "")

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Should not fail and should exit early returning nil
	err := Checkpoint(ctx, tmpDir, "No git checkpoint")
	if err != nil {
		t.Errorf("expected Checkpoint to succeed with warning when git is missing, got: %v", err)
	}
}

func TestCheckpoint_NonExistentDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "workspace")

	ctx := context.Background()
	// Checkpoint on a non-existent directory should create it and succeed
	err := Checkpoint(ctx, nestedDir, "Setup nested workspace")
	if err != nil {
		t.Fatalf("expected Checkpoint to succeed on non-existent directory, got %v", err)
	}

	if _, statErr := os.Stat(nestedDir); os.IsNotExist(statErr) {
		t.Error("expected Checkpoint to create directory")
	}

	inside, err := gitutil.IsInsideWorkTree(ctx, nestedDir)
	if err != nil || !inside {
		t.Error("expected nested directory to be inside git worktree")
	}
}

func TestCheckpoint_MockInitError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Write a file so it has something to commit
	initialFile := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(initialFile, []byte(`{"status":"initiated"}`), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "init" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	err := Checkpoint(ctx, tmpDir, "Test Init Error")
	if err == nil || !strings.Contains(err.Error(), "failed to initialize git repository") {
		t.Errorf("expected init failure error, got: %v", err)
	}
}

func TestCheckpoint_MockAddAllError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()
	ctx := context.Background()

	initialFile := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(initialFile, []byte(`{"status":"initiated"}`), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "add" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	err := Checkpoint(ctx, tmpDir, "Test Add Error")
	if err == nil || !strings.Contains(err.Error(), "failed to add files to git staging") {
		t.Errorf("expected add failure error, got: %v", err)
	}
}

func TestCheckpoint_MockCommitError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()
	ctx := context.Background()

	initialFile := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(initialFile, []byte(`{"status":"initiated"}`), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "commit" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	err := Checkpoint(ctx, tmpDir, "Test Commit Error")
	if err == nil || !strings.Contains(err.Error(), "failed to commit changes") {
		t.Errorf("expected commit failure error, got: %v", err)
	}
}

func TestCheckpoint_MockConfigError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()
	ctx := context.Background()

	initialFile := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(initialFile, []byte(`{"status":"initiated"}`), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 1 && args[0] == "config" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	err := Checkpoint(ctx, tmpDir, "Test Config Error")
	if err != nil {
		t.Errorf("expected checkpoint to succeed despite config error, got %v", err)
	}
}

func TestCheckpoint_InitiateError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()

	// Mock git commit to fail
	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "commit" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	opts := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err := Initiate(opts)
	if err == nil || !strings.Contains(err.Error(), "failed to create initial git checkpoint") {
		t.Errorf("expected initiate to fail due to checkpoint failure, got: %v", err)
	}
}

func TestCheckpoint_BrewError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()

	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 1,
	}
	_, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Mock git commit to fail
	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "commit" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	mockStanzas := []string{"Stanza 1"}
	mockLLMClient := &mockLLM{
		stanzas: mockStanzas,
	}

	optsBrew := BrewOptions{
		OutputDir: tmpDir,
		LLM:       mockLLMClient,
	}

	ctx := context.Background()
	err = Brew(ctx, optsBrew)
	if err == nil || !strings.Contains(err.Error(), "failed to commit changes") {
		t.Errorf("expected brew to fail due to checkpoint error, got: %v", err)
	}
}

func TestCheckpoint_MkdirAllError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	noWriteDir := filepath.Join(tmpDir, "nowrite")
	if err := os.Mkdir(noWriteDir, 0500); err != nil {
		t.Fatalf("failed to create nowrite dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(noWriteDir, 0700) // #nosec G302
	}()

	err := Checkpoint(context.Background(), filepath.Join(noWriteDir, "nested"), "Should fail on MkdirAll")
	if err == nil || !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("expected failed to create directory error, got: %v", err)
	}
}

func TestCheckpoint_StatNotDirError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")
	if err := os.WriteFile(filePath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err := Checkpoint(context.Background(), filepath.Join(filePath, "nested"), "Should fail on ENOTDIR stat")
	if err == nil || !strings.Contains(err.Error(), "failed to stat directory") {
		t.Errorf("expected failed to stat directory error, got: %v", err)
	}
}

func TestCheckpoint_AssembleError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	origExec := gitutil.ExecCommand
	defer func() { gitutil.ExecCommand = origExec }()

	tmpDir := t.TempDir()

	// Set up initiate
	optsInit := InitiateOptions{
		OutputDir:       tmpDir,
		Theme:           "Theme",
		TargetPageCount: 75, // hardcover minimum
	}
	_, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Mock git commit to fail
	gitutil.ExecCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		if command == "git" && len(args) > 0 && args[0] == "commit" {
			args = append(args, "--definitely-not-a-valid-flag-to-force-failure")
			return origExec(ctx, command, args...)
		}
		return origExec(ctx, command, args...)
	}

	// Mock KDP math and Typst client transports
	kdpMathTransport, serverKDP := mcpsdk.NewInMemoryTransports()
	typstTransport, serverTypst := mcpsdk.NewInMemoryTransports()

	// Create minimal mock servers for math and typst
	ctx := context.Background()
	serverMath := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "mock-math"}, nil)
	serverMath.AddTool(&mcpsdk.Tool{
		Name: "kdp_calculate_geometry",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"spine_width_inches":0.5}`}}}, nil
	})
	sessionMath, _ := serverMath.Connect(ctx, serverKDP, nil)
	defer func() { _ = sessionMath.Close() }()

	serverT := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "mock-typst"}, nil)
	serverT.AddTool(&mcpsdk.Tool{
		Name: "compile_interior",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"output_pdf":"interior.pdf"}`}}}, nil
	})
	sessionTypst, _ := serverT.Connect(ctx, serverTypst, nil)
	defer func() { _ = sessionTypst.Close() }()

	optsAssemble := AssembleOptions{
		InputDir:         tmpDir,
		Format:           "hardcover",
		KDPMathTransport: kdpMathTransport,
		TypstTransport:   typstTransport,
	}

	_, err = Assemble(ctx, optsAssemble)
	if err == nil || !strings.Contains(err.Error(), "failed to commit changes") {
		t.Errorf("expected Assemble to fail due to checkpoint commit error, got: %v", err)
	}
}

func TestCheckpoint_PathIsFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err := Checkpoint(context.Background(), filePath, "Should fail on file")
	if err == nil || !strings.Contains(err.Error(), "exists but is not a directory") {
		t.Errorf("expected exists but is not a directory error, got: %v", err)
	}
}

func TestCheckpoint_GitDirStatError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	noPermDir := filepath.Join(tmpDir, "noperm")
	if err := os.Mkdir(noPermDir, 0700); err != nil {
		t.Fatalf("failed to create noperm dir: %v", err)
	}

	gitDir := filepath.Join(noPermDir, ".git")
	if err := os.Mkdir(gitDir, 0700); err != nil {
		t.Fatalf("failed to create git dir: %v", err)
	}

	// Change permissions of noPermDir to 0000 so stat on gitDir fails with permission denied
	if err := os.Chmod(noPermDir, 0000); err != nil {
		t.Fatalf("failed to chmod noperm dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(noPermDir, 0700) // #nosec G302
	}()

	err := Checkpoint(context.Background(), noPermDir, "Should fail on git dir stat")
	if err == nil || !strings.Contains(err.Error(), "failed to stat git directory") {
		t.Errorf("expected stat git directory error, got: %v", err)
	}
}

func TestCheckpoint_DirStatError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("skipping test: git executable not found in PATH")
	}

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "noperm")
	if err := os.Mkdir(parentDir, 0700); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	targetDir := filepath.Join(parentDir, "target")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	// Change permissions of parentDir to 0000 so stat targetDir fails with permission denied
	if err := os.Chmod(parentDir, 0000); err != nil {
		t.Fatalf("failed to chmod parent dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(parentDir, 0700) // #nosec G302
	}()

	err := Checkpoint(context.Background(), targetDir, "Should fail on dir stat")
	if err == nil || !strings.Contains(err.Error(), "failed to stat directory") {
		t.Errorf("expected stat directory error, got: %v", err)
	}
}
