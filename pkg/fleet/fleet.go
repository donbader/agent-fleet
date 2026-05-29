// Package fleet orchestrates the lifecycle of an agent fleet.
// It coordinates config resolution, compose generation, and Docker Compose execution.
package fleet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/donbader/agent-fleet/pkg/compose"
	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/donbader/agent-fleet/pkg/provider"
)

// Runner executes Docker Compose commands. Interface for testability.
type Runner interface {
	Up(ctx context.Context, composeFile string, projectName string) error
	Down(ctx context.Context, composeFile string, projectName string) error
	Ps(ctx context.Context, composeFile string, projectName string) (string, error)
}

// Fleet manages the lifecycle of an agent fleet.
type Fleet struct {
	fleetFile string
	outputDir string
	runner    Runner
}

// Options configures fleet behavior.
type Options struct {
	// FleetFile is the path to fleet.yaml.
	FleetFile string

	// OutputDir is where generated files are written (default: .agent-fleet/).
	OutputDir string

	// Runner is the Docker Compose runner (default: DockerComposeRunner).
	Runner Runner
}

// New creates a new Fleet instance.
func New(opts Options) *Fleet {
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(filepath.Dir(opts.FleetFile), ".agent-fleet")
	}

	runner := opts.Runner
	if runner == nil {
		runner = &DockerComposeRunner{}
	}

	return &Fleet{
		fleetFile: opts.FleetFile,
		outputDir: outputDir,
		runner:    runner,
	}
}

// Up resolves config, generates compose, and starts the fleet.
func (f *Fleet) Up(ctx context.Context) error {
	// 1. Resolve configuration
	resolved, err := config.Resolve(f.fleetFile)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	// 2. Find repo root (where images/ lives)
	repoRoot, err := findRepoRoot(f.fleetFile)
	if err != nil {
		return fmt.Errorf("finding repo root: %w", err)
	}

	// 3. Set up provider resolver (clones remote providers to cache)
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "agent-fleet", "providers")
	resolver := provider.NewResolver(cacheDir)

	// 4. Generate docker-compose.yml
	gen := compose.New(resolved, repoRoot, resolver)
	data, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("generating compose: %w", err)
	}

	// 4. Generate gateway rules config
	rulesData, err := gen.GatewayRulesYAML()
	if err != nil {
		return fmt.Errorf("generating gateway rules: %w", err)
	}

	// 5. Write to output directory
	if err := os.MkdirAll(f.outputDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	composeFile := filepath.Join(f.outputDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, data, 0644); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}

	rulesFile := filepath.Join(f.outputDir, "gateway-rules.yaml")
	if err := os.WriteFile(rulesFile, rulesData, 0644); err != nil {
		return fmt.Errorf("writing gateway rules: %w", err)
	}

	// 6. Start containers
	projectName := resolved.Fleet.Fleet.Name
	if err := f.runner.Up(ctx, composeFile, projectName); err != nil {
		return fmt.Errorf("starting fleet: %w", err)
	}

	return nil
}

// Down stops and removes fleet containers.
func (f *Fleet) Down(ctx context.Context) error {
	// Resolve to get project name
	resolved, err := config.Resolve(f.fleetFile)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	composeFile := filepath.Join(f.outputDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("no compose file found at %s (fleet not running?)", composeFile)
	}

	projectName := resolved.Fleet.Fleet.Name
	return f.runner.Down(ctx, composeFile, projectName)
}

// Status shows the current fleet status.
func (f *Fleet) Status(ctx context.Context) (string, error) {
	resolved, err := config.Resolve(f.fleetFile)
	if err != nil {
		return "", fmt.Errorf("resolving config: %w", err)
	}

	composeFile := filepath.Join(f.outputDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return "Fleet is not running (no compose file found)", nil
	}

	projectName := resolved.Fleet.Fleet.Name
	return f.runner.Ps(ctx, composeFile, projectName)
}

// ComposeFilePath returns the path to the generated docker-compose.yml.
func (f *Fleet) ComposeFilePath() string {
	return filepath.Join(f.outputDir, "docker-compose.yml")
}

// DockerComposeRunner executes real Docker Compose commands.
type DockerComposeRunner struct{}

func (r *DockerComposeRunner) Up(ctx context.Context, composeFile string, projectName string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFile,
		"-p", projectName,
		"up", "-d", "--build")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *DockerComposeRunner) Down(ctx context.Context, composeFile string, projectName string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFile,
		"-p", projectName,
		"down", "--remove-orphans")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *DockerComposeRunner) Ps(ctx context.Context, composeFile string, projectName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFile,
		"-p", projectName,
		"ps", "--format", "table")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// findRepoRoot walks up from the given path looking for the repo root.
// It looks for go.mod or .git as indicators.
func findRepoRoot(fromPath string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(fromPath))
	if err != nil {
		return "", err
	}

	for {
		// Check for go.mod (primary indicator)
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		// Check for .git
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		// Check for images/ directory
		if _, err := os.Stat(filepath.Join(dir, "images")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root (no go.mod, .git, or images/ found above %s)", fromPath)
		}
		dir = parent
	}
}
