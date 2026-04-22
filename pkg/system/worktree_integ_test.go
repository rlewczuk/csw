package system

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/rlewczuk/csw/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalizeWorktreeSession(t *testing.T) {
	tests := []struct {
		name               string
		worktreeBranch     string
		merge              bool
		customTemplate     string
		llmMessage         string
		omitSystemTemplate bool
		commitErr          error
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
			mergeErr:           apis.ErrMergeConflict,
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
			name:               "generation error preserves worktree",
			worktreeBranch:     "feature/error",
			llmMessage:         "irrelevant",
			omitSystemTemplate: true,
			expectCommit:       false,
			expectDropWorktree: false,
			expectStderr:       "worktree commit message generation failed",
		},
		{
			name:           "no branch skips finalization",
			worktreeBranch: "",
			llmMessage:     "ignored",
			expectCommit:   false,
		},
		{
			name:               "commit failure with merge=false preserves worktree",
			worktreeBranch:     "feature/commit-fail",
			llmMessage:         "some message",
			commitErr:          fmt.Errorf("simulated commit error"),
			expectCommit:       true,
			expectDropWorktree: false,
			expectedMessage:    "[feature/commit-fail] some message",
			expectStderr:       "worktree commit failed",
		},
		{
			name:               "commit failure with merge=true preserves worktree and skips merge",
			worktreeBranch:     "feature/commit-fail-merge",
			merge:              true,
			llmMessage:         "some message",
			commitErr:          fmt.Errorf("simulated commit error"),
			expectCommit:       true,
			expectDropWorktree: false,
			expectedMessage:    "[feature/commit-fail-merge] some message",
			expectStderr:       "merge skipped because commit failed",
		},
		{
			name:               "non-conflict merge error preserves worktree and branch",
			worktreeBranch:     "feature/merge-fail",
			merge:              true,
			llmMessage:         "merge will fail",
			mergeErr:           fmt.Errorf("simulated merge error"),
			expectCommit:       true,
			expectMerge:        true,
			expectDeleteBranch: false,
			expectDropWorktree: false,
			expectedMessage:    "[feature/merge-fail] merge will fail",
			expectStderr:       "automatic merge failed",
		},
		{
			name:               "commit no changes error is tolerated and drops worktree",
			worktreeBranch:     "feature/no-changes",
			llmMessage:         "no changes msg",
			commitErr:          apis.ErrNoChangesToCommit,
			expectCommit:       true,
			expectDropWorktree: true,
			expectedMessage:    "[feature/no-changes] no changes msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, session, mockVCS := newFinalizeWorktreeFixture(t, tt.llmMessage, !tt.omitSystemTemplate)
			if tt.commitErr != nil {
				mockVCS.SetCommitError(tt.commitErr)
			}
			if tt.mergeErr != nil {
				mockVCS.SetMergeError(tt.mergeErr)
			}

			var stderr bytes.Buffer
			_, _ = FinalizeWorktreeSession(context.Background(), mockVCS, tt.worktreeBranch, tt.merge, tt.customTemplate, system, session, &stderr, "", "", "")

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

func TestFinalizeWorktreeSessionUsesDetectedBaseBranch(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "merge into detected base branch", true)

	originalRunGit := vcs.RunGitCommand
	defer func() {
		vcs.RunGitCommand = originalRunGit
	}()

	vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
		if len(args) == 3 && args[0] == "rev-parse" && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
			return "develop", nil
		}
		return "", nil
	}

	_, _ = FinalizeWorktreeSession(context.Background(), mockVCS, "feature/detect-base", true, "", sweSystem, session, &bytes.Buffer{}, "/repo", "", "")

	mergeCalls := mockVCS.GetMergeCalls()
	require.Len(t, mergeCalls, 1)
	assert.Equal(t, "develop", mergeCalls[0].Into)
	assert.Equal(t, "feature/detect-base", mergeCalls[0].From)
}

func TestFinalizeWorktreeSessionMergeStashesAndRestoresLocalChanges(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "merge with local changes", true)
	repoDir := t.TempDir()

	originalRunGit := vcs.RunGitCommand
	defer func() {
		vcs.RunGitCommand = originalRunGit
	}()

	commands := make([]string, 0)
	vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		commands = append(commands, fmt.Sprintf("%s::%s", workDir, command))
		switch command {
		case "rev-parse --abbrev-ref HEAD":
			return "main", nil
		case "status --porcelain":
			return " M README.md", nil
		case "stash push --include-untracked -m csw: automatic stash before merge":
			return "Saved working directory and index state", nil
		case "stash apply --index stash@{0}":
			return "", nil
		case "stash drop stash@{0}":
			return "Dropped stash@{0}", nil
		default:
			return "", nil
		}
	}

	var stderr bytes.Buffer
	_, err := FinalizeWorktreeSession(context.Background(), mockVCS, "feature/stash-ok", true, "", sweSystem, session, &stderr, repoDir, "", "")
	require.NoError(t, err)

	assert.Contains(t, commands, fmt.Sprintf("%s::status --porcelain", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::stash push --include-untracked -m csw: automatic stash before merge", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::stash apply --index stash@{0}", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::stash drop stash@{0}", repoDir))
	assert.NotContains(t, stderr.String(), "failed to restore stashed local changes")
}

func TestFinalizeWorktreeSessionRetriesCommitMessageGenerationOnTemporaryError(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "retry generated message", true)
	originalNewGenerationChatModelForWorktreeFunc := newGenerationChatModelForWorktreeFunc
	defer func() {
		newGenerationChatModelForWorktreeFunc = originalNewGenerationChatModelForWorktreeFunc
	}()

	newGenerationChatModelForWorktreeFunc = func(
		modelSpec string,
		providers map[string]models.ModelProvider,
		options *models.ChatOptions,
		config *conf.CswConfig,
		primaryProvider models.ModelProvider,
		aliases map[string][]string,
		retryPolicyOverride *models.RetryPolicy,
		retryLogFn func(string, shared.MessageType),
	) (models.ChatModel, error) {
		fastRetryPolicy := &models.RetryPolicy{
			InitialDelay: time.Millisecond,
			MaxRetries:   1,
			MaxDelay:     time.Millisecond,
		}
		return core.NewGenerationChatModelFromSpec(modelSpec, providers, options, config, primaryProvider, aliases, fastRetryPolicy, retryLogFn)
	}

	provider, ok := sweSystem.ModelProviders["mock"].(*models.MockClient)
	require.True(t, ok)
	rateLimitCount := 1
	provider.RateLimitError = &models.RateLimitError{Message: "rate exceeded", RetryAfterSeconds: 0}
	provider.RateLimitErrorCount = &rateLimitCount
	provider.SetChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "retry generated message")})
	if sweSystem.Config == nil {
		sweSystem.Config = &conf.CswConfig{}
	}
	sweSystem.Config.GlobalConfig = &conf.GlobalConfig{LLMRetryMaxAttempts: 2}

	var stderr bytes.Buffer
	_, err := FinalizeWorktreeSession(context.Background(), mockVCS, "feature/retry-finalize", false, "", sweSystem, session, &stderr, "", "", "")
	require.NoError(t, err)

	commitCalls := mockVCS.GetCommitCalls()
	require.Len(t, commitCalls, 1)
	assert.Equal(t, "[feature/retry-finalize] retry generated message", commitCalls[0].Message)
	assert.Contains(t, stderr.String(), "worktree commit message generation retry")
	require.GreaterOrEqual(t, len(provider.RecordedMessages), 2)
}

func TestFinalizeWorktreeSessionUsesFallbackModelForCommitMessageGeneration(t *testing.T) {
	primary := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	secondary := models.NewMockProvider([]models.ModelInfo{{Name: "backup-model"}})
	primary.SetChatResponse("test-model", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
	secondary.SetChatResponse("backup-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "fallback generated message")})

	sweSystem := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"p1": primary, "p2": secondary},
		ModelAliases:   map[string][]string{},
		Config: &conf.CswConfig{
			GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 1},
			AgentConfigFiles: map[string]map[string]string{
				"commit": {
					"system.md":  "system prompt",
					"prompt.md":  "{{- range .Messages }}{{ . }}\n{{- end }}",
					"message.md": "[{{ .Branch }}] {{ .Message }}",
				},
			},
		},
	}

	session, err := sweSystem.NewSession("p1/test-model,p2/backup-model", nil)
	require.NoError(t, err)
	mockVCS := vfs.NewMockVCS(vfs.NewMockVFS())

	var stderr bytes.Buffer
	_, finalizeErr := FinalizeWorktreeSession(context.Background(), mockVCS, "feature/fallback-finalize", false, "", sweSystem, session, &stderr, "", "", "")
	require.NoError(t, finalizeErr)

	commitCalls := mockVCS.GetCommitCalls()
	require.Len(t, commitCalls, 1)
	assert.Equal(t, "[feature/fallback-finalize] fallback generated message", commitCalls[0].Message)
	require.Len(t, primary.RecordedMessages, 1)
	require.Len(t, secondary.RecordedMessages, 1)
}

func TestFinalizeWorktreeSessionMergeUnstashConflictRestoresCleanStateAndKeepsStash(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "merge with local changes", true)
	repoDir := t.TempDir()

	originalRunGit := vcs.RunGitCommand
	defer func() {
		vcs.RunGitCommand = originalRunGit
	}()

	commands := make([]string, 0)
	vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		commands = append(commands, fmt.Sprintf("%s::%s", workDir, command))
		switch command {
		case "rev-parse --abbrev-ref HEAD":
			return "main", nil
		case "status --porcelain":
			return " M README.md", nil
		case "stash push --include-untracked -m csw: automatic stash before merge":
			return "Saved working directory and index state", nil
		case "stash apply --index stash@{0}":
			return "", fmt.Errorf("runGitCommand() [cli.go]: git stash apply --index stash@{0} failed: CONFLICT (content): Merge conflict in README.md")
		case "reset --hard HEAD":
			return "HEAD is now at abc123", nil
		case "clean -fd":
			return "", nil
		case "stash drop stash@{0}":
			return "Dropped stash@{0}", nil
		default:
			return "", nil
		}
	}

	var stderr bytes.Buffer
	_, err := FinalizeWorktreeSession(context.Background(), mockVCS, "feature/stash-conflict", true, "", sweSystem, session, &stderr, repoDir, "", "")
	require.NoError(t, err)

	assert.Contains(t, commands, fmt.Sprintf("%s::status --porcelain", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::stash push --include-untracked -m csw: automatic stash before merge", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::stash apply --index stash@{0}", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::reset --hard HEAD", repoDir))
	assert.Contains(t, commands, fmt.Sprintf("%s::clean -fd", repoDir))
	assert.NotContains(t, commands, fmt.Sprintf("%s::stash drop stash@{0}", repoDir))
	assert.Contains(t, stderr.String(), "automatic unstash failed due to conflicts")
	assert.Contains(t, stderr.String(), "local changes remain stashed")
	assert.Contains(t, stderr.String(), "please unstash manually when ready")
}

func newFinalizeWorktreeFixture(t *testing.T, llmMessage string, includeSystemTemplate bool) (*SweSystem, *core.SweSession, *vfs.MockVCS) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, llmMessage),
	})

	configStore := &conf.CswConfig{AgentConfigFiles: map[string]map[string]string{
		"commit": {
			"prompt.md":  "{{- range .Messages }}{{ . }}\n{{- end }}",
			"message.md": "[{{ .Branch }}] {{ .Message }}",
		},
	}}
	if includeSystemTemplate {
		configStore.AgentConfigFiles["commit"]["system.md"] = "system prompt"
	}

	system := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		Config:         configStore,
	}

	session, err := system.NewSession("mock/test-model", nil)
	require.NoError(t, err)

	mockVCS := vfs.NewMockVCS(vfs.NewMockVFS())
	return system, session, mockVCS
}

func TestMergeWorktreeWithConflictResolution(t *testing.T) {
	t.Run("resolves conflict via sub-session and retries rebase", func(t *testing.T) {
		sweSystem, session := newConflictResolutionFixture(t)
		repoDir := t.TempDir()
		worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "test")
		baseBranch := "develop"
		originalRunGit := vcs.RunGitCommand
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := vcs.ListGitConflictFiles
		defer func() {
			vcs.RunGitCommand = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			vcs.ListGitConflictFiles = originalListConflicts
		}()

		commands := make([]string, 0)
		resetCalls := make([]string, 0)
		checkedOutBasePath := filepath.Join(repoDir, "develop")
		mergeWorktreePath := ""
		rebaseCalls := 0
		vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
			joined := fmt.Sprintf("%s::git %s", workDir, strings.Join(args, " "))
			commands = append(commands, joined)

			switch {
			case strings.HasSuffix(joined, "::git rebase develop"):
				rebaseCalls++
				if rebaseCalls == 1 {
					return "", fmt.Errorf("runGitCommand() [cli.go]: git rebase develop failed: CONFLICT (content): Merge conflict in pkg/core/session.go")
				}
				return "", nil
			case strings.Contains(joined, "::git worktree add --force"):
				if len(args) == 5 {
					mergeWorktreePath = args[3]
				}
				return "", nil
			case strings.HasSuffix(joined, "::git merge --ff-only feature/test"):
				return "", nil
			case strings.HasSuffix(joined, "::git rev-parse HEAD"):
				return "abc123", nil
			case len(args) == 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain":
				return strings.Join([]string{
					"worktree " + checkedOutBasePath,
					"HEAD 1111111",
					"branch refs/heads/develop",
					"",
					"worktree " + mergeWorktreePath,
					"HEAD 2222222",
					"branch refs/heads/develop",
				}, "\n"), nil
			case len(args) == 3 && args[0] == "reset" && args[1] == "--hard" && args[2] == "develop":
				resetCalls = append(resetCalls, workDir)
				return "", nil
			case strings.Contains(joined, "::git worktree remove --force"):
				return "", nil
			default:
				return "", nil
			}
		}

		vcs.ListGitConflictFiles = func(workDir string) []string {
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
		headCommitID, err := mergeWorktreeWithConflictResolution(context.Background(), repoDir, worktreeDir, "feature/test", baseBranch, "parent task prompt", sweSystem, session, &stderr)
		require.NoError(t, err)
		assert.Equal(t, "abc123", headCommitID)
		assert.Equal(t, 1, subAgentCalls)
		require.Len(t, resetCalls, 1)
		assert.Equal(t, checkedOutBasePath, resetCalls[0])
		assert.NotEmpty(t, commands)
		assert.Contains(t, stderr.String(), "conflict detected")
	})

	t.Run("fails when sub-session reports error status", func(t *testing.T) {
		sweSystem, session := newConflictResolutionFixture(t)
		repoDir := t.TempDir()
		worktreeDir := filepath.Join(repoDir, ".cswdata", "work", "feature", "test")
		originalRunGit := vcs.RunGitCommand
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := vcs.ListGitConflictFiles
		defer func() {
			vcs.RunGitCommand = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			vcs.ListGitConflictFiles = originalListConflicts
		}()

		vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
			if len(args) == 2 && args[0] == "rebase" && args[1] == "develop" {
				return "", fmt.Errorf("runGitCommand() [cli.go]: git rebase develop failed: CONFLICT (content): Merge conflict in file.txt")
			}
			return "", nil
		}

		vcs.ListGitConflictFiles = func(workDir string) []string { return []string{"file.txt"} }
		executeConflictSubAgentFunc = func(parent *core.SweSession, request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
			return tool.SubAgentTaskResult{Status: "error", Summary: "failed"}, nil
		}

		_, err := mergeWorktreeWithConflictResolution(context.Background(), repoDir, worktreeDir, "feature/test", "develop", "prompt", sweSystem, session, &bytes.Buffer{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ended with status")
	})
}

func TestSyncCheckedOutBranchWorktrees(t *testing.T) {
	t.Run("updates checked-out target branch worktrees except skipped path", func(t *testing.T) {
		originalRunGit := vcs.RunGitCommand
		defer func() {
			vcs.RunGitCommand = originalRunGit
		}()

		worktreeListOutput := strings.Join([]string{
			"worktree /repo/main",
			"HEAD 1111111",
			"branch refs/heads/main",
			"",
			"worktree /repo/.cswdata/work/.merge-main-123",
			"HEAD 2222222",
			"branch refs/heads/main",
			"",
			"worktree /repo/worktrees/feature",
			"HEAD 3333333",
			"branch refs/heads/feature/test",
		}, "\n")

		resetCalls := make([]string, 0)
		vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
			if len(args) == 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
				assert.Equal(t, "/repo", workDir)
				return worktreeListOutput, nil
			}

			if len(args) == 3 && args[0] == "reset" && args[1] == "--hard" && args[2] == "main" {
				resetCalls = append(resetCalls, workDir)
				return "", nil
			}

			return "", nil
		}

		err := syncCheckedOutBranchWorktrees("/repo", "main", "/repo/.cswdata/work/.merge-main-123")
		require.NoError(t, err)
		require.Len(t, resetCalls, 1)
		assert.Equal(t, "/repo/main", resetCalls[0])
	})

	t.Run("returns error when checked-out branch worktree reset fails", func(t *testing.T) {
		originalRunGit := vcs.RunGitCommand
		defer func() {
			vcs.RunGitCommand = originalRunGit
		}()

		worktreeListOutput := strings.Join([]string{
			"worktree /repo/main",
			"HEAD 1111111",
			"branch refs/heads/main",
			"",
		}, "\n")

		vcs.RunGitCommand = func(workDir string, args ...string) (string, error) {
			if len(args) == 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
				return worktreeListOutput, nil
			}

			if len(args) == 3 && args[0] == "reset" && args[1] == "--hard" && args[2] == "main" {
				return "", fmt.Errorf("runGitCommand() [cli.go]: git reset --hard main failed: boom")
			}

			return "", nil
		}

		err := syncCheckedOutBranchWorktrees("/repo", "main", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update checked-out \"main\" worktree")
	})
}

func newConflictResolutionFixture(t *testing.T) (*SweSystem, *core.SweSession) {
	t.Helper()

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	configStore := &conf.CswConfig{AgentConfigFiles: map[string]map[string]string{
		"conflict": {
			"prompt.md": "You are running in a conflict resolution session.\n{{ .OriginalPrompt }}\n{{ .ConflictFiles }}\n{{ .ConflictOutput }}",
		},
	}}

	sweSystem := &SweSystem{
		ModelProviders: map[string]models.ModelProvider{"mock": provider},
		Config:         configStore,
	}

	session, err := sweSystem.NewSession("mock/test-model", nil)
	require.NoError(t, err)

	return sweSystem, session
}
