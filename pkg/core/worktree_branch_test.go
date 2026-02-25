package core

import (
	"context"
	"testing"

	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
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
			expectedBranch:   "very-long-symbolic-b",
			expectedContains: "Fix worktree cleanup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, provider := newWorktreeBranchTestSystem(t, tt.llmResponse)

			branch, err := GenerateWorktreeBranchName(context.Background(), system, "mock/test-model", tt.inputPrompt)
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
				system, _ := newWorktreeBranchTestSystem(t, "valid-name")
				system.ConfigStore = nil
				return system
			},
			modelName:      "mock/test-model",
			expectedErrSub: "config store cannot be nil",
		},
		{
			name: "fails when generated name is empty after normalization",
			setupSystem: func(t *testing.T) *SweSystem {
				system, _ := newWorktreeBranchTestSystem(t, "!!!")
				return system
			},
			modelName:      "mock/test-model",
			expectedErrSub: "generated branch name is empty",
		},
		{
			name: "fails on invalid model format",
			setupSystem: func(t *testing.T) *SweSystem {
				system, _ := newWorktreeBranchTestSystem(t, "valid-name")
				return system
			},
			modelName:      "test-model",
			expectedErrSub: "invalid model format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := tt.setupSystem(t)
			_, err := GenerateWorktreeBranchName(context.Background(), system, tt.modelName, "Fix cleanup issue")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrSub)
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
		{name: "trims to twenty chars", input: "this name is definitely too long", expected: "this-name-is-definit"},
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

	store := confimpl.NewMockConfigStore()
	store.SetAgentConfigFile("worktree", "system.md", []byte("system worktree prompt"))
	store.SetAgentConfigFile("worktree", "message.md", []byte("input:\n{{ .Input }}"))

	system := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		ConfigStore:    store,
	}

	return system, provider
}
