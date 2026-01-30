package main

import (
	"github.com/spf13/cobra"
)

// ConfCommand creates the conf command with all subcommands.
func ConfCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conf",
		Short: "Manage configuration",
		Long:  "Manage configuration including providers, roles, and other settings",
	}

	// Add subcommands
	cmd.AddCommand(ProviderCommand())

	return cmd
}
