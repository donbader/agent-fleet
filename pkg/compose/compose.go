// Package compose generates Docker Compose configurations from a resolved fleet.
package compose

import (
	"fmt"
	"strings"

	"github.com/donbader/agent-fleet/pkg/config"
	"gopkg.in/yaml.v3"
)

// ComposeFile represents a docker-compose.yml structure.
type ComposeFile struct {
	Version  string                    `yaml:"version,omitempty"`
	Services map[string]*Service       `yaml:"services"`
	Networks map[string]*Network       `yaml:"networks"`
	Volumes  map[string]*Volume        `yaml:"volumes,omitempty"`
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

// Volume represents a Docker Compose volume.
type Volume struct{}

// Generator creates Docker Compose files from fleet configuration.
type Generator struct {
	fleet *config.ResolvedFleet
}

// New creates a new Compose generator for the given resolved fleet.
func New(fleet *config.ResolvedFleet) *Generator {
	return &Generator{fleet: fleet}
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
		Image:   "ghcr.io/donbader/agent-fleet/gateway:latest",
		Networks: []string{g.internalNetworkName(), g.externalNetworkName()},
		Environment: map[string]string{
			"FLEET_NAME": g.fleet.Fleet.Fleet.Name,
		},
		Restart: "unless-stopped",
	}
}

// agentService creates an agent container service definition.
func (g *Generator) agentService(name string, agent *config.AgentConfig) *Service {
	svc := &Service{
		Image:     g.agentImage(agent),
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

	// Add build context if user_base_image_stage is set
	if stage := g.userBaseImageStage(agent); stage != "" {
		svc.Build = &BuildConfig{
			Context:    fmt.Sprintf("./agents/%s", name),
			Dockerfile: "Dockerfile",
		}
		svc.Image = "" // Use build instead
	}

	return svc
}

// agentImage determines the Docker image for an agent based on its runtime provider.
func (g *Generator) agentImage(agent *config.AgentConfig) string {
	// Extract the runtime name from the provider path
	// e.g., "github.com/donbader/agent-fleet/runtimes/codex" -> "codex"
	provider := agent.Runtime.Provider
	parts := strings.Split(provider, "/")
	runtimeName := parts[len(parts)-1]
	return fmt.Sprintf("ghcr.io/donbader/agent-fleet/%s:latest", runtimeName)
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
