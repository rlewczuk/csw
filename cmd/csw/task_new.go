package main

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
)

const (
	taskNewFallbackBranchName  = "unnamed"
	taskNewFallbackDescription = "New Task"
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
	var cliConfigPath string
	var cliProjectConfig string
	var noCommit bool

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
				if strings.TrimSpace(shadowDir) != "" {
					resolvedShadowDir, shadowErr := system.ResolveWorkDir(strings.TrimSpace(shadowDir))
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
				NoCommit:      noCommit,
				ParentBranch:  strings.TrimSpace(parentBranch),
				Role:          strings.TrimSpace(role),
				ParentTaskID:  strings.TrimSpace(parent),
				Deps:          append([]string(nil), deps...),
				ModelName:     strings.TrimSpace(cliModel),
				WorkDir:       strings.TrimSpace(cliWorkDir),
				ShadowDir:     strings.TrimSpace(shadowDir),
				ProjectConfig: strings.TrimSpace(cliProjectConfig),
				ConfigPath:    strings.TrimSpace(cliConfigPath),
			})
			if err != nil {
				return err
			}

			manager, _, err := loadTaskManager(cmd)
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
	command.Flags().StringVar(&cliConfigPath, "config-path", "", "Colon-separated list of config directories (optional, added to default hierarchy)")
	command.Flags().StringVar(&cliProjectConfig, "project-config", "", "Custom project config directory (default: .csw/config)")
	command.Flags().StringVar(&parent, "parent", "", "Parent task name or UUID")
	command.Flags().BoolVar(&noCommit, "no-commit", false, "Do not commit task results automatically")

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
		return core.TaskCreateParams{}, fmt.Errorf("resolveTaskCreateParams() [task.go]: failed to resolve CLI parameters: %w", err)
	}

	modelName := strings.TrimSpace(params.ModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(defaults.Model)
	}

	resolvedBranch := strings.TrimSpace(params.Branch)
	if params.NoCommit || defaults.NoCommit {
		resolvedBranch = ""
	}
	if resolvedBranch == "" {
		if params.NoCommit || defaults.NoCommit {
			resolvedName := strings.TrimSpace(params.Name)
			if resolvedName == "" {
				resolvedName = taskNewFallbackBranchName
			}

			resolvedDescription := strings.TrimSpace(params.Description)
			if resolvedDescription == "" {
				generatedDescription, err := generateTaskDescriptionFunc(ctx, taskCreateResolveParams{
					Prompt:        resolvedPrompt,
					Branch:        taskNewFallbackBranchName,
					Role:          strings.TrimSpace(params.Role),
					ModelName:     modelName,
					WorkDir:       workDir,
					ShadowDir:     strings.TrimSpace(params.ShadowDir),
					ProjectConfig: strings.TrimSpace(params.ProjectConfig),
					ConfigPath:    strings.TrimSpace(params.ConfigPath),
				})
				if err != nil {
					resolvedDescription = taskNewFallbackDescription
				} else {
					resolvedDescription = strings.TrimSpace(generatedDescription)
				}
			}
			if resolvedDescription == "" {
				resolvedDescription = taskNewFallbackDescription
			}

			return core.TaskCreateParams{
				ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
				Name:          resolvedName,
				Description:   resolvedDescription,
				FeatureBranch: "",
				NoCommit:      true,
				ParentBranch:  strings.TrimSpace(params.ParentBranch),
				Role:          strings.TrimSpace(params.Role),
				Deps:          append([]string(nil), params.Deps...),
				Prompt:        resolvedPrompt,
			}, nil
		}

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
			resolvedBranch = taskNewFallbackBranchName
		} else {
			resolvedBranch = strings.TrimSpace(generatedBranch)
		}
	}

	if resolvedBranch == "" {
		resolvedBranch = taskNewFallbackBranchName
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
			resolvedDescription = taskNewFallbackDescription
		} else {
			resolvedDescription = strings.TrimSpace(generatedDescription)
		}
	}

	if resolvedDescription == "" {
		resolvedDescription = taskNewFallbackDescription
	}

	return core.TaskCreateParams{
		ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
		Name:          resolvedName,
		Description:   resolvedDescription,
		FeatureBranch: resolvedBranch,
		NoCommit:      params.NoCommit,
		ParentBranch:  strings.TrimSpace(params.ParentBranch),
		Role:          strings.TrimSpace(params.Role),
		Deps:          append([]string(nil), params.Deps...),
		Prompt:        resolvedPrompt,
	}, nil
}

func generateTaskDescription(ctx context.Context, params taskCreateResolveParams) (string, error) {
	configPath, err := system.BuildConfigPath(strings.TrimSpace(params.ProjectConfig), strings.TrimSpace(params.ConfigPath))
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to build config path: %w", err)
	}
	configStore, err := conf.CswConfigLoad(configPath)
	if err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to load config: %w", err)
	}
	if configStore.GlobalConfig == nil {
		configStore.GlobalConfig = &conf.GlobalConfig{}
	}
	parameters := &configStore.GlobalConfig.Parameters
	parameters.Workdir = strings.TrimSpace(params.WorkDir)
	parameters.ShadowDir = strings.TrimSpace(params.ShadowDir)
	parameters.Model = strings.TrimSpace(params.ModelName)
	parameters.Role = strings.TrimSpace(pickTaskRoleName(params.Role))

	logRoot := strings.TrimSpace(params.WorkDir)
	if strings.TrimSpace(params.ShadowDir) != "" {
		logRoot = strings.TrimSpace(params.ShadowDir)
	}
	if logRoot == "" {
		logRoot, err = system.ResolveWorkDir("")
		if err != nil {
			return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to resolve log root: %w", err)
		}
	}
	if err := logging.SetLogsDirectory(filepath.Join(logRoot, ".cswdata"), true); err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to initialize task description logging: %w", err)
	}
	logger := logging.GetGlobalLogger()
	logger.Info("task_description_generation_configured",
		"model", strings.TrimSpace(parameters.Model),
		"branch", strings.TrimSpace(params.Branch),
		"role", strings.TrimSpace(parameters.Role),
		"work_dir", strings.TrimSpace(params.WorkDir),
		"shadow_dir", strings.TrimSpace(params.ShadowDir),
	)

	sweSystem, err := buildTaskDescriptionSystemFunc(configStore)
	if err != nil {
		logger.Error("task_description_generation_system_build_failed", "error", err.Error())
		_ = logging.FlushLogs()
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to build system: %w", err)
	}
	if err := logging.SetLogsDirectory(filepath.Join(logRoot, ".cswdata"), true); err != nil {
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to restore task description logging: %w", err)
	}
	logger = logging.GetGlobalLogger()
	runtimeConfig := configStore.Runtime
	if runtimeConfig.Cleanup != nil {
		defer runtimeConfig.Cleanup()
	}

	modelRefs, err := models.ExpandProviderModelChain(strings.TrimSpace(parameters.Model), sweSystem.ModelAliases)
	if err != nil || len(modelRefs) == 0 {
		logger.Error("task_description_generation_model_parse_failed", "error", fmt.Sprintf("%v", err))
		_ = logging.FlushLogs()
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to parse resolved model name: %w", err)
	}

	providerName := strings.TrimSpace(modelRefs[0].Provider)
	provider, found := sweSystem.ModelProviders[providerName]
	if !found {
		logger.Error("task_description_generation_provider_not_found", "provider", providerName)
		_ = logging.FlushLogs()
		return "", fmt.Errorf("generateTaskDescription() [task.go]: provider not found: %s", providerName)
	}

	chatModel, err := newGenerationChatModelFromSpecFunc(
		strings.TrimSpace(parameters.Model),
		sweSystem.ModelProviders,
		nil,
		sweSystem.Config,
		provider,
		sweSystem.ModelAliases,
		nil,
		nil,
	)
	if err != nil {
		logger.Error("task_description_generation_chat_model_create_failed", "error", err.Error())
		_ = logging.FlushLogs()
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to create chat model chain: %w", err)
	}
	logger.Info("task_description_generation_start",
		"model", strings.TrimSpace(parameters.Model),
		"provider", providerName,
		"branch", strings.TrimSpace(params.Branch),
		"role", strings.TrimSpace(parameters.Role),
	)
	chatModel = &taskDescriptionLoggingChatModel{
		delegate: chatModel,
		logger:   logger,
		model:    strings.TrimSpace(parameters.Model),
		provider: providerName,
		branch:   strings.TrimSpace(params.Branch),
	}

	generatedDescription, err := core.GenerateCommitMessage(ctx, chatModel, sweSystem.Config, strings.TrimSpace(params.Prompt), strings.TrimSpace(params.Branch), "")
	if err != nil {
		logger.Error("task_description_generation_failed", "error", err.Error())
		_ = logging.FlushLogs()
		return "", fmt.Errorf("generateTaskDescription() [task.go]: failed to generate description: %w", err)
	}

	trimmedDescription := strings.TrimSpace(generatedDescription)
	logger.Info("task_description_generation_complete", "description", trimmedDescription)
	_ = logging.FlushLogs()
	return trimmedDescription, nil
}

// taskDescriptionLoggingChatModel logs task-description LLM prompts and responses.
type taskDescriptionLoggingChatModel struct {
	delegate models.ChatModel
	logger   *slog.Logger
	model    string
	provider string
	branch   string
}

func (m *taskDescriptionLoggingChatModel) Compactor() models.ChatCompator {
	return m.delegate.Compactor()
}

func (m *taskDescriptionLoggingChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	if m.logger != nil {
		m.logger.Info("task_description_llm_prompt",
			"provider", m.provider,
			"model", m.model,
			"branch", m.branch,
			"messages", messages,
		)
	}

	response, err := m.delegate.Chat(ctx, messages, options, tools)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("task_description_llm_response_error",
				"provider", m.provider,
				"model", m.model,
				"branch", m.branch,
				"error", err.Error(),
			)
		}
		return nil, err
	}

	if m.logger != nil {
		m.logger.Info("task_description_llm_response",
			"provider", m.provider,
			"model", m.model,
			"branch", m.branch,
			"response", response,
			"response_text", response.GetText(),
		)
	}

	return response, nil
}

func (m *taskDescriptionLoggingChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	return m.delegate.ChatStream(ctx, messages, options, tools)
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
