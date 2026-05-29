package gateway

// EgressRule defines a single egress rule.
type EgressRule struct {
	Host     []string       `yaml:"host,omitempty" json:"host,omitempty"`
	Endpoint []string       `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Provider string         `yaml:"provider,omitempty" json:"provider,omitempty"`
	Options  map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
}

// EgressPreset is an ordered list of egress rules.
type EgressPreset []EgressRule
