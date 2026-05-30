// Package compose generates Docker Compose configurations from a resolved fleet.
package compose

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/donbader/agent-fleet/pkg/provider"
	"gopkg.in/yaml.v3"
)

// ComposeFile represents a docker-compose.yml structure.
type ComposeFile struct {
	Services map[string]*Service      `yaml:"services"`
	Networks map[string]*Network      `yaml:"networks"`
	Volumes  map[string]*VolumeConfig `yaml:"volumes,omitempty"`
}

// Service represents a Docker Compose service.
type Service struct {
	Image       string            `yaml:"image,omitempty"`
	Build       *BuildConfig      `yaml:"build,omitempty"`
	Command     []string          `yaml:"command,omitempty,flow"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	CapAdd      []string          `yaml:"cap_add,omitempty"`
	Sysctls     []string          `yaml:"sysctls,omitempty"`
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

// VolumeConfig represents a top-level volume declaration.
type VolumeConfig struct{}

// RenderContext is the JSON input passed to a provider's render script via stdin.
type RenderContext struct {
	Name        string            `json:"name"`
	FleetName   string            `json:"fleet_name"`
	AgentDir    string            `json:"agent_dir"`
	Options     map[string]any    `json:"options"`
	Env         map[string]string `json:"env"`
	GatewayHost string            `json:"gateway_host"`
	GatewayPort string            `json:"gateway_port"`
}

// Generator creates Docker Compose files from fleet configuration.
type Generator struct {
	fleet    *config.ResolvedFleet
	repoRoot string             // absolute path to repo root (where images/ lives)
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

	// Collect named volumes from all services and declare them top-level
	volumes := collectNamedVolumes(compose.Services)
	if len(volumes) > 0 {
		compose.Volumes = volumes
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
		slog.Warn("render script failed, using defaults", "agent", name, "error", err)
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
	}

	// Merge user-defined env vars (applies to both render script and fallback)
	if len(agent.Env) > 0 {
		if svc.Environment == nil {
			svc.Environment = make(map[string]string)
		}
		for k, v := range agent.Env {
			svc.Environment[k] = v
		}
	}

	// Fleet-level concerns (always applied by CLI, not provider)
	// Merge user-declared volumes from agent.yaml
	if len(agent.Volumes) > 0 {
		svc.Volumes = appendUnique(svc.Volumes, agent.Volumes)
	}

	// Merge user-declared ports from agent.yaml with provider-declared ports.
	if len(agent.Ports) > 0 {
		svc.Ports = appendUnique(svc.Ports, agent.Ports)
	}

	// Agents always need the internal network to reach the gateway.
	// If the service exposes ports to the host, it also needs the external
	// network — Docker won't publish ports from internal-only networks.
	// The iptables rules inside the container are the real security boundary.
	svc.Networks = []string{g.internalNetworkName()}
	if len(svc.Ports) > 0 {
		svc.Networks = append(svc.Networks, g.externalNetworkName())
	}
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
		AgentDir:    filepath.Join(g.repoRoot, "agents", name),
		Options:     agent.Runtime.Options,
		Env:         agent.Env,
		GatewayHost: g.gatewayServiceName(),
		GatewayPort: "8080",
	}

	contextJSON, err := json.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("marshaling render context: %w", err)
	}

	// Execute render script with context available via env var and stdin
	cmd = exec.Command("bash", renderScript)
	cmd.Dir = providerDir
	cmd.Stdin = bytes.NewReader(contextJSON)
	cmd.Env = append(os.Environ(), "RENDER_CONTEXT="+string(contextJSON))

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

	// Resolve relative build context paths against the provider directory.
	// The render script runs with cwd=providerDir, so relative paths like "."
	// refer to providerDir — but Docker Compose resolves them relative to the
	// compose file location. Convert to absolute to avoid mismatch.
	if svc.Build != nil && svc.Build.Context != "" && !filepath.IsAbs(svc.Build.Context) {
		svc.Build.Context = filepath.Join(providerDir, svc.Build.Context)
	}

	return &svc, nil
}

func (g *Generator) gatewayServiceName() string {
	return "gateway"
}

func (g *Generator) internalNetworkName() string {
	return "internal"
}

func (g *Generator) externalNetworkName() string {
	return "external"
}

// collectNamedVolumes scans all services for named volume references
// and returns them as top-level volume declarations.
// Named volumes are those that don't start with "." or "/" (bind mounts).
func collectNamedVolumes(services map[string]*Service) map[string]*VolumeConfig {
	volumes := make(map[string]*VolumeConfig)
	for _, svc := range services {
		for _, v := range svc.Volumes {
			// Split on ":" to get the source part
			parts := strings.SplitN(v, ":", 2)
			source := parts[0]
			// Named volumes don't start with "." or "/"
			if len(source) > 0 && source[0] != '.' && source[0] != '/' {
				volumes[source] = &VolumeConfig{}
			}
		}
	}
	if len(volumes) == 0 {
		return nil
	}
	return volumes
}

// appendUnique appends items from src to dst, skipping duplicates.
func appendUnique(dst, src []string) []string {
	seen := make(map[string]bool, len(dst))
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range src {
		if !seen[v] {
			dst = append(dst, v)
			seen[v] = true
		}
	}
	return dst
}
