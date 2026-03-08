package core

import (
	"context"
	"testing"

	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCommitMessage(t *testing.T) {
	tests := []struct {
		name              string
		branch            string
		customTemplate    string
		llmResponse       string
		expected          string
		expectedMsgCount  int
		expectedContains  []string
	}{
		{
			name:             "uses default message template and limits to ten words",
			branch:           "feature/commit-gen",
			llmResponse:      "implement commit message generator for worktree sessions and prompts now quickly",
			expected:         "[feature/commit-gen] implement commit message generator for worktree sessions and prompts now",
			expectedMsgCount: 2,
			expectedContains: []string{"Implement commit message generator", "Add tests and integrate into worktree commit flow"},
		},
		{
			name:              "uses custom message template",
			branch:            "feature/custom",
			customTemplate:    "branch={{ .Branch }} msg={{ .Message }}",
			llmResponse:       "add custom commit template option",
			expected:          "branch=feature/custom msg=add custom commit template option",
			expectedMsgCount:  2,
			expectedContains:  []string{"Implement commit message generator", "Add tests and integrate into worktree commit flow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweSystem, session, provider := newCommitMessageTestSystem(t, tt.llmResponse)

			err := session.UserPrompt("Implement commit message generator")
			require.NoError(t, err)
			err = session.UserPrompt("Add tests and integrate into worktree commit flow")
			require.NoError(t, err)

			message, err := GenerateCommitMessage(context.Background(), sweSystem.ModelProviders, sweSystem.ConfigStore, session, tt.branch, tt.customTemplate)
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
		setupSystem    func(t *testing.T) (*SweSystem, *SweSession)
		expectedErrSub string
	}{
		{
			name: "fails when template file is missing",
			setupSystem: func(t *testing.T) (*SweSystem, *SweSession) {
				model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
				store := confimpl.NewMockConfigStore()
				system := &SweSystem{ModelProviders: map[string]models.ModelProvider{"mock": model}, ConfigStore: store}
				session, err := system.NewSession("mock/test-model", nil)
				require.NoError(t, err)
				return system, session
			},
			expectedErrSub: "commit/system.md",
		},
		{
			name: "fails when config store is missing",
			setupSystem: func(t *testing.T) (*SweSystem, *SweSession) {
				system, session, _ := newCommitMessageTestSystem(t, "ignored")
				system.ConfigStore = nil
				return system, session
			},
			expectedErrSub: "config store cannot be nil",
		},
		{
			name: "fails when generated message is empty",
			setupSystem: func(t *testing.T) (*SweSystem, *SweSession) {
				system, session, _ := newCommitMessageTestSystem(t, "   ")
				return system, session
			},
			expectedErrSub: "generated message is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, session := tt.setupSystem(t)
			_, err := GenerateCommitMessage(context.Background(), system.ModelProviders, system.ConfigStore, session, "feature/test", "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrSub)
		})
	}
}

func newCommitMessageTestSystem(t *testing.T, llmResponse string) (*SweSystem, *SweSession, *models.MockClient) {
	t.Helper()

	model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	model.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmResponse),
	})

	store := confimpl.NewMockConfigStore()
	store.SetAgentConfigFile("commit", "system.md", []byte("system prompt"))
	store.SetAgentConfigFile("commit", "prompt.md", []byte("messages:\n{{- range .Messages }}\n- {{ . }}\n{{- end }}"))
	store.SetAgentConfigFile("commit", "message.md", []byte("[{{ .Branch }}] {{ .Message }}"))

	system := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": model},
		ConfigStore:    store,
	}

	session, err := system.NewSession("mock/test-model", nil)
	require.NoError(t, err)

	return system, session, model
}

func TestCollectUserMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []*models.ChatMessage
		expected []string
	}{
		{
			name:     "empty messages",
			messages: []*models.ChatMessage{},
			expected: []string{},
		},
		{
			name: "only user messages collected",
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleUser, "user message"),
				models.NewTextMessage(models.ChatRoleAssistant, "assistant message"),
				models.NewTextMessage(models.ChatRoleUser, "another user message"),
			},
			expected: []string{"user message", "another user message"},
		},
		{
			name: "nil messages skipped",
			messages: []*models.ChatMessage{
				nil,
				models.NewTextMessage(models.ChatRoleUser, "valid message"),
			},
			expected: []string{"valid message"},
		},
		{
			name: "empty text skipped",
			messages: []*models.ChatMessage{
				models.NewTextMessage(models.ChatRoleUser, "   "),
				models.NewTextMessage(models.ChatRoleUser, "valid"),
			},
			expected: []string{"valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CollectUserMessages(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
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
