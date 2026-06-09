package main

import (
	"context"
	"fmt"
	"iter"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTaskCreateParamsGeneratesBranchAndDescription(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalBranchResolver := resolveTaskWorktreeBranchNameFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		resolveTaskWorktreeBranchNameFunc = originalBranchResolver
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunParameters, error) {
		_ = params
		return conf.RunParameters{Model: "provider/model", Worktree: "feature/%"}, nil
	}

	resolveTaskWorktreeBranchNameFunc = func(ctx context.Context, params system.ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "provider/model", params.ModelName)
		assert.Equal(t, "feature/%", params.WorktreeBranch)
		return "feature/generated", nil
	}

	generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		assert.Equal(t, "feature/generated", params.Branch)
		assert.Equal(t, "provider/model", params.ModelName)
		return "generated description", nil
	}

	resolved, err := resolveTaskCreateParams(context.Background(), taskCreateResolveParams{Prompt: "do this task"})
	require.NoError(t, err)
	assert.Equal(t, "feature/generated", resolved.FeatureBranch)
	assert.Equal(t, "feature/generated", resolved.Name)
	assert.Equal(t, "generated description", resolved.Description)
	assert.Equal(t, "do this task", resolved.Prompt)
}

func TestResolveTaskCreateParamsFallsBackWhenGenerationFails(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalBranchResolver := resolveTaskWorktreeBranchNameFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		resolveTaskWorktreeBranchNameFunc = originalBranchResolver
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunParameters, error) {
		_ = params
		return conf.RunParameters{Model: "provider/model", Worktree: "feature/%"}, nil
	}

	resolveTaskWorktreeBranchNameFunc = func(ctx context.Context, params system.ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		assert.Equal(t, "do this task", params.Prompt)
		return "", fmt.Errorf("uncorrectable llm error")
	}

	generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
		_ = ctx
		assert.Equal(t, taskNewFallbackBranchName, params.Branch)
		assert.Equal(t, "do this task", params.Prompt)
		return "", fmt.Errorf("uncorrectable llm error")
	}

	resolved, err := resolveTaskCreateParams(context.Background(), taskCreateResolveParams{Prompt: "do this task"})
	require.NoError(t, err)
	assert.Equal(t, taskNewFallbackBranchName, resolved.FeatureBranch)
	assert.Equal(t, taskNewFallbackBranchName, resolved.Name)
	assert.Equal(t, taskNewFallbackDescription, resolved.Description)
	assert.Equal(t, "do this task", resolved.Prompt)
}

func TestResolveTaskCreateParamsFallsBackWhenGenerationReturnsEmptyValues(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalBranchResolver := resolveTaskWorktreeBranchNameFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		resolveTaskWorktreeBranchNameFunc = originalBranchResolver
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunParameters, error) {
		_ = params
		return conf.RunParameters{Model: "provider/model", Worktree: "feature/%"}, nil
	}

	resolveTaskWorktreeBranchNameFunc = func(ctx context.Context, params system.ResolveWorktreeBranchNameParams) (string, error) {
		_ = ctx
		_ = params
		return "", nil
	}

	generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
		_ = ctx
		assert.Equal(t, taskNewFallbackBranchName, params.Branch)
		return "", nil
	}

	resolved, err := resolveTaskCreateParams(context.Background(), taskCreateResolveParams{Prompt: "do this task"})
	require.NoError(t, err)
	assert.Equal(t, taskNewFallbackBranchName, resolved.FeatureBranch)
	assert.Equal(t, taskNewFallbackDescription, resolved.Description)
	assert.Equal(t, "do this task", resolved.Prompt)
}

func TestResolveTaskCreateParamsNoCommitSetsEmptyFeatureBranch(t *testing.T) {
	originalDefaults := resolveTaskRunDefaultsFunc
	originalDescriptionGenerator := generateTaskDescriptionFunc
	t.Cleanup(func() {
		resolveTaskRunDefaultsFunc = originalDefaults
		generateTaskDescriptionFunc = originalDescriptionGenerator
	})

	tests := []struct {
		name       string
		params     taskCreateResolveParams
		parameters conf.RunParameters
	}{
		{
			name:       "cli flag",
			params:     taskCreateResolveParams{Prompt: "do this task", NoCommit: true},
			parameters: conf.RunParameters{Model: "provider/model", Worktree: "feature/%"},
		},
		{
			name:       "run default",
			params:     taskCreateResolveParams{Prompt: "do this task"},
			parameters: conf.RunParameters{Model: "provider/model", Worktree: "feature/%", NoCommit: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunParameters, error) {
				_ = params
				return tt.parameters, nil
			}

			generateTaskDescriptionFunc = func(ctx context.Context, params taskCreateResolveParams) (string, error) {
				_ = ctx
				assert.Equal(t, taskNewFallbackBranchName, params.Branch)
				return "generated description", nil
			}

			resolved, err := resolveTaskCreateParams(context.Background(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, "", resolved.FeatureBranch)
			assert.True(t, resolved.NoCommit)
			assert.Equal(t, taskNewFallbackBranchName, resolved.Name)
			assert.Equal(t, "generated description", resolved.Description)
		})
	}
}

func TestTaskNewCommandPromptFlagIsOptional(t *testing.T) {
	command := taskNewCommand()
	promptFlag := command.Flags().Lookup("prompt")
	require.NotNil(t, promptFlag)
	_, required := promptFlag.Annotations[cobra.BashCompOneRequiredFlag]
	assert.False(t, required)
	assert.Nil(t, command.Flags().Lookup("run"))
}

func TestGenerateTaskDescriptionUsesRetryAndFallbackChain(t *testing.T) {
	originalBuilder := buildTaskDescriptionSystemFunc
	originalModelBuilder := newGenerationChatModelFromSpecFunc
	t.Cleanup(func() {
		buildTaskDescriptionSystemFunc = originalBuilder
		newGenerationChatModelFromSpecFunc = originalModelBuilder
	})

	tests := []struct {
		name      string
		modelSpec string
		setup     func(primary *models.MockClient, backup *models.MockClient)
	}{
		{
			name:      "retries temporary failures for single model",
			modelSpec: "mock/test-model",
			setup: func(primary *models.MockClient, backup *models.MockClient) {
				_ = backup
				networkErrorCount := 1
				primary.NetworkError = &models.NetworkError{Message: "temporary network issue", IsRetryable: true}
				primary.NetworkErrorCount = &networkErrorCount
				primary.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "retry description")})
			},
		},
		{
			name:      "falls back to secondary model when primary fails",
			modelSpec: "mock/test-model,mockb/backup-model",
			setup: func(primary *models.MockClient, backup *models.MockClient) {
				primary.SetChatResponse("test-model", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
				backup.SetChatResponse("backup-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "fallback description")})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
			backup := models.NewMockProvider([]models.ModelInfo{{Name: "backup-model"}})
			tt.setup(primary, backup)

			configStore := &conf.CswConfig{
				GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2},
				AgentConfigFiles: map[string]map[string]string{
					"commit": {
						"system.md":  "system prompt",
						"prompt.md":  "messages:\n{{- range .Messages }}\n- {{ . }}\n{{- end }}",
						"message.md": "[{{ .Branch }}] {{ .Message }}",
					},
				},
			}

			buildTaskDescriptionSystemFunc = func(config *conf.CswConfig) (*system.SweSystem, error) {
				config.GlobalConfig.Parameters.Model = tt.modelSpec
				config.Runtime.Cleanup = func() {}
				return &system.SweSystem{
					ModelProviders: map[string]models.ModelProvider{"mock": primary, "mockb": backup},
					ModelAliases:   map[string][]string{},
					Config:         configStore,
				}, nil
			}

			newGenerationChatModelFromSpecFunc = func(
				modelSpec string,
				providers map[string]models.ModelProvider,
				options *models.ChatOptions,
				config *conf.CswConfig,
				primaryProvider models.ModelProvider,
				aliases map[string][]string,
				retryPolicyOverride *models.RetryPolicy,
				retryLogFn func(string, shared.MessageType),
			) (models.ChatModel, error) {
				_ = config
				_ = primaryProvider
				_ = retryLogFn

				retryPolicy := retryPolicyOverride
				if retryPolicy == nil {
					retryPolicy = &models.RetryPolicy{
						InitialDelay: 0,
						MaxRetries:   1,
						MaxDelay:     time.Millisecond,
					}
				}

				return models.NewChatModelFromProviderChain(modelSpec, providers, options, retryPolicy, nil, aliases)
			}

			description, err := generateTaskDescription(context.Background(), taskCreateResolveParams{
				Prompt:    "improve retry handling",
				Branch:    "feature/retry",
				ModelName: tt.modelSpec,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, description)
			require.NotEmpty(t, primary.RecordedMessages)
			if tt.modelSpec == "mock/test-model,mockb/backup-model" {
				require.NotEmpty(t, backup.RecordedMessages)
			}
		})
	}
}

func TestGenerateTaskDescriptionUsesResolvedModelFromBuildSystem(t *testing.T) {
	originalBuilder := buildTaskDescriptionSystemFunc
	originalModelBuilder := newGenerationChatModelFromSpecFunc
	t.Cleanup(func() {
		buildTaskDescriptionSystemFunc = originalBuilder
		newGenerationChatModelFromSpecFunc = originalModelBuilder
	})

	primary := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	primary.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "generated description")})
	configStore := &conf.CswConfig{
		GlobalConfig: &conf.GlobalConfig{Parameters: conf.RunParameters{Model: "mock/test-model"}},
		AgentConfigFiles: map[string]map[string]string{
			"commit": {
				"system.md":  "system prompt",
				"prompt.md":  "{{ range .Messages }}{{ . }}{{ end }}",
				"message.md": "{{ .Message }}",
			},
		},
	}

	buildTaskDescriptionSystemFunc = func(config *conf.CswConfig) (*system.SweSystem, error) {
		_ = config
		return &system.SweSystem{
			ModelProviders: map[string]models.ModelProvider{"mock": primary},
			ModelAliases:   map[string][]string{},
			Config:         configStore,
		}, nil
	}

	newGenerationChatModelFromSpecFunc = func(
		modelSpec string,
		providers map[string]models.ModelProvider,
		options *models.ChatOptions,
		config *conf.CswConfig,
		primaryProvider models.ModelProvider,
		aliases map[string][]string,
		retryPolicyOverride *models.RetryPolicy,
		retryLogFn func(string, shared.MessageType),
	) (models.ChatModel, error) {
		_ = options
		_ = config
		_ = primaryProvider
		_ = aliases
		_ = retryPolicyOverride
		_ = retryLogFn

		if modelSpec != "mock/test-model" {
			return failingTaskDescriptionChatModel{}, nil
		}

		return providers["mock"].ChatModel("test-model", nil), nil
	}

	description, err := generateTaskDescription(context.Background(), taskCreateResolveParams{
		Prompt:    "improve task description generation",
		Branch:    "feature/task-description",
		ModelName: "mock/default",
	})
	require.NoError(t, err)
	assert.Equal(t, "generated description", description)
}

type failingTaskDescriptionChatModel struct{}

func (failingTaskDescriptionChatModel) Chat(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) (*models.ChatMessage, error) {
	_ = ctx
	_ = messages
	_ = options
	_ = tools
	return nil, fmt.Errorf("test should use resolved model from built system")
}

func (failingTaskDescriptionChatModel) ChatStream(ctx context.Context, messages []*models.ChatMessage, options *models.ChatOptions, tools []tool.ToolInfo) iter.Seq[*models.ChatMessage] {
	_ = ctx
	_ = messages
	_ = options
	_ = tools
	return func(yield func(*models.ChatMessage) bool) {}
}

func (failingTaskDescriptionChatModel) Compactor() models.ChatCompator {
	return nil
}
