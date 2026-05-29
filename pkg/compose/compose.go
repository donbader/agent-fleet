// Package compose generates Docker Compose configurations from a resolved fleet.
package compose

import (
	"fmt"

	"github.com/donbader/agent-fleet/pkg/config"
)

// Generator creates Docker Compose files from fleet configuration.
type Generator struct {
	fleet *config.ResolvedFleet
}

// New creates a new Compose generator for the given resolved fleet.
func New(fleet *config.ResolvedFleet) *Generator {
	return &Generator{fleet: fleet}
}

// Generate produces the docker-compose.yml content.
func (g *Generator) Generate() ([]byte, error) {
	// TODO: implement Docker Compose generation
	// This will create:
	// - One container per agent (with runtime provider image)
	// - One gateway proxy container (shared or per-agent)
	// - Network configuration (internal + external)
	// - Volume mounts for agent home directories
	return nil, fmt.Errorf("compose generation not implemented yet")
}
