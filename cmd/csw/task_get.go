package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func taskGetCommand() *cobra.Command {
	var asJSON bool
	var asYAML bool
	var includeSummary bool

	command := &cobra.Command{
		Use:   "get {name|uuid}",
		Short: "Get task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}
			if asJSON && asYAML {
				return fmt.Errorf("taskGetCommand.RunE() [task.go]: --json and --yaml cannot be used together")
			}

			taskData, summaryMeta, summaryText, err := manager.GetTask(core.TaskLookup{Identifier: strings.TrimSpace(args[0])}, includeSummary)
			if err != nil {
				return err
			}

			if asJSON {
				payload := map[string]any{"task": taskData}
				if summaryMeta != nil {
					payload["summary_meta"] = summaryMeta
				}
				if strings.TrimSpace(summaryText) != "" {
					payload["summary"] = strings.TrimSpace(summaryText)
				}
				return outputJSON(payload)
			}

			if asYAML {
				payload := map[string]any{"task": taskData}
				if summaryMeta != nil {
					payload["summary_meta"] = summaryMeta
				}
				if strings.TrimSpace(summaryText) != "" {
					payload["summary"] = strings.TrimSpace(summaryText)
				}
				content, marshalErr := yaml.Marshal(payload)
				if marshalErr != nil {
					return fmt.Errorf("taskGetCommand.RunE() [task.go]: failed to marshal yaml: %w", marshalErr)
				}
				_, _ = fmt.Fprint(os.Stdout, string(content))
				return nil
			}

			printTaskHuman(taskData, summaryMeta, summaryText)
			return nil
		},
	}

	command.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	command.Flags().BoolVar(&asYAML, "yaml", false, "Output as YAML")
	command.Flags().BoolVar(&includeSummary, "summary", false, "Include latest session summary")

	return command
}

func printTaskHuman(taskData *core.Task, summaryMeta *core.TaskSessionSummary, summaryText string) {
	if taskData == nil {
		return
	}
	fmt.Fprintf(os.Stdout, "UUID: %s\n", taskData.UUID)
	fmt.Fprintf(os.Stdout, "Name: %s\n", taskData.Name)
	fmt.Fprintf(os.Stdout, "Description: %s\n", taskData.Description)
	fmt.Fprintf(os.Stdout, "Status: %s\n", taskData.Status)
	fmt.Fprintf(os.Stdout, "Feature branch: %s\n", taskData.FeatureBranch)
	fmt.Fprintf(os.Stdout, "Parent branch: %s\n", taskData.ParentBranch)
	fmt.Fprintf(os.Stdout, "Role: %s\n", taskData.Role)
	fmt.Fprintf(os.Stdout, "Parent task: %s\n", taskData.ParentTaskID)
	fmt.Fprintf(os.Stdout, "Deps: %s\n", strings.Join(taskData.Deps, ","))
	fmt.Fprintf(os.Stdout, "Sessions: %s\n", strings.Join(taskData.SessionIDs, ","))
	fmt.Fprintf(os.Stdout, "Subtasks: %s\n", strings.Join(taskData.SubtaskIDs, ","))
	fmt.Fprintf(os.Stdout, "Created: %s\n", taskData.CreatedAt)
	fmt.Fprintf(os.Stdout, "Updated: %s\n", taskData.UpdatedAt)
	if summaryMeta != nil {
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintf(os.Stdout, "Last summary session: %s (%s)\n", summaryMeta.SessionID, summaryMeta.Status)
		if strings.TrimSpace(summaryText) != "" {
			fmt.Fprintln(os.Stdout, strings.TrimSpace(summaryText))
		}
	}
}
