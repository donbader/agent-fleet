package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadFleet loads fleet.yaml from the given path.
func LoadFleet(path string) (*FleetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fleet config: %w", err)
	}

	var cfg FleetConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing fleet config: %w", err)
	}

	if err := validateFleet(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadAgent loads an agent.yaml from the given path.
func LoadAgent(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent config: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing agent config: %w", err)
	}

	if err := validateAgent(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Resolve loads fleet.yaml and all referenced agent.yaml files, returning a fully resolved fleet.
func Resolve(fleetPath string) (*ResolvedFleet, error) {
	fleet, err := LoadFleet(fleetPath)
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Dir(fleetPath)
	agents := make(map[string]*AgentConfig, len(fleet.Agents))

	for _, name := range fleet.Agents {
		agentPath := filepath.Join(baseDir, "agents", name, "agent.yaml")
		agent, err := LoadAgent(agentPath)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", name, err)
		}

		// Validate egress references exist in fleet presets
		for _, preset := range agent.Egress {
			if _, ok := fleet.EgressPresets[preset]; !ok {
				return nil, fmt.Errorf("agent %q references undefined egress preset %q", name, preset)
			}
		}

		agents[name] = agent
	}

	return &ResolvedFleet{
		Fleet:  *fleet,
		Agents: agents,
	}, nil
}

func validateFleet(cfg *FleetConfig) error {
	if cfg.Fleet.Name == "" {
		return fmt.Errorf("fleet.name is required")
	}
	if len(cfg.Agents) == 0 {
		return fmt.Errorf("at least one agent is required")
	}
	if len(cfg.EgressPresets) == 0 {
		return fmt.Errorf("at least one egress preset is required")
	}

	// Check for duplicate agent names
	seen := make(map[string]bool)
	for _, name := range cfg.Agents {
		if seen[name] {
			return fmt.Errorf("duplicate agent name: %q", name)
		}
		seen[name] = true
	}

	// Validate each preset has at least one rule
	for name, preset := range cfg.EgressPresets {
		if len(preset) == 0 {
			return fmt.Errorf("egress preset %q has no rules", name)
		}
		for i, rule := range preset {
			if err := validateEgressRule(rule); err != nil {
				return fmt.Errorf("egress preset %q rule %d: %w", name, i, err)
			}
		}
	}

	return nil
}

func validateAgent(cfg *AgentConfig) error {
	if len(cfg.Egress) == 0 {
		return fmt.Errorf("egress is required (at least one preset reference)")
	}
	if cfg.Runtime.Provider == "" {
		return fmt.Errorf("runtime.provider is required")
	}
	return nil
}

func validateEgressRule(rule EgressRule) error {
	// A rule must have at least one matcher or a provider
	if len(rule.Host) == 0 && len(rule.Endpoint) == 0 && rule.Provider == "" {
		return fmt.Errorf("rule must have at least one of: host, endpoint, or provider")
	}
	return nil
}
