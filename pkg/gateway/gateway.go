// Package gateway implements the transparent egress proxy.
// It intercepts all outbound TCP traffic (via iptables redirect) and applies
// egress rules — credential injection, MITM, or passthrough.
package gateway

import (
	"context"
	"fmt"

	"github.com/donbader/agent-fleet/pkg/config"
)

// Gateway is the transparent egress proxy.
type Gateway struct {
	presets map[string]config.EgressPreset
	rules   []config.EgressRule // compiled, ordered rules for this agent
}

// Config holds gateway startup configuration.
type Config struct {
	// ListenAddr is the address the proxy listens on (e.g., ":8080").
	ListenAddr string

	// Presets are the named egress presets from fleet.yaml.
	Presets map[string]config.EgressPreset

	// ActivePresets is the ordered list of preset names for this agent.
	ActivePresets []string
}

// New creates a new Gateway instance.
func New(cfg Config) (*Gateway, error) {
	// Compile ordered rules from active presets
	var rules []config.EgressRule
	for _, name := range cfg.ActivePresets {
		preset, ok := cfg.Presets[name]
		if !ok {
			return nil, fmt.Errorf("undefined egress preset: %q", name)
		}
		rules = append(rules, preset...)
	}

	return &Gateway{
		presets: cfg.Presets,
		rules:   rules,
	}, nil
}

// Run starts the gateway proxy. Blocks until context is cancelled.
func (g *Gateway) Run(ctx context.Context) error {
	// TODO: implement transparent proxy
	// 1. Listen on configured address
	// 2. Accept connections (redirected by iptables)
	// 3. Extract original destination (SO_ORIGINAL_DST)
	// 4. Match against rules (first match wins)
	// 5. For matching rules with provider: MITM + credential injection
	// 6. For passthrough rules: direct TCP proxy
	// 7. Default deny: reject if no rule matches
	return fmt.Errorf("gateway proxy not implemented yet")
}

// CompiledRules returns the ordered list of egress rules for inspection/testing.
func (g *Gateway) CompiledRules() []config.EgressRule {
	return g.rules
}
