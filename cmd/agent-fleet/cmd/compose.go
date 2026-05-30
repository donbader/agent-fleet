package cmd

import (
	"context"

	"github.com/donbader/agent-fleet/pkg/fleet"
	"github.com/spf13/cobra"
)

var composeCmd = &cobra.Command{
	Use:                "compose [args...]",
	Short:              "Run docker compose commands with fleet context",
	Long:               `Passes through to docker compose with the correct project name and compose file. All arguments after "compose" are forwarded directly.`,
	DisableFlagParsing: true,
	RunE:               runCompose,
	Example: `  agent-fleet compose exec coder bash
  agent-fleet compose logs coder -f
  agent-fleet compose restart coder
  agent-fleet compose top`,
}

func init() {
	rootCmd.AddCommand(composeCmd)
}

func runCompose(cmd *cobra.Command, args []string) error {
	f := fleet.New(fleet.Options{FleetFile: fleetFile})
	return f.Compose(context.Background(), args)
}
