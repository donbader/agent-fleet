package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Template utilities for provider render scripts",
}

var templateInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Substitute variables in a partial Dockerfile template",
	Long: `Reads a partial Dockerfile, substitutes ${VAR} patterns with provided values,
and outputs the result to stdout. Unknown variables are left as-is.

Used by provider render scripts to process user_base templates.`,
	RunE: runTemplateInject,
}

var (
	templateSource string
	templateVars   []string
)

func init() {
	templateInjectCmd.Flags().StringVar(&templateSource, "source", "", "Path to the template file")
	templateInjectCmd.Flags().StringArrayVar(&templateVars, "var", nil, "Variable substitution (KEY=VALUE), can be repeated")
	_ = templateInjectCmd.MarkFlagRequired("source")

	templateCmd.AddCommand(templateInjectCmd)
	rootCmd.AddCommand(templateCmd)
}

func runTemplateInject(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(templateSource)
	if err != nil {
		return fmt.Errorf("reading template %q: %w", templateSource, err)
	}

	vars := parseVarFlags(templateVars)
	result := substituteVars(string(content), vars)

	fmt.Print(result)
	return nil
}

// parseVarFlags parses KEY=VALUE strings into a map.
func parseVarFlags(flags []string) map[string]string {
	vars := make(map[string]string, len(flags))
	for _, f := range flags {
		key, value, ok := strings.Cut(f, "=")
		if ok {
			vars[key] = value
		}
	}
	return vars
}

// substituteVars replaces ${VAR} patterns in input with values from vars.
// Unknown variables (not in vars) are left as-is.
func substituteVars(input string, vars map[string]string) string {
	if len(vars) == 0 || input == "" {
		return input
	}

	result := input
	for key, value := range vars {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
	}
	return result
}
