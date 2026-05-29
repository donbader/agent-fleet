package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new fleet directory with scaffolded config files",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const fleetSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/fleet.schema.json"
const agentSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/agent.schema.json"

func runInit(cmd *cobra.Command, args []string) error {
	name := "my-fleet"
	if len(args) > 0 {
		name = args[0]
	}

	dir := name
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Create fleet.yaml
	fleetYAML := fmt.Sprintf(`$schema: "%s"

fleet:
  name: %s

agents:
  - coder

egress-presets:
  default:
    - host: ["*"]  # allow all outbound traffic
`, fleetSchemaURL, name)

	if err := os.WriteFile(filepath.Join(dir, "fleet.yaml"), []byte(fleetYAML), 0644); err != nil {
		return fmt.Errorf("writing fleet.yaml: %w", err)
	}

	// Create agents/coder/agent.yaml
	agentDir := filepath.Join(dir, "agents", "coder")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("creating agent dir: %w", err)
	}

	agentYAML := fmt.Sprintf(`$schema: "%s"

egress:
  - default

runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
`, agentSchemaURL)

	if err := os.WriteFile(filepath.Join(agentDir, "agent.yaml"), []byte(agentYAML), 0644); err != nil {
		return fmt.Errorf("writing agent.yaml: %w", err)
	}

	// Create .env.example
	envExample := `# Agent Fleet Environment Variables
# Copy this file to .env and fill in your values.

# GitHub Personal Access Token (for egress credential injection)
# GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Telegram Bot Token (for channel messaging)
# TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz

# Telegram allowed user IDs (comma-separated)
# TELEGRAM_ALLOWED_USERS=123456789
`

	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(envExample), 0644); err != nil {
		return fmt.Errorf("writing .env.example: %w", err)
	}

	// Create .env (copy of example, gitignored)
	gitignore := `.env
.agent-fleet/
`
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	fmt.Printf("✓ Fleet initialized at ./%s\n", dir)
	fmt.Println()
	fmt.Println("  Files created:")
	fmt.Printf("    %s/fleet.yaml\n", dir)
	fmt.Printf("    %s/agents/coder/agent.yaml\n", dir)
	fmt.Printf("    %s/.env.example\n", dir)
	fmt.Printf("    %s/.gitignore\n", dir)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    1. cd %s\n", dir)
	fmt.Println("    2. cp .env.example .env  # fill in your secrets")
	fmt.Println("    3. agent-fleet up")

	return nil
}
