package main

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWorktreeCommitMessage(t *testing.T) {
	tests := []struct {
		name           string
		branch         string
		customTemplate string
		llmResponse    string
		expected       string
	}{
		{
			name:        "uses default message template and limits to ten words",
			branch:      "feature/commit-gen",
			llmResponse: "implement commit message generator for worktree sessions and prompts now quickly",
			expected:    "[feature/commit-gen] implement commit message generator for worktree sessions and prompts now",
		},
		{
			name:           "uses custom message template",
			branch:         "feature/custom",
			customTemplate: "branch={{ .Branch }} msg={{ .Message }}",
			llmResponse:    "add custom commit template option",
			expected:       "branch=feature/custom msg=add custom commit template option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweSystem, session, provider := newCommitMessageTestSystem(t, tt.llmResponse)

			err := session.UserPrompt("Implement commit message generator")
			require.NoError(t, err)
			err = session.UserPrompt("Add tests and integrate into worktree commit flow")
			require.NoError(t, err)

			message, err := generateWorktreeCommitMessage(context.Background(), sweSystem, session, tt.branch, tt.customTemplate)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, message)

			recorded := provider.RecordedMessages
			require.NotEmpty(t, recorded)
			require.Len(t, recorded[0], 2)
			assert.Equal(t, models.ChatRoleSystem, recorded[0][0].Role)
			assert.Equal(t, models.ChatRoleUser, recorded[0][1].Role)
			assert.Contains(t, recorded[0][1].GetText(), "Implement commit message generator")
			assert.Contains(t, recorded[0][1].GetText(), "Add tests and integrate into worktree commit flow")
		})
	}
}

func TestGenerateWorktreeCommitMessageErrors(t *testing.T) {
	tests := []struct {
		name           string
		setupSystem    func(t *testing.T) (*core.SweSystem, *core.SweSession)
		expectedErrSub string
	}{
		{
			name: "fails when template file is missing",
			setupSystem: func(t *testing.T) (*core.SweSystem, *core.SweSession) {
				model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
				system := &core.SweSystem{ModelProviders: map[string]models.ModelProvider{"mock": model}, VFS: vfs.NewMockVFS()}
				session, err := system.NewSession("mock/test-model", nil)
				require.NoError(t, err)
				return system, session
			},
			expectedErrSub: commitPromptSystemPath,
		},
		{
			name: "fails when generated message is empty",
			setupSystem: func(t *testing.T) (*core.SweSystem, *core.SweSession) {
				system, session, _ := newCommitMessageTestSystem(t, "   ")
				return system, session
			},
			expectedErrSub: "generated message is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, session := tt.setupSystem(t)
			_, err := generateWorktreeCommitMessage(context.Background(), system, session, "feature/test", "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrSub)
		})
	}
}

func newCommitMessageTestSystem(t *testing.T, llmResponse string) (*core.SweSystem, *core.SweSession, *models.MockClient) {
	t.Helper()

	model := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	model.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmResponse),
	})

	mockVFS := vfs.NewMockVFS()
	require.NoError(t, mockVFS.WriteFile(commitPromptSystemPath, []byte("system prompt")))
	require.NoError(t, mockVFS.WriteFile(commitPromptPromptPath, []byte("messages:\n{{- range .Messages }}\n- {{ . }}\n{{- end }}")))
	require.NoError(t, mockVFS.WriteFile(commitPromptMessagePath, []byte("[{{ .Branch }}] {{ .Message }}")))

	system := &core.SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": model},
		VFS:            mockVFS,
	}

	session, err := system.NewSession("mock/test-model", nil)
	require.NoError(t, err)

	return system, session, model
}
