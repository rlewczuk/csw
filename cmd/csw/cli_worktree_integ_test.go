package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/ui/mock"
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
			_, _ = finalizeWorktreeSession(context.Background(), mockVCS, tt.worktreeBranch, tt.merge, tt.customTemplate, system, session, &stderr, "", "", "", nil, nil)

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

	originalRunGit := runGitCommandFunc
	defer func() {
		runGitCommandFunc = originalRunGit
	}()

	runGitCommandFunc = func(workDir string, args ...string) (string, error) {
		if len(args) == 3 && args[0] == "rev-parse" && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
			return "develop", nil
		}
		return "", nil
	}

	_, _ = finalizeWorktreeSession(context.Background(), mockVCS, "feature/detect-base", true, "", sweSystem, session, &bytes.Buffer{}, "/repo", "", "", nil, nil)

	mergeCalls := mockVCS.GetMergeCalls()
	require.Len(t, mergeCalls, 1)
	assert.Equal(t, "develop", mergeCalls[0].Into)
	assert.Equal(t, "feature/detect-base", mergeCalls[0].From)
}

func TestFinalizeWorktreeSessionUsesMergeHook(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "merge via hook", true)
	configStore, ok := sweSystem.ConfigStore.(*confimpl.MockConfigStore)
	require.True(t, ok)
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-custom": {
			Name:    "merge-custom",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo merge-hook",
			RunOn:   conf.HookRunOnSandbox,
		},
	})

	hostRunner := runner.NewMockRunner()
	sandboxRunner := runner.NewMockRunner()
	sandboxRunner.SetResponseDetailed("echo merge-hook", "merge ok\n", "", 0, nil)
	hookEngine := core.NewHookEngine(configStore, hostRunner, sandboxRunner, sweSystem.ModelProviders)
	hookEngine.MergeContext(map[string]string{
		"branch":  "feature/hook",
		"workdir": "/repo/work",
		"rootdir": "/repo",
		"status":  string(core.HookSessionStatusRunning),
	})
	appView := mock.NewMockAppView()

	result, err := finalizeWorktreeSession(context.Background(), mockVCS, "feature/hook", true, "", sweSystem, session, &bytes.Buffer{}, "/repo", "/repo/work", "", hookEngine, appView)
	require.NoError(t, err)

	assert.Equal(t, "", result.HeadCommitID)
	require.Empty(t, mockVCS.GetMergeCalls())
	require.Len(t, sandboxRunner.GetExecutions(), 1)
	assert.Equal(t, "echo merge-hook", sandboxRunner.GetExecutions()[0].Command)
	require.Len(t, appView.ShowMessageCalls, 2)
	assert.Contains(t, appView.ShowMessageCalls[0].Message, "[hook:merge-custom] command")
	assert.Contains(t, appView.ShowMessageCalls[1].Message, "[hook:merge-custom][stdout]")
}

func TestFinalizeWorktreeSessionMergeHookProcessesFeedbackRequests(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "merge via hook feedback", true)
	provider, ok := sweSystem.ModelProviders["mock"].(*models.MockClient)
	require.True(t, ok)
	provider.AddChatResponse("test-model", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "feedback-response")})

	configStore, ok := sweSystem.ConfigStore.(*confimpl.MockConfigStore)
	require.True(t, ok)
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"merge-feedback": {
			Name:    "merge-feedback",
			Hook:    "merge",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo merge-feedback",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed(
		"echo merge-feedback",
		strings.Join([]string{
			"CSWFEEDBACK: {\"fn\":\"context\",\"args\":{\"feedback-state\":\"ready\"},\"response\":\"none\",\"id\":\"ctx\"}",
			"CSWFEEDBACK: {\"fn\":\"llm\",\"args\":{\"prompt\":\"feedback prompt\",\"model\":\"mock/test-model\"},\"response\":\"stdin\",\"id\":\"stdin-1\"}",
			"CSWFEEDBACK: {\"fn\":\"llm\",\"args\":{\"prompt\":\"feedback prompt rerun\",\"model\":\"mock/test-model\"},\"response\":\"rerun\",\"id\":\"rerun-1\"}",
		}, "\n")+"\n",
		"",
		0,
		nil,
	)
	hookEngine := core.NewHookEngine(configStore, hostRunner, nil, sweSystem.ModelProviders)
	appView := mock.NewMockAppView()

	_, _ = finalizeWorktreeSession(context.Background(), mockVCS, "feature/hook-feedback", true, "", sweSystem, session, &bytes.Buffer{}, "/repo", "/repo/work", "", hookEngine, appView)

	assert.Equal(t, "ready", hookEngine.ContextData()["feedback-state"])
	executions := hostRunner.GetExecutions()
	require.Len(t, executions, 3)
	assert.Equal(t, "echo merge-feedback", executions[0].Command)
	assert.Contains(t, executions[1].Command, "| (echo merge-feedback)")
	assert.Contains(t, executions[2].Command, "CSW_RESPONSE=")
	assert.Contains(t, executions[2].Command, "echo merge-feedback")
	require.Len(t, provider.RecordedMessages, 3)
}

func TestFinalizeWorktreeSessionCommitHookUsesReturnedCommitMessage(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "llm fallback message", true)
	configStore, ok := sweSystem.ConfigStore.(*confimpl.MockConfigStore)
	require.True(t, ok)
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"commit-custom": {
			Name:    "commit-custom",
			Hook:    "commit",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo commit-hook",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed(
		"echo commit-hook",
		"CSWFEEDBACK: {\"fn\":\"response\",\"args\":{\"status\":\"OK\",\"commit-message\":\"hook supplied message\"}}\n",
		"",
		0,
		nil,
	)
	hookEngine := core.NewHookEngine(configStore, hostRunner, nil, sweSystem.ModelProviders)

	_, err := finalizeWorktreeSession(context.Background(), mockVCS, "feature/commit-hook", false, "", sweSystem, session, &bytes.Buffer{}, "/repo", "/repo/work", "", hookEngine, nil)
	require.NoError(t, err)

	commitCalls := mockVCS.GetCommitCalls()
	require.Len(t, commitCalls, 1)
	assert.Equal(t, "hook supplied message", commitCalls[0].Message)
}

func TestFinalizeWorktreeSessionCommitHookCommittedStatusSkipsBuiltinCommit(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "llm fallback message", true)
	configStore, ok := sweSystem.ConfigStore.(*confimpl.MockConfigStore)
	require.True(t, ok)
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"commit-custom": {
			Name:    "commit-custom",
			Hook:    "commit",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo commit-hook",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed(
		"echo commit-hook",
		"CSWFEEDBACK: {\"fn\":\"response\",\"args\":{\"status\":\"COMMITED\"}}\n",
		"",
		0,
		nil,
	)
	hookEngine := core.NewHookEngine(configStore, hostRunner, nil, sweSystem.ModelProviders)

	originalRunGit := runGitCommandFunc
	defer func() {
		runGitCommandFunc = originalRunGit
	}()

	commands := make([]string, 0)
	runGitCommandFunc = func(workDir string, args ...string) (string, error) {
		commands = append(commands, fmt.Sprintf("%s::%s", workDir, strings.Join(args, " ")))
		if len(args) == 3 && args[0] == "rev-parse" && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
			return "main", nil
		}
		return "", nil
	}

	_, err := finalizeWorktreeSession(context.Background(), mockVCS, "feature/commit-hook", false, "", sweSystem, session, &bytes.Buffer{}, "/repo", "/repo/work", "", hookEngine, nil)
	require.NoError(t, err)

	assert.Empty(t, mockVCS.GetCommitCalls())
	assert.Contains(t, commands, "/repo/work::reset --hard HEAD")
	assert.Contains(t, commands, "/repo/work::clean -fd")
}

func TestFinalizeWorktreeSessionCommitHookErrorStatusAborts(t *testing.T) {
	sweSystem, session, mockVCS := newFinalizeWorktreeFixture(t, "llm fallback message", true)
	configStore, ok := sweSystem.ConfigStore.(*confimpl.MockConfigStore)
	require.True(t, ok)
	configStore.SetHookConfigs(map[string]*conf.HookConfig{
		"commit-custom": {
			Name:    "commit-custom",
			Hook:    "commit",
			Enabled: true,
			Type:    conf.HookTypeShell,
			Command: "echo commit-hook",
			RunOn:   conf.HookRunOnHost,
		},
	})

	hostRunner := runner.NewMockRunner()
	hostRunner.SetResponseDetailed(
		"echo commit-hook",
		"CSWFEEDBACK: {\"fn\":\"response\",\"args\":{\"status\":\"ERROR\"}}\n",
		"",
		0,
		nil,
	)
	hookEngine := core.NewHookEngine(configStore, hostRunner, nil, sweSystem.ModelProviders)

	_, err := finalizeWorktreeSession(context.Background(), mockVCS, "feature/commit-hook", false, "", sweSystem, session, &bytes.Buffer{}, "/repo", "/repo/work", "", hookEngine, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit hook")
	assert.Empty(t, mockVCS.GetCommitCalls())
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
		baseBranch := "develop"
		originalRunGit := runGitCommandFunc
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := listGitConflictFilesFunc
		defer func() {
			runGitCommandFunc = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			listGitConflictFilesFunc = originalListConflicts
		}()

		commands := make([]string, 0)
		resetCalls := make([]string, 0)
		checkedOutBasePath := filepath.Join(repoDir, "develop")
		mergeWorktreePath := ""
		rebaseCalls := 0
		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
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
		originalRunGit := runGitCommandFunc
		originalSubAgent := executeConflictSubAgentFunc
		originalListConflicts := listGitConflictFilesFunc
		defer func() {
			runGitCommandFunc = originalRunGit
			executeConflictSubAgentFunc = originalSubAgent
			listGitConflictFilesFunc = originalListConflicts
		}()

		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
			if len(args) == 2 && args[0] == "rebase" && args[1] == "develop" {
				return "", fmt.Errorf("runGitCommand() [cli.go]: git rebase develop failed: CONFLICT (content): Merge conflict in file.txt")
			}
			return "", nil
		}

		listGitConflictFilesFunc = func(workDir string) []string { return []string{"file.txt"} }
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
		originalRunGit := runGitCommandFunc
		defer func() {
			runGitCommandFunc = originalRunGit
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
		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
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
		originalRunGit := runGitCommandFunc
		defer func() {
			runGitCommandFunc = originalRunGit
		}()

		worktreeListOutput := strings.Join([]string{
			"worktree /repo/main",
			"HEAD 1111111",
			"branch refs/heads/main",
			"",
		}, "\n")

		runGitCommandFunc = func(workDir string, args ...string) (string, error) {
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
