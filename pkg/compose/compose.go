// Package compose generates Docker Compose configurations from a resolved fleet.
package compose

import (
	"fmt"
	"path/filepath"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/donbader/agent-fleet/pkg/provider"
	"gopkg.in/yaml.v3"
)

// ComposeFile represents a docker-compose.yml structure.
type ComposeFile struct {
	Services map[string]*Service `yaml:"services"`
	Networks map[string]*Network `yaml:"networks"`
}

// Service represents a Docker Compose service.
type Service struct {
	Image       string            `yaml:"image,omitempty"`
	Build       *BuildConfig      `yaml:"build,omitempty"`
	Command     []string          `yaml:"command,omitempty,flow"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	CapAdd      []string          `yaml:"cap_add,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}

// BuildConfig represents the build section of a service.
type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
}

// Network represents a Docker Compose network.
type Network struct {
	Internal bool `yaml:"internal,omitempty"`
}

// Generator creates Docker Compose files from fleet configuration.
type Generator struct {
	fleet    *config.ResolvedFleet
	repoRoot string // absolute path to repo root (where images/ lives)
	resolver *provider.Resolver // resolves remote provider paths to local dirs
}

// New creates a new Compose generator for the given resolved fleet.
// repoRoot is the absolute path to the repository root (where images/ directory lives).
// resolver is used to resolve remote provider paths to local directories.
func New(fleet *config.ResolvedFleet, repoRoot string, resolver *provider.Resolver) *Generator {
	return &Generator{fleet: fleet, repoRoot: repoRoot, resolver: resolver}
}

// Generate produces the docker-compose.yml content as YAML bytes.
func (g *Generator) Generate() ([]byte, error) {
	compose := &ComposeFile{
		Services: make(map[string]*Service),
		Networks: map[string]*Network{
			g.internalNetworkName(): {Internal: true},
			g.externalNetworkName(): {Internal: false},
		},
	}

	// Generate gateway proxy service
	compose.Services[g.gatewayServiceName()] = g.gatewayService()

	// Generate one service per agent
	for _, name := range g.fleet.Fleet.Agents {
		agent := g.fleet.Agents[name]
		compose.Services[name] = g.agentService(name, agent)
	}

	return yaml.Marshal(compose)
}

// gatewayService creates the gateway proxy service definition.
func (g *Generator) gatewayService() *Service {
	return &Service{
		Build: &BuildConfig{
			Context:    filepath.Join(g.repoRoot, "images", "gateway"),
			Dockerfile: "Dockerfile",
		},
		Networks: []string{g.internalNetworkName(), g.externalNetworkName()},
		Volumes:  []string{"./gateway-rules.yaml:/etc/gateway/rules.yaml:ro"},
		Environment: map[string]string{
			"FLEET_NAME": g.fleet.Fleet.Fleet.Name,
		},
		Restart: "unless-stopped",
	}
}

// GatewayRulesYAML generates the gateway rules config file content.
// This file is mounted into the gateway container at /etc/gateway/rules.yaml.
func (g *Generator) GatewayRulesYAML() ([]byte, error) {
	// Compile all rules from all presets in order
	var allRules []config.EgressRule
	for _, name := range g.fleet.Fleet.Agents {
		agent := g.fleet.Agents[name]
		for _, presetName := range agent.Egress {
			preset, ok := g.fleet.Fleet.EgressPresets[presetName]
			if ok {
				allRules = append(allRules, preset...)
			}
		}
	}

	// Deduplicate rules (same host pattern + provider)
	seen := make(map[string]bool)
	var unique []config.EgressRule
	for _, r := range allRules {
		key := fmt.Sprintf("%v|%s", r.Host, r.Provider)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}

	type rulesFile struct {
		Rules []config.EgressRule `yaml:"rules"`
	}
	return yaml.Marshal(rulesFile{Rules: unique})
}

// agentService creates an agent container service definition.
func (g *Generator) agentService(name string, agent *config.AgentConfig) *Service {
	// Resolve the runtime provider to a local build context
	buildCtx := filepath.Join(g.repoRoot, "images", "sandbox")
	if g.resolver != nil {
		if resolved, err := g.resolver.Resolve(agent.Runtime.Provider); err == nil {
			buildCtx = resolved
		}
	}

	svc := &Service{
		Build: &BuildConfig{
			Context:    buildCtx,
			Dockerfile: "Dockerfile",
		},
		Networks:  []string{g.internalNetworkName()},
		DependsOn: []string{g.gatewayServiceName()},
		CapAdd:    []string{"NET_ADMIN"}, // Required for iptables
		Environment: map[string]string{
			"AGENT_NAME":    name,
			"GATEWAY_HOST":  g.gatewayServiceName(),
			"GATEWAY_PORT":  "8080",
		},
		Restart: "unless-stopped",
	}

	// Add user-defined env vars
	for k, v := range agent.Env {
		svc.Environment[k] = v
	}

	// Add custom build context if user_base_image_stage is set
	if stage := g.userBaseImageStage(agent); stage != "" {
		svc.Build = &BuildConfig{
			Context:    filepath.Join(g.repoRoot, "agents", name),
			Dockerfile: "Dockerfile",
		}
	}

	return svc
}

// userBaseImageStage checks if the agent has a custom Dockerfile stage.
func (g *Generator) userBaseImageStage(agent *config.AgentConfig) string {
	if agent.Runtime.Options == nil {
		return ""
	}
	if stage, ok := agent.Runtime.Options["user_base_image_stage"]; ok {
		if s, ok := stage.(string); ok {
			return s
		}
	}
	return ""
}

func (g *Generator) gatewayServiceName() string {
	return fmt.Sprintf("%s-gateway", g.fleet.Fleet.Fleet.Name)
}

func (g *Generator) internalNetworkName() string {
	return fmt.Sprintf("%s-internal", g.fleet.Fleet.Fleet.Name)
}

func (g *Generator) externalNetworkName() string {
	return fmt.Sprintf("%s-external", g.fleet.Fleet.Fleet.Name)
}
