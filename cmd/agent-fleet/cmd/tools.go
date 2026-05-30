package cmd

import (
	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Helper tools for provider render scripts",
	Long:  `Subcommands used by provider render scripts. Not typically called directly by users.`,
}

func init() {
	rootCmd.AddCommand(toolsCmd)
}
