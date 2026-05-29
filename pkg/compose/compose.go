// Package compose generates Docker Compose configurations from a resolved fleet.
package compose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
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

// RenderContext is the JSON input passed to a provider's render script via stdin.
type RenderContext struct {
	Name        string         `json:"name"`
	FleetName   string         `json:"fleet_name"`
	Options     map[string]any `json:"options"`
	Env         map[string]string `json:"env"`
	GatewayHost string         `json:"gateway_host"`
	GatewayPort string         `json:"gateway_port"`
}

// Generator creates Docker Compose files from fleet configuration.
type Generator struct {
	fleet    *config.ResolvedFleet
	repoRoot string // absolute path to repo root (where images/ lives)
	resolver *provider.Resolver // resolves remote provider paths to local dirs
}

// New creates a new Compose generator for the given resolved fleet.
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
		svc, err := g.agentService(name, agent)
		if err != nil {
			return nil, fmt.Errorf("generating service for agent %q: %w", name, err)
		}
		compose.Services[name] = svc
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
func (g *Generator) GatewayRulesYAML() ([]byte, error) {
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

// agentService creates an agent container service definition by executing
// the provider's render script if available, or falling back to defaults.
func (g *Generator) agentService(name string, agent *config.AgentConfig) (*Service, error) {
	// Resolve the runtime provider to a local directory
	providerDir := filepath.Join(g.repoRoot, "images", "sandbox")
	if g.resolver != nil {
		if resolved, err := g.resolver.Resolve(agent.Runtime.Provider); err == nil {
			providerDir = resolved
		}
	}

	// Try to execute the provider's render script
	svc, err := g.executeRenderScript(providerDir, name, agent)
	if err != nil {
		log.Printf("[compose] render script failed for %s, using defaults: %v", name, err)
		svc = nil
	}

	if svc == nil {
		// Fallback: default service definition
		svc = &Service{
			Build: &BuildConfig{
				Context:    providerDir,
				Dockerfile: "Dockerfile",
			},
			Environment: map[string]string{
				"AGENT_NAME":   name,
				"GATEWAY_HOST": g.gatewayServiceName(),
				"GATEWAY_PORT": "8080",
			},
			CapAdd:  []string{"NET_ADMIN"},
			Restart: "unless-stopped",
		}

		// Add user-defined env vars
		for k, v := range agent.Env {
			svc.Environment[k] = v
		}
	}

	// Fleet-level concerns (always applied by CLI, not provider)
	svc.Networks = []string{g.internalNetworkName()}
	svc.DependsOn = []string{g.gatewayServiceName()}

	return svc, nil
}

// executeRenderScript runs the provider's render.sh and parses its YAML output.
func (g *Generator) executeRenderScript(providerDir string, name string, agent *config.AgentConfig) (*Service, error) {
	renderScript := filepath.Join(providerDir, "render.sh")

	// Check if render script exists
	cmd := exec.Command("test", "-f", renderScript)
	if cmd.Run() != nil {
		return nil, fmt.Errorf("no render.sh found at %s", renderScript)
	}

	// Build render context
	ctx := RenderContext{
		Name:        name,
		FleetName:   g.fleet.Fleet.Fleet.Name,
		Options:     agent.Runtime.Options,
		Env:         agent.Env,
		GatewayHost: g.gatewayServiceName(),
		GatewayPort: "8080",
	}

	contextJSON, err := json.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("marshaling render context: %w", err)
	}

	// Execute render script
	cmd = exec.Command("bash", renderScript)
	cmd.Dir = providerDir
	cmd.Stdin = bytes.NewReader(contextJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("render.sh failed: %w\nstderr: %s", err, stderr.String())
	}

	// Parse YAML output into Service
	var svc Service
	if err := yaml.Unmarshal(stdout.Bytes(), &svc); err != nil {
		return nil, fmt.Errorf("parsing render.sh output: %w", err)
	}

	return &svc, nil
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
