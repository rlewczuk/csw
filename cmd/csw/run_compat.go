package main

import (
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/commands"
	"github.com/rlewczuk/csw/pkg/core"
	sessionio "github.com/rlewczuk/csw/pkg/io"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"gopkg.in/yaml.v3"
)

type RunParams = system.RunParams

const defaultBashRunTimeout = 120 * time.Second

func runCommand(params *RunParams) error {
	return system.RunCommand(params)
}

func renderCommandPrompt(params *RunParams, workDir string, shellRunner runner.CommandRunner, hostShellRunner runner.CommandRunner) error {
	if params == nil {
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: params is nil")
	}
	if strings.TrimSpace(params.CommandName) == "" {
		return nil
	}

	template := params.CommandTemplate
	if strings.TrimSpace(template) == "" {
		template = params.Prompt
	}

	withArguments := commands.ApplyArguments(template, params.CommandArgs)
	expandedPrompt, err := commands.ExpandPrompt(withArguments, workDir, shellRunner, hostShellRunner)
	if err != nil {
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: failed to render command /%s: %w", params.CommandName, err)
	}

	params.Prompt = strings.TrimSpace(expandedPrompt)
	if params.Prompt == "" {
		return fmt.Errorf("renderCommandPrompt() [run_compat.go]: rendered command /%s prompt is empty", params.CommandName)
	}

	return nil
}

func PreparePromptWithContext(params *RunParams) error {
	return system.PreparePromptWithContext(params)
}

func BuildContainerStartupInfoMessage(buildResult system.BuildSystemResult) string {
	return system.BuildContainerStartupInfoMessage(buildResult)
}

func buildRunSessionOutput(params *RunParams, output stdio.Writer) core.SessionThreadOutput {
	if params == nil {
		return sessionio.NewTextSessionOutput(output)
	}
	if strings.TrimSpace(params.OutputFormat) == "jsonl" {
		return sessionio.NewJsonlSessionOutput(output)
	}
	return sessionio.NewTextSessionOutputWithSlug(output, params.WorktreeBranch)
}

type runSessionInput interface{ StartReadingInput() }

func buildRunStdinSessionInput(params *RunParams, thread core.SessionThreadInput, input stdio.Reader) runSessionInput {
	if params == nil || thread == nil || input == nil {
		return nil
	}
	if params.OutputFormat == "jsonl" {
		return sessionio.NewJsonlSessionInput(input, thread)
	}
	return sessionio.NewTextSessionInput(input, thread)
}

func buildSummaryMessageFunc(output core.SessionThreadOutput) func(string, shared.MessageType) {
	if output == nil {
		return nil
	}
	return func(message string, messageType shared.MessageType) {
		output.ShowMessage(message, string(messageType))
	}
}

func validateMergeRunParams(params *RunParams) error {
	if params == nil {
		return fmt.Errorf("validateMergeRunParams() [run_compat.go]: params cannot be nil")
	}
	if params.Merge && strings.TrimSpace(params.WorktreeBranch) == "" {
		return fmt.Errorf("runCommand() [run_compat.go]: --merge requires --worktree")
	}
	return nil
}

func resolveTaskRunMerge(mergeFlagChanged bool, cliMerge bool, cliWorktree string, resolver runDefaultsResolver, workDir string, shadowDir string, projectConfig string, configPath string) bool {
	if mergeFlagChanged {
		return cliMerge
	}
	if strings.TrimSpace(cliWorktree) != "" {
		return cliMerge
	}
	if resolver == nil {
		return cliMerge
	}
	defaults, err := resolver(system.ResolveRunDefaultsParams{WorkDir: workDir, ShadowDir: shadowDir, ProjectConfig: projectConfig, ConfigPath: configPath})
	if err != nil {
		return cliMerge
	}
	if defaults.Merge {
		return true
	}
	return cliMerge
}

func cloneRunTask(task *core.Task) *core.Task {
	if task == nil {
		return nil
	}
	cloned := *task
	cloned.Deps = append([]string(nil), task.Deps...)
	cloned.SessionIDs = append([]string(nil), task.SessionIDs...)
	cloned.SubtaskIDs = append([]string(nil), task.SubtaskIDs...)
	return &cloned
}

func applyCommandTaskMetadata(params *RunParams) error {
	if params == nil || params.Task == nil || params.CommandTaskMetadata == nil {
		return nil
	}
	taskDir := strings.TrimSpace(params.Task.TaskDir)
	if taskDir == "" {
		return nil
	}
	taskFilePath := filepath.Join(taskDir, "task.yml")
	taskBytes, err := os.ReadFile(taskFilePath)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to read task metadata: %w", err)
	}
	var persistedTask core.Task
	if err := yaml.Unmarshal(taskBytes, &persistedTask); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to parse task metadata: %w", err)
	}
	applyIfUnchanged := func(fieldName string, apply func()) {
		if params.InitialTask == nil {
			apply()
			return
		}
		currentField := reflect.ValueOf(persistedTask).FieldByName(fieldName)
		initialField := reflect.ValueOf(*params.InitialTask).FieldByName(fieldName)
		if !currentField.IsValid() || !initialField.IsValid() {
			return
		}
		if reflect.DeepEqual(currentField.Interface(), initialField.Interface()) {
			apply()
		}
	}
	metadata := params.CommandTaskMetadata
	if metadata.UUID != nil {
		applyIfUnchanged("UUID", func() { persistedTask.UUID = strings.TrimSpace(*metadata.UUID) })
	}
	if metadata.Name != nil {
		applyIfUnchanged("Name", func() { persistedTask.Name = strings.TrimSpace(*metadata.Name) })
	}
	if metadata.Description != nil {
		applyIfUnchanged("Description", func() { persistedTask.Description = strings.TrimSpace(*metadata.Description) })
	}
	if metadata.Status != nil {
		applyIfUnchanged("Status", func() { persistedTask.Status = strings.TrimSpace(*metadata.Status) })
	}
	if metadata.FeatureBranch != nil {
		applyIfUnchanged("FeatureBranch", func() { persistedTask.FeatureBranch = strings.TrimSpace(*metadata.FeatureBranch) })
	}
	if metadata.ParentBranch != nil {
		applyIfUnchanged("ParentBranch", func() { persistedTask.ParentBranch = strings.TrimSpace(*metadata.ParentBranch) })
	}
	if metadata.Role != nil {
		applyIfUnchanged("Role", func() { persistedTask.Role = strings.TrimSpace(*metadata.Role) })
	}
	if metadata.Deps != nil {
		applyIfUnchanged("Deps", func() { persistedTask.Deps = append([]string(nil), (*metadata.Deps)...) })
	}
	if metadata.SessionIDs != nil {
		applyIfUnchanged("SessionIDs", func() { persistedTask.SessionIDs = append([]string(nil), (*metadata.SessionIDs)...) })
	}
	if metadata.SubtaskIDs != nil {
		applyIfUnchanged("SubtaskIDs", func() { persistedTask.SubtaskIDs = append([]string(nil), (*metadata.SubtaskIDs)...) })
	}
	if metadata.ParentTaskID != nil {
		applyIfUnchanged("ParentTaskID", func() { persistedTask.ParentTaskID = strings.TrimSpace(*metadata.ParentTaskID) })
	}
	if metadata.CreatedAt != nil {
		applyIfUnchanged("CreatedAt", func() { persistedTask.CreatedAt = strings.TrimSpace(*metadata.CreatedAt) })
	}
	if metadata.UpdatedAt != nil {
		applyIfUnchanged("UpdatedAt", func() { persistedTask.UpdatedAt = strings.TrimSpace(*metadata.UpdatedAt) })
	}
	updatedBytes, err := yaml.Marshal(&persistedTask)
	if err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to serialize task metadata: %w", err)
	}
	if err := os.WriteFile(taskFilePath, updatedBytes, 0o644); err != nil {
		return fmt.Errorf("applyCommandTaskMetadata() [run_compat.go]: failed to persist task metadata: %w", err)
	}
	return nil
}
