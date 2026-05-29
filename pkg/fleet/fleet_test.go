package fleet

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// mockRunner records calls for testing.
type mockRunner struct {
	mu        sync.Mutex
	upCalls   []runnerCall
	downCalls []runnerCall
	psCalls   []runnerCall
	upErr     error
	downErr   error
	psOutput  string
	psErr     error
}

type runnerCall struct {
	composeFile string
	projectName string
}

func (m *mockRunner) Up(ctx context.Context, composeFile string, projectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upCalls = append(m.upCalls, runnerCall{composeFile, projectName})
	return m.upErr
}

func (m *mockRunner) Down(ctx context.Context, composeFile string, projectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downCalls = append(m.downCalls, runnerCall{composeFile, projectName})
	return m.downErr
}

func (m *mockRunner) Ps(ctx context.Context, composeFile string, projectName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.psCalls = append(m.psCalls, runnerCall{composeFile, projectName})
	return m.psOutput, m.psErr
}

func TestFleet_Up(t *testing.T) {
	dir := t.TempDir()
	setupTestFleet(t, dir)

	runner := &mockRunner{}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: filepath.Join(dir, ".agent-fleet"),
		Runner:    runner,
	})

	err := f.Up(context.Background())
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Verify compose file was written
	composeFile := filepath.Join(dir, ".agent-fleet", "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Error("compose file not written")
	}

	// Verify runner was called
	if len(runner.upCalls) != 1 {
		t.Fatalf("Up() called %d times, want 1", len(runner.upCalls))
	}
	if runner.upCalls[0].projectName != "test-fleet" {
		t.Errorf("projectName = %q, want %q", runner.upCalls[0].projectName, "test-fleet")
	}
	if runner.upCalls[0].composeFile != composeFile {
		t.Errorf("composeFile = %q, want %q", runner.upCalls[0].composeFile, composeFile)
	}
}

func TestFleet_Up_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Write invalid fleet.yaml
	os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte("invalid: yaml: ["), 0644)

	runner := &mockRunner{}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		Runner:    runner,
	})

	err := f.Up(context.Background())
	if err == nil {
		t.Error("expected error for invalid config")
	}

	// Runner should not be called
	if len(runner.upCalls) != 0 {
		t.Error("runner should not be called on config error")
	}
}

func TestFleet_Down(t *testing.T) {
	dir := t.TempDir()
	setupTestFleet(t, dir)

	// Create the compose file (simulating a previous Up)
	outputDir := filepath.Join(dir, ".agent-fleet")
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "docker-compose.yml"), []byte("version: '3'"), 0644)

	runner := &mockRunner{}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: outputDir,
		Runner:    runner,
	})

	err := f.Down(context.Background())
	if err != nil {
		t.Fatalf("Down() error: %v", err)
	}

	if len(runner.downCalls) != 1 {
		t.Fatalf("Down() called %d times, want 1", len(runner.downCalls))
	}
	if runner.downCalls[0].projectName != "test-fleet" {
		t.Errorf("projectName = %q, want %q", runner.downCalls[0].projectName, "test-fleet")
	}
}

func TestFleet_Down_NotRunning(t *testing.T) {
	dir := t.TempDir()
	setupTestFleet(t, dir)

	runner := &mockRunner{}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: filepath.Join(dir, ".agent-fleet"),
		Runner:    runner,
	})

	err := f.Down(context.Background())
	if err == nil {
		t.Error("expected error when fleet not running")
	}
}

func TestFleet_Status(t *testing.T) {
	dir := t.TempDir()
	setupTestFleet(t, dir)

	// Create compose file
	outputDir := filepath.Join(dir, ".agent-fleet")
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "docker-compose.yml"), []byte("version: '3'"), 0644)

	runner := &mockRunner{psOutput: "NAME    STATUS\ncoder   running"}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: outputDir,
		Runner:    runner,
	})

	status, err := f.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status != "NAME    STATUS\ncoder   running" {
		t.Errorf("status = %q", status)
	}
}

func TestFleet_Status_NotRunning(t *testing.T) {
	dir := t.TempDir()
	setupTestFleet(t, dir)

	runner := &mockRunner{}
	f := New(Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: filepath.Join(dir, ".agent-fleet"),
		Runner:    runner,
	})

	status, err := f.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status != "Fleet is not running (no compose file found)" {
		t.Errorf("status = %q", status)
	}
}

func TestFleet_ComposeFilePath(t *testing.T) {
	f := New(Options{
		FleetFile: "/path/to/fleet.yaml",
		OutputDir: "/tmp/output",
	})

	got := f.ComposeFilePath()
	want := "/tmp/output/docker-compose.yml"
	if got != want {
		t.Errorf("ComposeFilePath() = %q, want %q", got, want)
	}
}

// setupTestFleet creates a minimal valid fleet config for testing.
func setupTestFleet(t *testing.T, dir string) {
	t.Helper()

	// Create go.mod so findRepoRoot works
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644)

	// Create images/ directory
	os.MkdirAll(filepath.Join(dir, "images", "gateway"), 0755)
	os.MkdirAll(filepath.Join(dir, "images", "sandbox"), 0755)

	fleetContent := `
fleet:
  name: test-fleet
agents:
  - coder
egress-presets:
  main:
    - host: ["*"]
`
	os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetContent), 0644)

	agentDir := filepath.Join(dir, "agents", "coder")
	os.MkdirAll(agentDir, 0755)

	agentContent := `
egress: [main]
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
`
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(agentContent), 0644)
}
