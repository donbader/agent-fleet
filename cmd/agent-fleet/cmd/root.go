package cmd

import (
	"fmt"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/spf13/cobra"
)

var (
	fleetFile string
	version   = "dev"
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
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(validateCmd)
}

// upCmd starts the fleet.
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the fleet (build images, create containers, start agents)",
	RunE:  runUp,
}

// downCmd stops the fleet.
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove fleet containers",
	RunE:  runDown,
}

// statusCmd shows fleet status.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show running agents, channels, and health",
	RunE:  runStatus,
}

// validateCmd validates configuration.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate fleet.yaml and agent configs without starting anything",
	RunE:  runValidate,
}

func runUp(cmd *cobra.Command, args []string) error {
	fmt.Println("Loading fleet configuration...")
	// TODO: implement fleet startup
	return fmt.Errorf("not implemented yet")
}

func runDown(cmd *cobra.Command, args []string) error {
	fmt.Println("Stopping fleet...")
	// TODO: implement fleet shutdown
	return fmt.Errorf("not implemented yet")
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("Fleet status:")
	// TODO: implement status display
	return fmt.Errorf("not implemented yet")
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
