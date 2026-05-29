package cmd

import (
	"context"
	"fmt"

	"github.com/donbader/agent-fleet/pkg/selfupdate"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade agent-fleet to the latest release",
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	updater := selfupdate.New(selfupdate.Config{
		Repo:           "donbader/agent-fleet",
		CurrentVersion: version,
	})

	// Check for updates
	fmt.Println("Checking for updates...")
	release, err := updater.CheckUpdate(ctx)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if release == nil {
		fmt.Printf("✓ Already on latest version (%s)\n", version)
		return nil
	}

	fmt.Printf("New version available: %s → %s\n", version, release.Version)

	// Download and replace
	fmt.Println("Downloading...")
	if err := updater.Apply(ctx, release); err != nil {
		return fmt.Errorf("applying update: %w", err)
	}

	fmt.Printf("✓ Upgraded to %s\n", release.Version)
	return nil
}
