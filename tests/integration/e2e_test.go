//go:build integration

// Package integration contains E2E tests that require Docker.
// Run with: go test -tags integration ./tests/integration/
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/donbader/agent-fleet/pkg/fleet"
)

// TestE2E_FleetUpDown tests the full fleet lifecycle with Docker.
// Requires Docker to be running.
func TestE2E_FleetUpDown(t *testing.T) {
	if os.Getenv("AGENT_FLEET_E2E") == "" {
		t.Skip("skipping E2E test (set AGENT_FLEET_E2E=1 to run)")
	}

	dir := t.TempDir()

	// Create fleet config
	fleetContent := `
fleet:
  name: e2e-test

agents:
  - echo-agent

egress-presets:
  allow-all:
    - host: ["*"]
`
	os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetContent), 0644)

	// Create agent config
	agentDir := filepath.Join(dir, "agents", "echo-agent")
	os.MkdirAll(agentDir, 0755)

	agentContent := `
egress: [allow-all]
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
`
	os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(agentContent), 0644)

	// Create fleet
	f := fleet.New(fleet.Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: filepath.Join(dir, ".agent-fleet"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Up
	t.Log("Starting fleet...")
	if err := f.Up(ctx); err != nil {
		t.Fatalf("Fleet.Up() error: %v", err)
	}

	// Verify compose file was generated
	composeFile := f.ComposeFilePath()
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Fatal("compose file not generated")
	}

	// Status
	status, err := f.Status(ctx)
	if err != nil {
		t.Fatalf("Fleet.Status() error: %v", err)
	}
	t.Logf("Fleet status:\n%s", status)

	// Down
	t.Log("Stopping fleet...")
	if err := f.Down(ctx); err != nil {
		t.Fatalf("Fleet.Down() error: %v", err)
	}

	t.Log("E2E test passed!")
}

// TestE2E_FleetValidateAndGenerate tests config validation and compose generation
// without actually starting Docker containers.
func TestE2E_FleetValidateAndGenerate(t *testing.T) {
	dir := t.TempDir()

	// Create a realistic fleet config
	fleetContent := `
fleet:
  name: integration-test

agents:
  - coder
  - reviewer

egress-presets:
  telegram-bot:
    - host: ["api.telegram.org"]
      provider: "github.com/donbader/agent-fleet/egress-rules/telegram-bot"
      options:
        token: "${TELEGRAM_BOT_TOKEN}"
  main:
    - host: ["api.github.com", "github.com"]
      provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
      options:
        token: "${GITHUB_PAT_TOKEN}"
    - host: ["*.npmjs.org", "registry.yarnpkg.com"]
    - host: ["*"]
`
	os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetContent), 0644)

	// Create coder agent
	coderDir := filepath.Join(dir, "agents", "coder")
	os.MkdirAll(coderDir, 0755)
	coderContent := `
egress: [telegram-bot, main]
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/channels-bridge"
  options:
    agent_provider: "github.com/donbader/agent-fleet/runtimes/codex"
    channels:
      - provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
        options:
          allowed_users: ["@admin"]
env:
  EDITOR: vim
`
	os.WriteFile(filepath.Join(coderDir, "agent.yaml"), []byte(coderContent), 0644)

	// Create reviewer agent
	reviewerDir := filepath.Join(dir, "agents", "reviewer")
	os.MkdirAll(reviewerDir, 0755)
	reviewerContent := `
egress: [main]
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/claude-code"
`
	os.WriteFile(filepath.Join(reviewerDir, "agent.yaml"), []byte(reviewerContent), 0644)

	// Use a mock runner so we don't need Docker
	runner := &noopRunner{}
	f := fleet.New(fleet.Options{
		FleetFile: filepath.Join(dir, "fleet.yaml"),
		OutputDir: filepath.Join(dir, ".agent-fleet"),
		Runner:    runner,
	})

	ctx := context.Background()

	// Up should succeed (validates config + generates compose + calls runner)
	if err := f.Up(ctx); err != nil {
		t.Fatalf("Fleet.Up() error: %v", err)
	}

	// Verify compose file was generated
	composeFile := f.ComposeFilePath()
	data, err := os.ReadFile(composeFile)
	if err != nil {
		t.Fatalf("reading compose file: %v", err)
	}

	content := string(data)

	// Verify services exist
	if !containsStr(content, "coder") {
		t.Error("compose file missing 'coder' service")
	}
	if !containsStr(content, "reviewer") {
		t.Error("compose file missing 'reviewer' service")
	}
	if !containsStr(content, "integration-test-gateway") {
		t.Error("compose file missing gateway service")
	}

	// Verify networks
	if !containsStr(content, "integration-test-internal") {
		t.Error("compose file missing internal network")
	}
	if !containsStr(content, "integration-test-external") {
		t.Error("compose file missing external network")
	}

	// Verify runner was called
	if !runner.upCalled {
		t.Error("runner.Up() was not called")
	}

	t.Logf("Generated compose file (%d bytes):\n%s", len(data), content)
}

// noopRunner is a Runner that does nothing (for testing without Docker).
type noopRunner struct {
	upCalled   bool
	downCalled bool
}

func (r *noopRunner) Up(ctx context.Context, composeFile string, projectName string) error {
	r.upCalled = true
	return nil
}

func (r *noopRunner) Down(ctx context.Context, composeFile string, projectName string) error {
	r.downCalled = true
	return nil
}

func (r *noopRunner) Ps(ctx context.Context, composeFile string, projectName string) (string, error) {
	return "NAME    STATUS\ncoder   running\nreviewer running", nil
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
