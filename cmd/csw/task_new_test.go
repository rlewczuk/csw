package main

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/system"
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

	resolveTaskRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
		_ = params
		return conf.RunDefaultsConfig{Model: "provider/model", Worktree: "feature/%"}, nil
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

			buildTaskDescriptionSystemFunc = func(params system.BuildSystemParams) (*system.SweSystem, system.BuildSystemResult, error) {
				_ = params
				return &system.SweSystem{
					ModelProviders: map[string]models.ModelProvider{"mock": primary, "mockb": backup},
					ModelAliases:   map[string][]string{},
					Config:         configStore,
				}, system.BuildSystemResult{ModelName: tt.modelSpec, Cleanup: func() {}}, nil
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
