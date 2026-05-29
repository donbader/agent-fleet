package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var ctxDefault string

var ctxCmd = &cobra.Command{
	Use:   "ctx <path>",
	Short: "Extract a value from the render context (used by provider render scripts)",
	Long: `Extract a value from the render context JSON using dot-path notation.

The render context is read from the RENDER_CONTEXT environment variable,
which is set automatically by agent-fleet when executing render scripts.

Examples:
  agent-fleet ctx .name
  agent-fleet ctx .options.auth_port --default 1455
  agent-fleet ctx .gateway_host
  agent-fleet ctx .env.OPENAI_API_KEY`,
	Args: cobra.ExactArgs(1),
	RunE: runCtx,
}

func init() {
	ctxCmd.Flags().StringVar(&ctxDefault, "default", "", "default value if path not found")
	rootCmd.AddCommand(ctxCmd)
}

func runCtx(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Read context from RENDER_CONTEXT env var
	contextJSON := os.Getenv("RENDER_CONTEXT")
	if contextJSON == "" {
		if ctxDefault != "" {
			fmt.Print(ctxDefault)
			return nil
		}
		return fmt.Errorf("RENDER_CONTEXT environment variable not set")
	}

	// Parse JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(contextJSON), &data); err != nil {
		return fmt.Errorf("parsing RENDER_CONTEXT: %w", err)
	}

	// Navigate dot-path
	value, found := navigatePath(data, path)
	if !found {
		if ctxDefault != "" {
			fmt.Print(ctxDefault)
			return nil
		}
		return fmt.Errorf("path %q not found in context", path)
	}

	// Output value
	switch v := value.(type) {
	case string:
		fmt.Print(v)
	case float64:
		// Output integers without decimal point
		if v == float64(int64(v)) {
			fmt.Print(int64(v))
		} else {
			fmt.Print(v)
		}
	case bool:
		fmt.Print(v)
	case nil:
		if ctxDefault != "" {
			fmt.Print(ctxDefault)
		}
	default:
		// For complex types, output as JSON
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshaling value: %w", err)
		}
		fmt.Print(string(b))
	}

	return nil
}

// navigatePath traverses a nested map using dot-path notation.
// Path must start with "." (e.g., ".name", ".options.auth_port").
func navigatePath(data map[string]any, path string) (any, bool) {
	// Strip leading dot
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return data, true
	}

	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}
