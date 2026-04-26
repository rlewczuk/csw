package system

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSystemParamsThinking(t *testing.T) {
	params := BuildSystemParams{
		Thinking: "high",
	}
	assert.Equal(t, "high", params.Thinking)
}

func TestBuildSystemParamsMaxToolThreads(t *testing.T) {
	params := BuildSystemParams{
		MaxToolThreads: 9,
	}
	assert.Equal(t, 9, params.MaxToolThreads)
}

func TestBuildSystemParamsAllowAllPermissions(t *testing.T) {
	params := BuildSystemParams{
		AllowAllPermissions: true,
	}
	assert.True(t, params.AllowAllPermissions)
}

func TestBuildSystemParamsNoRefresh(t *testing.T) {
	params := BuildSystemParams{
		NoRefresh: true,
	}
	assert.True(t, params.NoRefresh)
}

func TestPrepareSessionVFSWithoutWorktree(t *testing.T) {
	tmpDir := t.TempDir()

	repo, selectedVFS, err := PrepareSessionVFS(tmpDir, tmpDir, "", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isNull := repo.(*vcs.NullVCS)
	assert.True(t, isNull)
	assert.Equal(t, tmpDir, selectedVFS.WorktreePath())
}

func TestPrepareSessionVFSWithWorktreeCreatesBranchAndWorktree(t *testing.T) {
	repoDir := initTestGitRepository(t)

	repo, selectedVFS, err := PrepareSessionVFS(repoDir, repoDir, "feature/worktree", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isGit := repo.(*vcs.GitVCS)
	assert.True(t, isGit)

	expectedWorktreePath := filepath.Join(repoDir, ".cswdata", "work", "feature", "worktree")
	assert.Equal(t, expectedWorktreePath, selectedVFS.WorktreePath())

	_, err = selectedVFS.ReadFile("README.md")
	assert.NoError(t, err)

	branchCheck := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "refs/heads/feature/worktree")
	require.NoError(t, branchCheck.Run())
}

func TestPrepareSessionVFSWithMissingBranchCreatesBranchAndWorktree(t *testing.T) {
	repoDir := initTestGitRepository(t)

	repo, selectedVFS, err := PrepareSessionVFS(repoDir, repoDir, "feature/missing", nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)
	assert.Equal(t, filepath.Join(repoDir, ".cswdata", "work", "feature", "missing"), selectedVFS.WorktreePath())
}

func TestResolveWorktreeBranchName(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		modelName      string
		globalConfig   *conf.GlobalConfig
		providerConfig *conf.ModelProviderConfig
		aliases        map[string]conf.ModelAliasValue
		worktree       string
		generatorError error
		expected       string
		expectError    string
		generateCalls  int
	}{
		{name: "returns unchanged branch when no placeholder suffix", prompt: "Implement feature", modelName: "mock/test-model", worktree: "feature/fixed", expected: "feature/fixed", generateCalls: 0},
		{name: "returns error when placeholder used with empty prompt", prompt: "   ", modelName: "mock/test-model", worktree: "sp-1234-%", expectError: "requires non-empty prompt"},
		{name: "generates and appends branch suffix", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", expected: "sp-1234-worktree-cleanup", generateCalls: 1},
		{name: "prefix length does not affect generated suffix length", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "very-long-constant-prefix-%", expected: "very-long-constant-prefix-kebab-case-configuration", generateCalls: 1},
		{name: "generates and appends branch suffix with model alias", prompt: "Fix worktree cleanup issue", modelName: "default", aliases: map[string]conf.ModelAliasValue{"default": {Values: []string{"mock/test-model"}}}, worktree: "sp-1234-%", expected: "sp-1234-worktree-cleanup", generateCalls: 1},
		{name: "generates branch with retry on temporary provider error", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", expected: "sp-1234-worktree-cleanup", generateCalls: 1, globalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2}},
		{name: "generates branch using fallback model when primary fails", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model,mock/backup-model", worktree: "sp-1234-%", expected: "sp-1234-worktree-cleanup", generateCalls: 1, globalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 1}},
		{name: "propagates generator error", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", generatorError: errors.New("generation failed"), expectError: "generation failed", generateCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProviderConfig := &conf.ModelProviderConfig{Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}}
			if tt.providerConfig != nil {
				mockProviderConfig = tt.providerConfig.Clone()
			}
			store := &conf.CswConfig{ModelProviderConfigs: map[string]*conf.ModelProviderConfig{
				"mock": mockProviderConfig,
			}}
			if tt.globalConfig != nil {
				store.GlobalConfig = tt.globalConfig.Clone()
			}
			if len(tt.aliases) > 0 {
				store.ModelAliases = tt.aliases
			}

			originalNewComposite := newCompositeConfigStoreFunc
			originalResolveModel := resolveModelNameFunc
			originalCreateProviderMap := createProviderMapFunc
			originalGenerateBranch := generateWorktreeBranchNameFunc
			t.Cleanup(func() {
				newCompositeConfigStoreFunc = originalNewComposite
				resolveModelNameFunc = originalResolveModel
				createProviderMapFunc = originalCreateProviderMap
				generateWorktreeBranchNameFunc = originalGenerateBranch
			})

			newCompositeConfigStoreFunc = func(rootPath, configPath string) (*conf.CswConfig, error) {
				return store, nil
			}
			resolveModelNameFunc = func(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error) {
				return ResolveModelName(modelName, configStore, providerRegistry)
			}
			createProviderMapFunc = func(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
				provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}, {Name: "backup-model"}})
				provider.Config = mockProviderConfig.Clone()
				if strings.Contains(tt.name, "retry on temporary provider error") {
					rateLimitCount := 1
					provider.RateLimitError = &models.RateLimitError{Message: "rate exceeded", RetryAfterSeconds: 0}
					provider.RateLimitErrorCount = &rateLimitCount
				}
				if strings.Contains(tt.name, "fallback model") {
					provider.SetChatResponse("test-model", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
				}
				return map[string]models.ModelProvider{"mock": provider}, nil
			}

			generateCalls := 0
			generateWorktreeBranchNameFunc = func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore *conf.CswConfig, model string, inputPrompt string) (string, error) {
				generateCalls++
				if tt.generatorError != nil {
					return "", tt.generatorError
				}
				if strings.Contains(tt.name, "prefix length") {
					return "kebab-case-configuration", nil
				}
				return "worktree-cleanup", nil
			}

			branch, err := ResolveWorktreeBranchName(context.Background(), ResolveWorktreeBranchNameParams{
				Prompt:         tt.prompt,
				ModelName:      tt.modelName,
				WorkDir:        "",
				ShadowDir:      "",
				ProjectConfig:  "",
				ConfigPath:     "",
				WorktreeBranch: tt.worktree,
			})
			if tt.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, branch)
			assert.Equal(t, tt.generateCalls, generateCalls)
		})
	}
}

func TestResolveWorktreeBranchNameIgnoresBranchNameHook(t *testing.T) {
	store := &conf.CswConfig{ModelProviderConfigs: map[string]*conf.ModelProviderConfig{
		"mock": {Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}},
	}}

	originalNewComposite := newCompositeConfigStoreFunc
	originalResolveModel := resolveModelNameFunc
	originalCreateProviderMap := createProviderMapFunc
	originalGenerateBranch := generateWorktreeBranchNameFunc
	t.Cleanup(func() {
		newCompositeConfigStoreFunc = originalNewComposite
		resolveModelNameFunc = originalResolveModel
		createProviderMapFunc = originalCreateProviderMap
		generateWorktreeBranchNameFunc = originalGenerateBranch
	})

	newCompositeConfigStoreFunc = func(rootPath, configPath string) (*conf.CswConfig, error) {
		return store, nil
	}
	resolveModelNameFunc = func(modelName string, configStore *conf.CswConfig, providerRegistry *models.ProviderRegistry) (string, error) {
		return "mock/test-model", nil
	}

	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse(
		"test-model",
		&models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "hook-generated")},
	)
	createProviderMapFunc = func(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
		return map[string]models.ModelProvider{"mock": provider}, nil
	}

	generateCalls := 0
	generateWorktreeBranchNameFunc = func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore *conf.CswConfig, model string, inputPrompt string) (string, error) {
		generateCalls++
		return "fallback-generated", nil
	}

	branch, err := ResolveWorktreeBranchName(context.Background(), ResolveWorktreeBranchNameParams{
		Prompt:         "Add branch hook support",
		ModelName:      "mock/test-model",
		WorkDir:        "",
		ShadowDir:      "",
		ProjectConfig:  "",
		ConfigPath:     "",
		WorktreeBranch: "sp-%",
	})
	require.NoError(t, err)
	assert.Equal(t, "sp-fallback-generated", branch)
	assert.Equal(t, 1, generateCalls)
	assert.Empty(t, provider.RecordedMessages)
}


func initTestGitRepository(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()

	initCmd := exec.Command("git", "-C", repoDir, "init", "-b", "main")
	require.NoError(t, initCmd.Run())

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644))

	addCmd := exec.Command("git", "-C", repoDir, "add", "README.md")
	require.NoError(t, addCmd.Run())

	commitCmd := exec.Command("git", "-C", repoDir, "commit", "-m", "initial")
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	require.NoError(t, commitCmd.Run())

	return repoDir
}
