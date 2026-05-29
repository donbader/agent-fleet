package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize fleet config in the current directory",
	Long:  "Creates fleet.yaml and a sample agent config if they don't already exist.\nScans config files for ${VAR} references and generates .env.example.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const fleetSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/fleet.schema.json"
const agentSchemaURL = "https://raw.githubusercontent.com/donbader/agent-fleet/main/schemas/agent.schema.json"

// envVarPattern matches ${VAR_NAME} interpolation in YAML values.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

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

	// Scan all config files for ${VAR} references and generate .env.example
	vars := scanEnvVars(".")
	if err := writeEnvExample(vars); err != nil {
		return err
	}
	if len(vars) > 0 {
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

// scanEnvVars walks the directory for .yaml files and extracts ${VAR} references.
func scanEnvVars(dir string) []string {
	seen := make(map[string]bool)

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and .agent-fleet/
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules") {
			return filepath.SkipDir
		}
		// Only scan .yaml and .yml files
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		matches := envVarPattern.FindAllSubmatch(data, -1)
		for _, m := range matches {
			seen[string(m[1])] = true
		}
		return nil
	})

	vars := make([]string, 0, len(seen))
	for v := range seen {
		vars = append(vars, v)
	}
	sort.Strings(vars)
	return vars
}

// writeEnvExample generates .env.example from discovered env var references.
func writeEnvExample(vars []string) error {
	if len(vars) == 0 {
		// No env vars referenced — write a minimal placeholder
		if _, err := os.Stat(".env.example"); os.IsNotExist(err) {
			content := "# No ${VAR} references found in config files.\n# Add variables here as needed.\n"
			return os.WriteFile(".env.example", []byte(content), 0644)
		}
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Generated from fleet config — fill in your values.\n")
	sb.WriteString("# Variables referenced via ${VAR} in fleet.yaml / agent.yaml.\n\n")

	for _, v := range vars {
		sb.WriteString(fmt.Sprintf("%s=\n", v))
	}

	return os.WriteFile(".env.example", []byte(sb.String()), 0644)
}
