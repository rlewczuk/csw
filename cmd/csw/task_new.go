package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/spf13/cobra"
)

func taskNewCommand() *cobra.Command {
	var name string
	var description string
	var branch string
	var parentBranch string
	var role string
	var deps []string
	var prompt string
	var parent string
	var cliModel string
	var cliEditor string
	var cliWorkDir string
	var cliShadowDir string
	var cliConfigPath string
	var cliProjectConfig string

	command := &cobra.Command{
		Use:   "new",
		Short: "Create new persistent task",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPrompt, shouldCreate, err := func() (string, bool, error) {
				trimmedPrompt := strings.TrimSpace(prompt)
				if trimmedPrompt != "" {
					return trimmedPrompt, true, nil
				}

				workDir, err := system.ResolveWorkDir(strings.TrimSpace(cliWorkDir))
				if err != nil {
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to resolve work directory: %w", err)
				}

				editorCommand := strings.TrimSpace(cliEditor)
				if editorCommand == "" {
					editorCommand = strings.TrimSpace(os.Getenv("EDITOR"))
				}
				if editorCommand == "" {
					for _, candidate := range []string{"editor", "vim", "mcedit", "nano"} {
						if isTaskEditorAvailable(candidate) {
							editorCommand = candidate
							break
						}
					}
				}
				if editorCommand == "" {
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: no editor command found")
				}

				configRoot := workDir
				if strings.TrimSpace(cliShadowDir) != "" {
					resolvedShadowDir, shadowErr := system.ResolveWorkDir(strings.TrimSpace(cliShadowDir))
					if shadowErr != nil {
						return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to resolve shadow directory: %w", shadowErr)
					}
					configRoot = resolvedShadowDir
				}
				temporaryDir := filepath.Join(configRoot, ".cswdata", "tmp")
				if err := os.MkdirAll(temporaryDir, 0o755); err != nil {
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to create temporary directory: %w", err)
				}

				temporaryFile, err := os.CreateTemp(temporaryDir, "csw-task-new-*.md")
				if err != nil {
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to create temporary prompt file: %w", err)
				}
				temporaryFilePath := temporaryFile.Name()
				if closeErr := temporaryFile.Close(); closeErr != nil {
					_ = os.Remove(temporaryFilePath)
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to close temporary prompt file: %w", closeErr)
				}
				defer func() {
					_ = os.Remove(temporaryFilePath)
				}()

				editorProcess := exec.CommandContext(cmd.Context(), "sh", "-c", editorCommand+" "+shellQuote(temporaryFilePath))
				editorProcess.Stdin = os.Stdin
				editorProcess.Stdout = os.Stdout
				editorProcess.Stderr = os.Stderr

				if err := editorProcess.Run(); err != nil {
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to run editor: %w", err)
				}

				promptBytes, err := os.ReadFile(temporaryFilePath)
				if err != nil {
					if os.IsNotExist(err) {
						return "", false, nil
					}
					return "", false, fmt.Errorf("resolveTaskNewPrompt() [task.go]: failed to read temporary prompt file: %w", err)
				}

				editedPrompt := strings.TrimSpace(string(promptBytes))
				if editedPrompt == "" {
					return "", false, nil
				}

				return editedPrompt, true, nil
			}()
			if err != nil {
				return err
			}
			if !shouldCreate {
				fmt.Fprintln(os.Stdout, "Task not created: prompt is empty")
				return nil
			}

			createParams, err := resolveTaskCreateParams(cmd.Context(), taskCreateResolveParams{
				Prompt:        resolvedPrompt,
				Name:          strings.TrimSpace(name),
				Description:   strings.TrimSpace(description),
				Branch:        strings.TrimSpace(branch),
				ParentBranch:  strings.TrimSpace(parentBranch),
				Role:          strings.TrimSpace(role),
				ParentTaskID:  strings.TrimSpace(parent),
				Deps:          append([]string(nil), deps...),
				ModelName:     strings.TrimSpace(cliModel),
				WorkDir:       strings.TrimSpace(cliWorkDir),
				ShadowDir:     strings.TrimSpace(cliShadowDir),
				ProjectConfig: strings.TrimSpace(cliProjectConfig),
				ConfigPath:    strings.TrimSpace(cliConfigPath),
			})
			if err != nil {
				return err
			}

			manager, _, err := loadTaskBackend(cmd)
			if err != nil {
				return err
			}

			created, err := manager.CreateTask(createParams)
			if err != nil {
				return err
			}

			printTaskCreated(created)
			return nil
		},
	}

	command.Flags().StringVarP(&name, "name", "n", "", "Task name/slug")
	command.Flags().StringVarP(&description, "description", "d", "", "Task description")
	command.Flags().StringVarP(&branch, "branch", "b", "", "Feature branch name")
	command.Flags().StringVarP(&parentBranch, "parent-branch", "B", "", "Parent branch name")
	command.Flags().StringVarP(&role, "role", "r", "", "Task role")
	command.Flags().StringArrayVarP(&deps, "depends", "D", nil, "Dependency task UUID (repeatable)")
	command.Flags().StringVarP(&prompt, "prompt", "p", "", "Task prompt")
	command.Flags().StringVar(&cliModel, "model", "", "Model alias or model spec in provider/model format (single or comma-separated fallback list); if not set, uses defaults")
	command.Flags().StringVar(&cliEditor, "editor", "", "Editor command used for interactive prompt creation")
	command.Flags().StringVar(&cliWorkDir, "workdir", "", "Working directory (default: current directory)")
	command.Flags().StringVar(&cliShadowDir, "shadow-dir", "", "Shadow directory for agent files overlay (AGENTS.md, .agents*, .csw*, .cswdata)")
	command.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	command.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	command.Flags().StringVar(&parent, "parent", "", "Parent task name or UUID")

	return command
}

func resolveTaskCreateParams(ctx context.Context, params taskCreateResolveParams) (core.TaskCreateParams, error) {
	resolvedPrompt := strings.TrimSpace(params.Prompt)
	if resolvedPrompt == "" {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: prompt cannot be empty")
	}

	workDir, err := system.ResolveWorkDir(strings.TrimSpace(params.WorkDir))
	if err != nil {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve work directory: %w", err)
	}

	defaults, err := resolveTaskRunDefaultsFunc(system.ResolveRunDefaultsParams{
		WorkDir:       workDir,
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
	})
	if err != nil {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve CLI defaults: %w", err)
	}

	modelName := strings.TrimSpace(params.ModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(defaults.Model)
	}

	resolvedBranch := strings.TrimSpace(params.Branch)
	if resolvedBranch == "" {
		worktreeTemplate := strings.TrimSpace(defaults.Worktree)
		if worktreeTemplate == "" {
			worktreeTemplate = "%"
		}
		if !strings.HasSuffix(worktreeTemplate, "%") {
			worktreeTemplate += "-%"
		}

		generatedBranch, err := resolveTaskWorktreeBranchNameFunc(ctx, system.ResolveWorktreeBranchNameParams{
			Prompt:         resolvedPrompt,
			ModelName:      modelName,
			WorkDir:        workDir,
			ShadowDir:      strings.TrimSpace(params.ShadowDir),
			ProjectConfig:  strings.TrimSpace(params.ProjectConfig),
			ConfigPath:     strings.TrimSpace(params.ConfigPath),
			WorktreeBranch: worktreeTemplate,
		})
		if err != nil {
			return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve task branch: %w", err)
		}
		resolvedBranch = strings.TrimSpace(generatedBranch)
	}

	if resolvedBranch == "" {
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: resolved task branch cannot be empty")
	}

	resolvedName := strings.TrimSpace(params.Name)
	if resolvedName == "" {
		resolvedName = resolvedBranch
	}

	resolvedDescription := strings.TrimSpace(params.Description)
	if resolvedDescription == "" {
		generatedDescription, err := generateTaskDescriptionFunc(ctx, taskCreateResolveParams{
			Prompt:        resolvedPrompt,
			Branch:        resolvedBranch,
			Role:          strings.TrimSpace(params.Role),
			ModelName:     modelName,
			WorkDir:       workDir,
			ShadowDir:     strings.TrimSpace(params.ShadowDir),
			ProjectConfig: strings.TrimSpace(params.ProjectConfig),
			ConfigPath:    strings.TrimSpace(params.ConfigPath),
		})
		if err != nil {
			return core.TaskCreateParams{}, err
		}
		resolvedDescription = strings.TrimSpace(generatedDescription)
	}

	return core.TaskCreateParams{
		ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
		Name:          resolvedName,
		Description:   resolvedDescription,
		FeatureBranch: resolvedBranch,
		ParentBranch:  strings.TrimSpace(params.ParentBranch),
		Role:          strings.TrimSpace(params.Role),
		Deps:          append([]string(nil), params.Deps...),
		Prompt:        resolvedPrompt,
	}, nil
}

func generateTaskDescription(ctx context.Context, params taskCreateResolveParams) (string, error) {
	buildParams := system.BuildSystemParams{
		WorkDir:       strings.TrimSpace(params.WorkDir),
		ShadowDir:     strings.TrimSpace(params.ShadowDir),
		ConfigPath:    strings.TrimSpace(params.ConfigPath),
		ProjectConfig: strings.TrimSpace(params.ProjectConfig),
		ModelName:     strings.TrimSpace(params.ModelName),
		RoleName:      strings.TrimSpace(pickTaskRoleName(params.Role)),
	}

	sweSystem, buildResult, err := buildTaskDescriptionSystemFunc(buildParams)
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to build system: %w", err)
	}
	defer buildResult.Cleanup()

	modelRefs, err := models.ExpandProviderModelChain(strings.TrimSpace(buildResult.ModelName), sweSystem.ModelAliases)
	if err != nil || len(modelRefs) == 0 {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to parse resolved model name: %w", err)
	}

	providerName := strings.TrimSpace(modelRefs[0].Provider)
	provider, found := sweSystem.ModelProviders[providerName]
	if !found {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: provider not found: %s", providerName)
	}

	chatModel, err := newGenerationChatModelFromSpecFunc(
		strings.TrimSpace(buildResult.ModelName),
		sweSystem.ModelProviders,
		nil,
		sweSystem.Config,
		provider,
		sweSystem.ModelAliases,
		nil,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to create chat model chain: %w", err)
	}

	generatedDescription, err := core.GenerateCommitMessage(ctx, chatModel, sweSystem.Config, strings.TrimSpace(params.Prompt), strings.TrimSpace(params.Branch), "")
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to generate description: %w", err)
	}

	return strings.TrimSpace(generatedDescription), nil
}

func pickTaskRoleName(roleName string) string {
	trimmedRoleName := strings.TrimSpace(roleName)
	if trimmedRoleName == "" {
		return "developer"
	}

	return trimmedRoleName
}

func printTaskCreated(taskData *core.Task) {
	if taskData == nil {
		return
	}

	fmt.Fprintf(os.Stdout, "Task created: %s\n", strings.TrimSpace(taskData.UUID))
	fmt.Fprintf(os.Stdout, "Description: %s\n", strings.TrimSpace(taskData.Description))
}
