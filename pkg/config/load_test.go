package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFleet(t *testing.T) {
	dir := t.TempDir()
	fleetPath := filepath.Join(dir, "fleet.yaml")

	content := `
fleet:
  name: test-fleet

agents:
  - coder

egress-presets:
  main:
    - host: ["api.github.com"]
      provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
      options:
        token: "${GITHUB_PAT_TOKEN}"
    - host: ["*"]
`
	if err := os.WriteFile(fleetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFleet(fleetPath)
	if err != nil {
		t.Fatalf("LoadFleet() error: %v", err)
	}

	if cfg.Fleet.Name != "test-fleet" {
		t.Errorf("Fleet.Name = %q, want %q", cfg.Fleet.Name, "test-fleet")
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0] != "coder" {
		t.Errorf("Agents = %v, want [coder]", cfg.Agents)
	}
	if len(cfg.EgressPresets) != 1 {
		t.Errorf("EgressPresets count = %d, want 1", len(cfg.EgressPresets))
	}
	preset := cfg.EgressPresets["main"]
	if len(preset) != 2 {
		t.Fatalf("main preset rules = %d, want 2", len(preset))
	}
	if preset[0].Provider != "github.com/donbader/agent-fleet/egress-rules/github-pat" {
		t.Errorf("rule[0].Provider = %q", preset[0].Provider)
	}
	if preset[1].Host[0] != "*" {
		t.Errorf("rule[1].Host = %v, want [*]", preset[1].Host)
	}
}

func TestLoadFleet_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing fleet name",
			content: "fleet:\n  name: \"\"\nagents: [a]\negress-presets:\n  main:\n    - host: [\"*\"]\n",
			wantErr: "fleet.name is required",
		},
		{
			name:    "no agents",
			content: "fleet:\n  name: test\nagents: []\negress-presets:\n  main:\n    - host: [\"*\"]\n",
			wantErr: "at least one agent is required",
		},
		{
			name:    "no egress presets",
			content: "fleet:\n  name: test\nagents: [a]\negress-presets: {}\n",
			wantErr: "at least one egress preset is required",
		},
		{
			name:    "duplicate agent",
			content: "fleet:\n  name: test\nagents: [a, a]\negress-presets:\n  main:\n    - host: [\"*\"]\n",
			wantErr: "duplicate agent name",
		},
		{
			name:    "empty preset",
			content: "fleet:\n  name: test\nagents: [a]\negress-presets:\n  main: []\n",
			wantErr: "has no rules",
		},
		{
			name:    "rule with no matcher",
			content: "fleet:\n  name: test\nagents: [a]\negress-presets:\n  main:\n    - options: {}\n",
			wantErr: "must have at least one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "fleet.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadFleet(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadAgent(t *testing.T) {
	dir := t.TempDir()
	agentPath := filepath.Join(dir, "agent.yaml")

	content := `
egress: [telegram-bot-1, main]

runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/channels-bridge"
  options:
    agent_provider: "github.com/donbader/agent-fleet/runtimes/codex"
    channels:
      - provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
        options:
          allowed_users: ["@myuser"]

env:
  EDITOR: vim
`
	if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAgent(agentPath)
	if err != nil {
		t.Fatalf("LoadAgent() error: %v", err)
	}

	if len(cfg.Egress) != 2 {
		t.Errorf("Egress = %v, want 2 items", cfg.Egress)
	}
	if cfg.Runtime.Provider != "github.com/donbader/agent-fleet/runtimes/channels-bridge" {
		t.Errorf("Runtime.Provider = %q", cfg.Runtime.Provider)
	}
	if cfg.Env["EDITOR"] != "vim" {
		t.Errorf("Env[EDITOR] = %q, want vim", cfg.Env["EDITOR"])
	}
}

func TestLoadAgent_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing egress",
			content: "egress: []\nruntime:\n  provider: foo\n",
			wantErr: "egress is required",
		},
		{
			name:    "missing runtime provider",
			content: "egress: [main]\nruntime:\n  provider: \"\"\n",
			wantErr: "runtime.provider is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "agent.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadAgent(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()

	// Create fleet.yaml
	fleetContent := `
fleet:
  name: test-fleet
agents:
  - coder
egress-presets:
  main:
    - host: ["*"]
`
	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create agents/coder/agent.yaml
	agentDir := filepath.Join(dir, "agents", "coder")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentContent := `
egress: [main]
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
`
	if err := os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := Resolve(filepath.Join(dir, "fleet.yaml"))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	if resolved.Fleet.Fleet.Name != "test-fleet" {
		t.Errorf("Fleet.Name = %q", resolved.Fleet.Fleet.Name)
	}
	if len(resolved.Agents) != 1 {
		t.Fatalf("Agents count = %d, want 1", len(resolved.Agents))
	}
	agent := resolved.Agents["coder"]
	if agent == nil {
		t.Fatal("agent 'coder' not found")
	}
	if agent.Runtime.Provider != "github.com/donbader/agent-fleet/runtimes/codex" {
		t.Errorf("agent.Runtime.Provider = %q", agent.Runtime.Provider)
	}
}

func TestResolve_UndefinedPreset(t *testing.T) {
	dir := t.TempDir()

	fleetContent := `
fleet:
  name: test
agents:
  - coder
egress-presets:
  main:
    - host: ["*"]
`
	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetContent), 0644); err != nil {
		t.Fatal(err)
	}

	agentDir := filepath.Join(dir, "agents", "coder")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentContent := `
egress: [nonexistent]
runtime:
  provider: foo
`
	if err := os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Resolve(filepath.Join(dir, "fleet.yaml"))
	if err == nil {
		t.Fatal("expected error for undefined preset")
	}
	if !contains(err.Error(), "undefined egress preset") {
		t.Errorf("error = %q, want to contain 'undefined egress preset'", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
