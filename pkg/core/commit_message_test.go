package core

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCommitMessage(t *testing.T) {
	tests := []struct {
		name              string
		userPrompt        string
		branch            string
		customTemplate    string
		llmResponse       string
		expected          string
		expectedMsgCount  int
		expectedContains  []string
	}{
		{
			name:             "uses default message template and limits to ten words",
			userPrompt:       "Implement commit message generator",
			branch:           "feature/commit-gen",
			llmResponse:      "implement commit message generator for worktree sessions and prompts now quickly",
			expected:         "[feature/commit-gen] implement commit message generator for worktree sessions and prompts now",
			expectedMsgCount: 2,
			expectedContains: []string{"Implement commit message generator"},
		},
		{
			name:              "uses custom message template",
			userPrompt:        "Add tests and integrate into worktree commit flow",
			branch:            "feature/custom",
			customTemplate:    "branch={{ .Branch }} msg={{ .Message }}",
			llmResponse:       "add custom commit template option",
			expected:          "branch=feature/custom msg=add custom commit template option",
			expectedMsgCount:  2,
			expectedContains:  []string{"Add tests and integrate into worktree commit flow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configStore, chatModel, provider := newCommitMessageTestFixture(t, tt.llmResponse)

			message, err := GenerateCommitMessage(context.Background(), chatModel, configStore, tt.userPrompt, tt.branch, tt.customTemplate)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, message)

			recorded := provider.RecordedMessages
			require.NotEmpty(t, recorded)
			require.Len(t, recorded[0], tt.expectedMsgCount)
			assert.Equal(t, models.ChatRoleSystem, recorded[0][0].Role)
			assert.Equal(t, models.ChatRoleUser, recorded[0][1].Role)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, recorded[0][1].GetText(), expected)
			}
		})
	}
}

func TestGenerateCommitMessageErrors(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T) (models.ChatModel, *conf.CswConfig)
		expectedErrSub string
	}{
		{
			name: "fails when template file is missing",
			setup: func(t *testing.T) (models.ChatModel, *conf.CswConfig) {
				model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
				store := &conf.CswConfig{}
				return model.ChatModel("test-model", nil), store
			},
			expectedErrSub: "commit/system.md",
		},
		{
			name: "fails when config store is missing",
			setup: func(t *testing.T) (models.ChatModel, *conf.CswConfig) {
				_, chatModel, _ := newCommitMessageTestFixture(t, "ignored")
				return chatModel, nil
			},
			expectedErrSub: "config store cannot be nil",
		},
		{
			name: "fails when chat model is missing",
			setup: func(t *testing.T) (models.ChatModel, *conf.CswConfig) {
				store := newCommitMessageConfigStore()
				return nil, store
			},
			expectedErrSub: "chat model cannot be nil",
		},
		{
			name: "fails when generated message is empty",
			setup: func(t *testing.T) (models.ChatModel, *conf.CswConfig) {
				store, chatModel, _ := newCommitMessageTestFixture(t, "   ")
				return chatModel, store
			},
			expectedErrSub: "generated message is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatModel, configStore := tt.setup(t)
			_, err := GenerateCommitMessage(context.Background(), chatModel, configStore, "user prompt", "feature/test", "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrSub)
		})
	}
}

func TestGenerateCommitMessageUsesRetryAndFallbackChatModelChain(t *testing.T) {
	tests := []struct {
		name            string
		modelSpec       string
		setupProviders  func(primary *models.MockClient, secondary *models.MockClient)
		expectPrimary   int
		expectSecondary int
	}{
		{
			name:      "retries temporary rate limit errors",
			modelSpec: "p1/test-model",
			setupProviders: func(primary *models.MockClient, secondary *models.MockClient) {
				_ = secondary
				rateLimitCount := 1
				primary.RateLimitError = &models.RateLimitError{Message: "rate exceeded", RetryAfterSeconds: 0}
				primary.RateLimitErrorCount = &rateLimitCount
				primary.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "retry commit description")})
			},
			expectPrimary:   2,
			expectSecondary: 0,
		},
		{
			name:      "falls back to secondary provider",
			modelSpec: "p1/test-model,p2/backup-model",
			setupProviders: func(primary *models.MockClient, secondary *models.MockClient) {
				primary.SetChatResponse("test-model", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
				secondary.SetChatResponse("backup-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "fallback commit description")})
			},
			expectPrimary:   1,
			expectSecondary: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
			secondary := models.NewMockProvider([]models.ModelInfo{{Name: "backup-model"}})
			tt.setupProviders(primary, secondary)

			configStore := newCommitMessageConfigStore()
			configStore.GlobalConfig = &conf.GlobalConfig{LLMRetryMaxAttempts: 2}

			chatModel, err := NewGenerationChatModelFromSpec(
				tt.modelSpec,
				map[string]models.ModelProvider{"p1": primary, "p2": secondary},
				nil,
				configStore,
				primary,
				nil,
				nil,
			)
			require.NoError(t, err)

			message, generationErr := GenerateCommitMessage(context.Background(), chatModel, configStore, "do task", "feature/retry", "")
			require.NoError(t, generationErr)
			assert.NotEmpty(t, message)
			assert.Len(t, primary.RecordedMessages, tt.expectPrimary)
			assert.Len(t, secondary.RecordedMessages, tt.expectSecondary)
		})
	}
}

func newCommitMessageTestFixture(t *testing.T, llmResponse string) (*conf.CswConfig, models.ChatModel, *models.MockClient) {
	t.Helper()

	model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	model.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmResponse),
	})
	chatModel := model.ChatModel("test-model", nil)

	store := newCommitMessageConfigStore()

	return store, chatModel, model
}

func newCommitMessageConfigStore() *conf.CswConfig {
	return &conf.CswConfig{
		AgentConfigFiles: map[string]map[string]string{
			"commit": {
				"system.md":  "system prompt",
				"prompt.md":  "messages:\n{{- range .Messages }}\n- {{ . }}\n{{- end }}",
				"message.md": "[{{ .Branch }}] {{ .Message }}",
			},
		},
	}
}

func TestLimitWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWords int
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			maxWords: 5,
			expected: "",
		},
		{
			name:     "fewer words than limit",
			input:    "one two three",
			maxWords: 5,
			expected: "one two three",
		},
		{
			name:     "exact word count",
			input:    "one two three",
			maxWords: 3,
			expected: "one two three",
		},
		{
			name:     "more words than limit",
			input:    "one two three four five six",
			maxWords: 3,
			expected: "one two three",
		},
		{
			name:     "zero max words",
			input:    "one two three",
			maxWords: 0,
			expected: "",
		},
		{
			name:     "negative max words",
			input:    "one two three",
			maxWords: -1,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LimitWords(tt.input, tt.maxWords)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderCommitPrompt(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        any
		expected    string
		expectError bool
	}{
		{
			name:     "simple template",
			template: "Hello {{ .Message }}",
			data:     struct{ Message string }{Message: "World"},
			expected: "Hello World",
		},
		{
			name:     "template with range",
			template: "{{ range .Items }}{{ . }} {{ end }}",
			data:     struct{ Items []string }{Items: []string{"a", "b", "c"}},
			expected: "a b c ",
		},
		{
			name:        "invalid template",
			template:    "Hello {{ .Missing",
			data:        struct{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderCommitPrompt(tt.template, tt.data)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
