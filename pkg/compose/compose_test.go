package compose

import (
	"testing"

	"github.com/donbader/agent-fleet/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestGenerate_SimpleFleet(t *testing.T) {
	fleet := &config.ResolvedFleet{
		Fleet: config.FleetConfig{
			Fleet:  config.FleetMeta{Name: "myfleet"},
			Agents: []string{"coder"},
			EgressPresets: map[string]config.EgressPreset{
				"main": {{Host: []string{"*"}}},
			},
		},
		Agents: map[string]*config.AgentConfig{
			"coder": {
				Egress: []string{"main"},
				Runtime: config.ProviderRef{
					Provider: "github.com/donbader/agent-fleet/runtimes/codex",
				},
			},
		},
	}

	gen := New(fleet, "/repo", nil)
	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Parse back to verify structure
	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("unmarshal generated compose: %v", err)
	}

	// Should have 2 services: gateway + coder
	if len(compose.Services) != 2 {
		t.Errorf("services count = %d, want 2", len(compose.Services))
	}

	// Gateway service — should use build
	gw := compose.Services["myfleet-gateway"]
	if gw == nil {
		t.Fatal("gateway service not found")
	}
	if gw.Build == nil {
		t.Fatal("gateway should have build config")
	}
	if gw.Build.Context != "/repo/images/gateway" {
		t.Errorf("gateway build context = %q, want /repo/images/gateway", gw.Build.Context)
	}

	// Agent service — should use build
	agent := compose.Services["coder"]
	if agent == nil {
		t.Fatal("coder service not found")
	}
	if agent.Build == nil {
		t.Fatal("coder should have build config")
	}
	if agent.Build.Context != "/repo/images/sandbox" {
		t.Errorf("coder build context = %q, want /repo/images/sandbox", agent.Build.Context)
	}
	if len(agent.CapAdd) == 0 || agent.CapAdd[0] != "NET_ADMIN" {
		t.Errorf("coder cap_add = %v, want [NET_ADMIN]", agent.CapAdd)
	}

	// Networks
	if len(compose.Networks) != 2 {
		t.Errorf("networks count = %d, want 2", len(compose.Networks))
	}
	internal := compose.Networks["myfleet-internal"]
	if internal == nil || !internal.Internal {
		t.Error("internal network missing or not internal")
	}
	external := compose.Networks["myfleet-external"]
	if external == nil || external.Internal {
		t.Error("external network missing or is internal")
	}
}

func TestGenerate_MultiAgent(t *testing.T) {
	fleet := &config.ResolvedFleet{
		Fleet: config.FleetConfig{
			Fleet:  config.FleetMeta{Name: "team"},
			Agents: []string{"coder", "reviewer"},
			EgressPresets: map[string]config.EgressPreset{
				"main": {{Host: []string{"*"}}},
			},
		},
		Agents: map[string]*config.AgentConfig{
			"coder": {
				Egress:  []string{"main"},
				Runtime: config.ProviderRef{Provider: "github.com/donbader/agent-fleet/runtimes/codex"},
				Env:     map[string]string{"EDITOR": "vim"},
			},
			"reviewer": {
				Egress:  []string{"main"},
				Runtime: config.ProviderRef{Provider: "github.com/donbader/agent-fleet/runtimes/claude-code"},
			},
		},
	}

	gen := New(fleet, "/repo", nil)
	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 3 services: gateway + 2 agents
	if len(compose.Services) != 3 {
		t.Errorf("services count = %d, want 3", len(compose.Services))
	}

	// Coder has custom env
	coder := compose.Services["coder"]
	if coder == nil {
		t.Fatal("coder service not found")
	}
	if coder.Environment["EDITOR"] != "vim" {
		t.Errorf("coder EDITOR = %q, want vim", coder.Environment["EDITOR"])
	}

	// Reviewer uses sandbox build
	reviewer := compose.Services["reviewer"]
	if reviewer == nil {
		t.Fatal("reviewer service not found")
	}
	if reviewer.Build == nil {
		t.Fatal("reviewer should have build config")
	}
	if reviewer.Build.Context != "/repo/images/sandbox" {
		t.Errorf("reviewer build context = %q", reviewer.Build.Context)
	}
}

func TestGenerate_AgentDependsOnGateway(t *testing.T) {
	fleet := &config.ResolvedFleet{
		Fleet: config.FleetConfig{
			Fleet:  config.FleetMeta{Name: "test"},
			Agents: []string{"a"},
			EgressPresets: map[string]config.EgressPreset{
				"main": {{Host: []string{"*"}}},
			},
		},
		Agents: map[string]*config.AgentConfig{
			"a": {
				Egress:  []string{"main"},
				Runtime: config.ProviderRef{Provider: "github.com/donbader/agent-fleet/runtimes/codex"},
			},
		},
	}

	gen := New(fleet, "/repo", nil)
	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	agent := compose.Services["a"]
	if agent == nil {
		t.Fatal("agent 'a' not found")
	}
	if len(agent.DependsOn) != 1 || agent.DependsOn[0] != "test-gateway" {
		t.Errorf("depends_on = %v, want [test-gateway]", agent.DependsOn)
	}
}
