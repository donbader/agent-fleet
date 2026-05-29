package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize fleet config in the current directory",
	Long:  "Creates fleet.yaml and a sample agent config if they don't already exist.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const fleetSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/fleet.schema.json"
const agentSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/agent.schema.json"

func runInit(cmd *cobra.Command, args []string) error {
	created := []string{}

	// Create fleet.yaml if missing
	if _, err := os.Stat("fleet.yaml"); os.IsNotExist(err) {
		fleetYAML := fmt.Sprintf(`# yaml-language-server: $schema=%s

fleet:
  name: my-fleet

agents:
  - coder

egress-presets:
  default:
    - host: ["*"]  # allow all outbound traffic
`, fleetSchemaURL)

		if err := os.WriteFile("fleet.yaml", []byte(fleetYAML), 0644); err != nil {
			return fmt.Errorf("writing fleet.yaml: %w", err)
		}
		created = append(created, "fleet.yaml")
	} else {
		fmt.Println("fleet.yaml already exists, skipping")
	}

	// Create agents/coder/agent.yaml if missing
	agentDir := filepath.Join("agents", "coder")
	agentFile := filepath.Join(agentDir, "agent.yaml")
	if _, err := os.Stat(agentFile); os.IsNotExist(err) {
		if err := os.MkdirAll(agentDir, 0755); err != nil {
			return fmt.Errorf("creating agent dir: %w", err)
		}

		agentYAML := fmt.Sprintf(`# yaml-language-server: $schema=%s

egress:
  - default

runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
`, agentSchemaURL)

		if err := os.WriteFile(agentFile, []byte(agentYAML), 0644); err != nil {
			return fmt.Errorf("writing agent.yaml: %w", err)
		}
		created = append(created, agentFile)
	} else {
		fmt.Println("agents/coder/agent.yaml already exists, skipping")
	}

	// Create .env.example if missing
	if _, err := os.Stat(".env.example"); os.IsNotExist(err) {
		envExample := `# Agent Fleet Environment Variables
# Copy this file to .env and fill in your values.

# GitHub Personal Access Token (for egress credential injection)
# GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Telegram Bot Token (for channel messaging)
# TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz

# Telegram allowed users (comma-separated IDs or usernames, e.g. 123456789,@myuser)
# TELEGRAM_ALLOWED_USERS=123456789,@myuser
`
		if err := os.WriteFile(".env.example", []byte(envExample), 0644); err != nil {
			return fmt.Errorf("writing .env.example: %w", err)
		}
		created = append(created, ".env.example")
	}

	// Create .gitignore if missing
	if _, err := os.Stat(".gitignore"); os.IsNotExist(err) {
		gitignore := `.env
.agent-fleet/
`
		if err := os.WriteFile(".gitignore", []byte(gitignore), 0644); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
		created = append(created, ".gitignore")
	}

	if len(created) == 0 {
		fmt.Println("✓ Nothing to do — all files already exist")
	} else {
		fmt.Println("✓ Initialized fleet config:")
		for _, f := range created {
			fmt.Printf("    %s\n", f)
		}
		fmt.Println()
		fmt.Println("  Next steps:")
		fmt.Println("    1. cp .env.example .env  # fill in your secrets")
		fmt.Println("    2. agent-fleet up")
	}

	return nil
}
