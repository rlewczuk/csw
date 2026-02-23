package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCliWorktreeAndCommitMessageFlagsDefinition(t *testing.T) {
	cmd := CliCommand()

	worktreeFlag := cmd.Flags().Lookup("worktree")
	require.NotNil(t, worktreeFlag)
	assert.Equal(t, "", worktreeFlag.DefValue)

	commitMessageFlag := cmd.Flags().Lookup("commit-message")
	require.NotNil(t, commitMessageFlag)
	assert.Equal(t, "", commitMessageFlag.DefValue)
	assert.Equal(t, "string", commitMessageFlag.Value.Type())
}

func TestFinalizeWorktreeSession(t *testing.T) {
	tests := []struct {
		name                 string
		worktreeBranch       string
		customTemplate       string
		llmMessage           string
		removeSystemTemplate bool
		expectCommit         bool
		expectedMessage      string
		expectStderr         string
	}{
		{
			name:            "commits generated message and drops worktree",
			worktreeBranch:  "feature/default",
			llmMessage:      "implement commit generator using llm and prompts",
			expectCommit:    true,
			expectedMessage: "[feature/default] implement commit generator using llm and prompts",
		},
		{
			name:            "uses custom commit template",
			worktreeBranch:  "feature/custom",
			customTemplate:  "branch={{ .Branch }} | {{ .Message }}",
			llmMessage:      "add custom template option",
			expectCommit:    true,
			expectedMessage: "branch=feature/custom | add custom template option",
		},
		{
			name:                 "generation error skips commit and logs error",
			worktreeBranch:       "feature/error",
			llmMessage:           "irrelevant",
			removeSystemTemplate: true,
			expectCommit:         false,
			expectStderr:         "worktree commit message generation failed",
		},
		{
			name:           "no branch skips finalization",
			worktreeBranch: "",
			llmMessage:     "ignored",
			expectCommit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, session, mockVCS := newFinalizeWorktreeFixture(t, tt.llmMessage)
			if tt.removeSystemTemplate {
				require.NoError(t, system.VFS.DeleteFile(CommitPromptSystemPath, false, true))
			}

			var stderr bytes.Buffer
			finalizeWorktreeSession(context.Background(), mockVCS, tt.worktreeBranch, tt.customTemplate, system, session, &stderr)

			commitCalls := mockVCS.GetCommitCalls()
			if tt.expectCommit {
				require.Len(t, commitCalls, 1)
				assert.Equal(t, tt.worktreeBranch, commitCalls[0].Branch)
				assert.Equal(t, tt.expectedMessage, commitCalls[0].Message)
			} else {
				assert.Empty(t, commitCalls)
			}

			dropCalls := mockVCS.GetDropCalls()
			if tt.worktreeBranch == "" {
				assert.Empty(t, dropCalls)
			} else {
				require.Len(t, dropCalls, 1)
				assert.Equal(t, tt.worktreeBranch, dropCalls[0])
			}

			if tt.expectStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectStderr)
			}
		})
	}
}

func newFinalizeWorktreeFixture(t *testing.T, llmMessage string) (*core.SweSystem, *core.SweSession, *vfs.MockVCS) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmMessage),
	})

	mockVFS := vfs.NewMockVFS()
	require.NoError(t, mockVFS.WriteFile(CommitPromptSystemPath, []byte("system prompt")))
	require.NoError(t, mockVFS.WriteFile(CommitPromptPromptPath, []byte("{{- range .Messages }}{{ . }}\n{{- end }}")))
	require.NoError(t, mockVFS.WriteFile(CommitPromptMessagePath, []byte("[{{ .Branch }}] {{ .Message }}")))

	system := &core.SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		VFS:            mockVFS,
	}

	session, err := system.NewSession("mock/test-model", nil)
	require.NoError(t, err)
	require.NoError(t, session.UserPrompt("Implement commit message workflow"))

	mockVCS := vfs.NewMockVCS(vfs.NewMockVFS())
	return system, session, mockVCS
}
