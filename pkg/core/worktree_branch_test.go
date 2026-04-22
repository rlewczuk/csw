package core

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWorktreeBranchName(t *testing.T) {
	tests := []struct {
		name             string
		llmResponse      string
		inputPrompt      string
		expectedBranch   string
		expectedContains string
	}{
		{
			name:             "normalizes generated symbolic branch name",
			llmResponse:      "Linear Merge Cleanup!",
			inputPrompt:      "Modify merge process to keep history linear",
			expectedBranch:   "linear-merge-cleanup",
			expectedContains: "Modify merge process",
		},
		{
			name:             "limits generated branch suffix to twenty chars",
			llmResponse:      "very long symbolic branch name for worktree cleanup",
			inputPrompt:      "Fix worktree cleanup",
			expectedBranch:   "very-long-symbolic-branch",
			expectedContains: "Fix worktree cleanup",
		},
		{
			name:             "allows overflow up to twenty five chars to keep nearest word",
			llmResponse:      "kebab case configuration keys",
			inputPrompt:      "Normalize branch names",
			expectedBranch:   "kebab-case-configuration",
			expectedContains: "Normalize branch names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweSystem, provider := newWorktreeBranchTestSystem(t, tt.llmResponse)

			branch, err := GenerateWorktreeBranchName(context.Background(), sweSystem.ModelProviders, sweSystem.Config, "mock/test-model", tt.inputPrompt)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBranch, branch)

			recorded := provider.RecordedMessages
			require.NotEmpty(t, recorded)
			require.Len(t, recorded[0], 2)
			assert.Equal(t, models.ChatRoleSystem, recorded[0][0].Role)
			assert.Equal(t, models.ChatRoleUser, recorded[0][1].Role)
			assert.Contains(t, recorded[0][1].GetText(), tt.expectedContains)
		})
	}
}

func TestGenerateWorktreeBranchNameErrors(t *testing.T) {
	tests := []struct {
		name           string
		setupSystem    func(t *testing.T) *SweSystem
		modelName      string
		expectedErrSub string
	}{
		{
			name: "fails when config store missing",
			setupSystem: func(t *testing.T) *SweSystem {
				sweSystem, _ := newWorktreeBranchTestSystem(t, "valid-name")
				sweSystem.Config = nil
				return sweSystem
			},
			modelName:      "mock/test-model",
			expectedErrSub: "config store cannot be nil",
		},
		{
			name: "fails when generated name is empty after normalization",
			setupSystem: func(t *testing.T) *SweSystem {
				sweSystem, _ := newWorktreeBranchTestSystem(t, "!!!")
				return sweSystem
			},
			modelName:      "mock/test-model",
			expectedErrSub: "generated branch name is empty",
		},
		{
			name: "fails on unknown model alias",
			setupSystem: func(t *testing.T) *SweSystem {
				sweSystem, _ := newWorktreeBranchTestSystem(t, "valid-name")
				return sweSystem
			},
			modelName:      "unknown-alias",
			expectedErrSub: "invalid model format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweSystem := tt.setupSystem(t)
			_, err := GenerateWorktreeBranchName(context.Background(), sweSystem.ModelProviders, sweSystem.Config, tt.modelName, "Fix cleanup issue")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrSub)
		})
	}
}

func TestGenerateWorktreeBranchNameAliasModel(t *testing.T) {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}, {Name: "backup-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, "alias-branch-name"),
	})

	store := &conf.CswConfig{
		AgentConfigFiles: map[string]map[string]string{
			"worktree": {
				"system.md":  "system worktree prompt",
				"message.md": "input:\n{{ .Input }}",
			},
		},
		ModelAliases: map[string]conf.ModelAliasValue{
			"default": {Values: []string{"mock/test-model", "mock/backup-model"}},
		},
	}

	branch, err := GenerateWorktreeBranchName(
		context.Background(),
		map[string]models.ModelProvider{"mock": provider},
		store,
		"default",
		"Fix branch naming",
	)
	require.NoError(t, err)
	assert.Equal(t, "alias-branch-name", branch)
	require.NotEmpty(t, provider.RecordedMessages)
}

func TestGenerateWorktreeBranchNameUsesRetryAndFallbackChatModelChain(t *testing.T) {
	tests := []struct {
		name            string
		modelSpec       string
		setupProviders  func(primary *models.MockClient, secondary *models.MockClient)
		retryPolicy     *models.RetryPolicy
		expectPrimary   int
		expectSecondary int
		expectedBranch  string
		maxDuration     time.Duration
	}{
		{
			name:      "retries temporary rate limit errors",
			modelSpec: "p1/test-model",
			setupProviders: func(primary *models.MockClient, secondary *models.MockClient) {
				_ = secondary
				rateLimitCount := 1
				primary.RateLimitError = &models.RateLimitError{Message: "rate exceeded", RetryAfterSeconds: 0}
				primary.RateLimitErrorCount = &rateLimitCount
				primary.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "retry branch output")})
			},
			retryPolicy:     &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond},
			expectPrimary:   2,
			expectSecondary: 0,
			expectedBranch:  "retry-branch-output",
			maxDuration:     50 * time.Millisecond,
		},
		{
			name:      "falls back to secondary provider",
			modelSpec: "p1/test-model,p2/backup-model",
			setupProviders: func(primary *models.MockClient, secondary *models.MockClient) {
				primary.SetChatResponse("test-model", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
				secondary.SetChatResponse("backup-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "fallback branch output")})
			},
			retryPolicy:     &models.RetryPolicy{InitialDelay: time.Millisecond, MaxRetries: 1, MaxDelay: time.Millisecond},
			expectPrimary:   1,
			expectSecondary: 1,
			expectedBranch:  "fallback-branch-output",
			maxDuration:     50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
			secondary := models.NewMockProvider([]models.ModelInfo{{Name: "backup-model"}})
			tt.setupProviders(primary, secondary)

			store := &conf.CswConfig{
				GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2},
				AgentConfigFiles: map[string]map[string]string{
					"worktree": {
						"system.md":  "system worktree prompt",
						"message.md": "input:\n{{ .Input }}",
					},
				},
			}

			start := time.Now()

			branch, err := generateWorktreeBranchNameWithRetryPolicyOverride(
				context.Background(),
				map[string]models.ModelProvider{"p1": primary, "p2": secondary},
				store,
				tt.modelSpec,
				"Fix branch naming",
				tt.retryPolicy,
			)
			elapsed := time.Since(start)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBranch, branch)
			assert.Len(t, primary.RecordedMessages, tt.expectPrimary)
			assert.Len(t, secondary.RecordedMessages, tt.expectSecondary)
			assert.Less(t, elapsed, tt.maxDuration)
		})
	}
}

func TestRenderWorktreeBranchPrompt(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        any
		expected    string
		expectError bool
	}{
		{
			name:     "simple template",
			template: "Task: {{ .Input }}",
			data:     WorktreeBranchPromptData{Input: "Fix worktree cleanup"},
			expected: "Task: Fix worktree cleanup",
		},
		{
			name:        "invalid template",
			template:    "Task: {{ .Input",
			data:        WorktreeBranchPromptData{Input: "Fix"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderWorktreeBranchPrompt(tt.template, tt.data)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeWorktreeBranchSymbolicName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "keeps lowercase and dashes", input: "linear-merge", expected: "linear-merge"},
		{name: "normalizes spaces and symbols", input: "Linear Merge Cleanup!!!", expected: "linear-merge-cleanup"},
		{name: "collapses repeated separators", input: "worktree___cleanup---fix", expected: "worktree-cleanup-fix"},
		{name: "trims to nearest word around twenty chars", input: "this name is definitely too long", expected: "this-name-is-definitely"},
		{name: "allows overflow up to twenty five chars", input: "kebab-case-configuration-keys", expected: "kebab-case-configuration"},
		{name: "falls back to twenty five chars when first word too long", input: "supercalifragilisticexpialidocious", expected: "supercalifragilisticexpia"},
		{name: "empty when no alphanumeric characters", input: "!!!", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeWorktreeBranchSymbolicName(tt.input))
		})
	}
}

func newWorktreeBranchTestSystem(t *testing.T, llmResponse string) (*SweSystem, *models.MockClient) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmResponse),
	})

	store := &conf.CswConfig{
		AgentConfigFiles: map[string]map[string]string{
			"worktree": {
				"system.md":  "system worktree prompt",
				"message.md": "input:\n{{ .Input }}",
			},
		},
	}

	sweSystem := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		Config:         store,
	}

	return sweSystem, provider
}
