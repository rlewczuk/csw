package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
)

func taskMergeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "merge {name|uuid}",
		Short: "Merge task feature branch to parent branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, vcsRepo, err := loadTaskManager(cmd)
			if err != nil {
				return err
			}

			merged, err := manager.MergeTask(core.TaskLookup{Identifier: strings.TrimSpace(args[0])}, vcsRepo)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Task merged: %s (%s -> %s)\n", merged.UUID, merged.FeatureBranch, merged.ParentBranch)
			return nil
		},
	}

	return command
}
