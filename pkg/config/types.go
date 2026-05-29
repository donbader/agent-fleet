// Package config handles parsing and validation of fleet.yaml and agent.yaml files.
package config

// FleetConfig represents the top-level fleet.yaml configuration.
type FleetConfig struct {
	Fleet        FleetMeta              `yaml:"fleet"`
	Agents       []string               `yaml:"agents"`
	EgressPresets map[string]EgressPreset `yaml:"egress-presets"`
}

// FleetMeta contains fleet-level metadata.
type FleetMeta struct {
	Name string `yaml:"name"`
}

// EgressPreset is an ordered list of egress rules.
type EgressPreset []EgressRule

// EgressRule defines a single egress rule within a preset.
type EgressRule struct {
	Host     []string       `yaml:"host,omitempty"`
	Endpoint []string       `yaml:"endpoint,omitempty"`
	Provider string         `yaml:"provider,omitempty"`
	Options  map[string]any `yaml:"options,omitempty"`
}

// AgentConfig represents a per-agent agent.yaml configuration.
type AgentConfig struct {
	Egress  []string       `yaml:"egress"`
	Runtime ProviderRef    `yaml:"runtime"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// ProviderRef is a reference to a provider with optional options.
type ProviderRef struct {
	Provider string         `yaml:"provider"`
	Options  map[string]any `yaml:"options,omitempty"`
}

// ResolvedFleet is the fully resolved fleet configuration (fleet.yaml + all agent.yamls).
type ResolvedFleet struct {
	Fleet  FleetConfig
	Agents map[string]*AgentConfig
}
