package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
)

func taskArchiveCommand() *cobra.Command {
	var status string

	command := &cobra.Command{
		Use:   "archive [name|uuid]",
		Short: "Archive tasks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			return runTaskArchive(manager, args, status, os.Stdout)
		},
	}

	command.Flags().StringVar(&status, "status", "", "Archive all tasks by status")

	return command
}

func runTaskArchive(manager *core.TaskManager, args []string, status string, output io.Writer) error {
	if manager == nil {
		return fmt.Errorf("runTaskArchive() [task.go]: manager cannot be nil")
	}
	if output == nil {
		return fmt.Errorf("runTaskArchive() [task.go]: output cannot be nil")
	}

	trimmedStatus := strings.TrimSpace(status)
	if len(args) == 1 && trimmedStatus != "" {
		return fmt.Errorf("runTaskArchive() [task.go]: task identifier and --status cannot be used together")
	}

	if len(args) == 1 {
		archivedTask, archiveErr := manager.ArchiveTask(core.TaskLookup{Identifier: strings.TrimSpace(args[0])})
		if archiveErr != nil {
			return archiveErr
		}
		_, _ = fmt.Fprintf(output, "Task archived: %s\t%s\n", archivedTask.UUID, archivedTask.Name)
		return nil
	}

	if trimmedStatus == "" {
		trimmedStatus = core.TaskStatusMerged
	}
	archivedTasks, archiveErr := manager.ArchiveTasksByStatus(trimmedStatus)
	if archiveErr != nil {
		return archiveErr
	}

	sort.Slice(archivedTasks, func(i, j int) bool {
		if archivedTasks[i] == nil || archivedTasks[j] == nil {
			return i < j
		}
		if archivedTasks[i].Name == archivedTasks[j].Name {
			return archivedTasks[i].UUID < archivedTasks[j].UUID
		}
		return archivedTasks[i].Name < archivedTasks[j].Name
	})
	for _, archivedTask := range archivedTasks {
		if archivedTask == nil {
			continue
		}
		_, _ = fmt.Fprintf(output, "Task archived: %s\t%s\n", archivedTask.UUID, archivedTask.Name)
	}

	return nil
}
