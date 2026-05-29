package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		var permErr *selfupdate.ErrPermissionDenied
		if errors.As(err, &permErr) {
			return rerunWithSudo()
		}
		return fmt.Errorf("applying update: %w", err)
	}

	fmt.Printf("✓ Upgraded to %s\n", release.Version)
	return nil
}

// rerunWithSudo prompts the user and re-executes the upgrade with sudo.
func rerunWithSudo() error {
	fmt.Print("Permission denied. Re-run with sudo? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		return fmt.Errorf("upgrade cancelled (need write access to binary location)")
	}

	// Re-exec with sudo
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	cmd := exec.Command("sudo", exePath, "upgrade")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
