package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
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

	mergeFlag := cmd.Flags().Lookup("merge")
	require.NotNil(t, mergeFlag)
	assert.Equal(t, "false", mergeFlag.DefValue)
	assert.Equal(t, "bool", mergeFlag.Value.Type())
}

func TestFinalizeWorktreeSession(t *testing.T) {
	tests := []struct {
		name               string
		worktreeBranch     string
		merge              bool
		customTemplate     string
		llmMessage         string
		omitSystemTemplate bool
		mergeErr           error
		expectCommit       bool
		expectMerge        bool
		expectDeleteBranch bool
		expectDropWorktree bool
		expectedMessage    string
		expectStderr       string
	}{
		{
			name:               "commits generated message and drops worktree",
			worktreeBranch:     "feature/default",
			llmMessage:         "implement commit generator using llm and prompts",
			expectCommit:       true,
			expectDropWorktree: true,
			expectedMessage:    "[feature/default] implement commit generator using llm and prompts",
		},
		{
			name:               "merges branch and cleans up",
			worktreeBranch:     "feature/merge",
			merge:              true,
			llmMessage:         "add automatic merge workflow",
			expectCommit:       true,
			expectMerge:        true,
			expectDeleteBranch: true,
			expectDropWorktree: true,
			expectedMessage:    "[feature/merge] add automatic merge workflow",
		},
		{
			name:               "merge conflict keeps worktree and branch",
			worktreeBranch:     "feature/conflict",
			merge:              true,
			llmMessage:         "trigger conflict",
			mergeErr:           vfs.ErrMergeConflict,
			expectCommit:       true,
			expectMerge:        true,
			expectDeleteBranch: false,
			expectDropWorktree: false,
			expectedMessage:    "[feature/conflict] trigger conflict",
			expectStderr:       "automatic merge failed due to conflicts",
		},
		{
			name:               "uses custom commit template",
			worktreeBranch:     "feature/custom",
			customTemplate:     "branch={{ .Branch }} | {{ .Message }}",
			llmMessage:         "add custom template option",
			expectCommit:       true,
			expectDropWorktree: true,
			expectedMessage:    "branch=feature/custom | add custom template option",
		},
		{
			name:               "generation error skips commit and logs error",
			worktreeBranch:     "feature/error",
			llmMessage:         "irrelevant",
			omitSystemTemplate: true,
			expectCommit:       false,
			expectDropWorktree: true,
			expectStderr:       "worktree commit message generation failed",
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
			system, session, mockVCS := newFinalizeWorktreeFixture(t, tt.llmMessage, !tt.omitSystemTemplate)
			if tt.mergeErr != nil {
				mockVCS.SetMergeError(tt.mergeErr)
			}

			var stderr bytes.Buffer
			_ = finalizeWorktreeSession(context.Background(), mockVCS, tt.worktreeBranch, tt.merge, tt.customTemplate, system, session, &stderr, "", "", "")

			commitCalls := mockVCS.GetCommitCalls()
			if tt.expectCommit {
				require.Len(t, commitCalls, 1)
				assert.Equal(t, tt.worktreeBranch, commitCalls[0].Branch)
				assert.Equal(t, tt.expectedMessage, commitCalls[0].Message)
			} else {
				assert.Empty(t, commitCalls)
			}

			dropCalls := mockVCS.GetDropCalls()
			if !tt.expectDropWorktree {
				assert.Empty(t, dropCalls)
			} else {
				require.Len(t, dropCalls, 1)
				assert.Equal(t, tt.worktreeBranch, dropCalls[0])
			}

			mergeCalls := mockVCS.GetMergeCalls()
			if !tt.expectMerge {
				assert.Empty(t, mergeCalls)
			} else {
				require.Len(t, mergeCalls, 1)
				assert.Equal(t, "main", mergeCalls[0].Into)
				assert.Equal(t, tt.worktreeBranch, mergeCalls[0].From)
			}

			deleteCalls := mockVCS.GetDeleteCalls()
			if !tt.expectDeleteBranch {
				assert.Empty(t, deleteCalls)
			} else {
				require.Len(t, deleteCalls, 1)
				assert.Equal(t, tt.worktreeBranch, deleteCalls[0])
			}

			if tt.expectStderr != "" {
				assert.Contains(t, stderr.String(), tt.expectStderr)
			}
		})
	}
}

func newFinalizeWorktreeFixture(t *testing.T, llmMessage string, includeSystemTemplate bool) (*system.SweSystem, *core.SweSession, *vfs.MockVCS) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmMessage),
	})

	configStore := confimpl.NewMockConfigStore()
	if includeSystemTemplate {
		configStore.SetAgentConfigFile("commit", "system.md", []byte("system prompt"))
	}
	configStore.SetAgentConfigFile("commit", "prompt.md", []byte("{{- range .Messages }}{{ . }}\n{{- end }}"))
	configStore.SetAgentConfigFile("commit", "message.md", []byte("[{{ .Branch }}] {{ .Message }}"))

	system := &system.SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		ConfigStore:    configStore,
	}

	session, err := system.NewSession("mock/test-model", nil)
	require.NoError(t, err)
	require.NoError(t, session.UserPrompt("Implement commit message workflow"))

	mockVCS := vfs.NewMockVCS(vfs.NewMockVFS())
	return system, session, mockVCS
}

func TestMergeWorktreeWithConflictResolution(t *testing.T) {
	t.Run("resolves conflict via sub-session and retries rebase", func(t *testing.T) {
		sweSystem, session := newConflictResolutionFixture(t)
		repoDir := t.TempDir()
		worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "test")
		originalRunGit := runGitCommandFunc
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := listGitConflictFilesFunc
		defer func() {
			runGitCommandFunc = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			listGitConflictFilesFunc = originalListConflicts
		}()

		commands := make([]string, 0)
		rebaseCalls := 0
		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
			joined := fmt.Sprintf("%s::git %s", workDir, strings.Join(args, " "))
			commands = append(commands, joined)

			switch {
			case strings.HasSuffix(joined, "::git rebase main"):
				rebaseCalls++
				if rebaseCalls == 1 {
					return "", fmt.Errorf("runGitCommand() [cli.go]: git rebase main failed: CONFLICT (content): Merge conflict in pkg/core/session.go")
				}
				return "", nil
			case strings.Contains(joined, "::git worktree add --force"):
				return "", nil
			case strings.HasSuffix(joined, "::git merge --ff-only feature/test"):
				return "", nil
			case strings.HasSuffix(joined, "::git rev-parse HEAD"):
				return "abc123", nil
			case strings.Contains(joined, "::git worktree remove --force"):
				return "", nil
			default:
				return "", nil
			}
		}

		listGitConflictFilesFunc = func(workDir string) []string {
			return []string{"pkg/core/session.go"}
		}

		subAgentCalls := 0
		executeConflictSubAgentFunc = func(parent *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
			subAgentCalls++
			assert.Equal(t, session, parent)
			assert.Equal(t, "developer", request.Role)
			assert.Contains(t, request.Prompt, "conflict resolution session")
			assert.Contains(t, request.Prompt, "parent task prompt")
			return tool.SubAgentTaskResult{Status: "completed", Summary: "resolved"}, nil
		}

		var stderr bytes.Buffer
		headCommitID, err := mergeWorktreeWithConflictResolution(context.Background(), repoDir, worktreeDir, "feature/test", "parent task prompt", sweSystem, session, &stderr)
		require.NoError(t, err)
		assert.Equal(t, "abc123", headCommitID)
		assert.Equal(t, 1, subAgentCalls)
		assert.NotEmpty(t, commands)
		assert.Contains(t, stderr.String(), "conflict detected")
	})

	t.Run("fails when sub-session reports error status", func(t *testing.T) {
		sweSystem, session := newConflictResolutionFixture(t)
		repoDir := t.TempDir()
		worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "test")
		originalRunGit := runGitCommandFunc
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := listGitConflictFilesFunc
		defer func() {
			runGitCommandFunc = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			listGitConflictFilesFunc = originalListConflicts
		}()

		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
			if len(args) == 2 && args[0] == "rebase" && args[1] == "main" {
				return "", fmt.Errorf("runGitCommand() [cli.go]: git rebase main failed: CONFLICT (content): Merge conflict in file.txt")
			}
			return "", nil
		}

		listGitConflictFilesFunc = func(workDir string) []string { return []string{"file.txt"} }
		executeConflictSubAgentFunc = func(parent *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
			return tool.SubAgentTaskResult{Status: "error", Summary: "failed"}, nil
		}

		_, err := mergeWorktreeWithConflictResolution(context.Background(), repoDir, worktreeDir, "feature/test", "prompt", sweSystem, session, &bytes.Buffer{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ended with status")
	})
}

func newConflictResolutionFixture(t *testing.T) (*system.SweSystem, *core.SweSession) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	configStore := confimpl.NewMockConfigStore()
	configStore.SetAgentConfigFile("conflict", "prompt.md", []byte("You are running in a conflict resolution session.\n{{ .OriginalPrompt }}\n{{ .ConflictFiles }}\n{{ .ConflictOutput }}"))

	sweSystem := &system.SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		ConfigStore:    configStore,
	}

	session, err := sweSystem.NewSession("mock/test-model", nil)
	require.NoError(t, err)

	return sweSystem, session
}
