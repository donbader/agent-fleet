package cmd

import (
	"fmt"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/spf13/cobra"
)

var (
	fleetFile string
	version   = "dev"
	commit    = "none"    //nolint:unused // set via ldflags at build time
	date      = "unknown" //nolint:unused // set via ldflags at build time
)

var rootCmd = &cobra.Command{
	Use:   "agent-fleet",
	Short: "Opinionated agent sandbox orchestrator",
	Long: `agent-fleet deploys AI coding agents inside Docker containers with:
  - Transparent egress proxy (iptables-enforced)
  - Composable egress-presets with credential injection
  - Per-agent messaging channels (Telegram, etc.)
  - Optional Docker API Proxy for controlled container spawning`,
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&fleetFile, "fleet-file", "f", "fleet.yaml", "path to fleet.yaml")
}

func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string (called from main or build flags).
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

// validateCmd validates configuration.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate fleet.yaml and agent configs without starting anything",
	RunE:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	resolved, err := resolveFleet()
	if err != nil {
		return err
	}
	fmt.Printf("✓ Fleet %q is valid (%d agents)\n", resolved.Fleet.Fleet.Name, len(resolved.Agents))
	for name, agent := range resolved.Agents {
		fmt.Printf("  - %s (runtime: %s, egress: %v)\n", name, agent.Runtime.Provider, agent.Egress)
	}
	return nil
}

func resolveFleet() (*config.ResolvedFleet, error) {
	return config.Resolve(fleetFile)
}
