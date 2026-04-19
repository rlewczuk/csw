package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func taskMergeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "merge {name|uuid}",
		Short: "Merge task feature branch to parent branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, backend, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			merged, err := backend.MergeTask(cmd.Context(), strings.TrimSpace(args[0]), "")
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Task merged: %s (%s -> %s)\n", merged.UUID, merged.FeatureBranch, merged.ParentBranch)
			return nil
		},
	}

	return command
}
