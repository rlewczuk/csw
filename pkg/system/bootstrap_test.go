package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestApplyDisableRefreshToProviders(t *testing.T) {
	mockProvider := models.NewMockProvider(nil)
	mockProvider.Config = &conf.ModelProviderConfig{Name: "mock-provider"}

	providers := map[string]models.ModelProvider{
		"mock": mockProvider,
	}

	applyDisableRefreshToProviders(providers)

	assert.NotNil(t, mockProvider.GetConfig())
	assert.True(t, mockProvider.GetConfig().DisableRefresh)
}

func TestKimiTagResolution(t *testing.T) {
	store, err := conf.CswConfigLoad("@DEFAULTS")
	require.NoError(t, err)

	providerRegistry := models.NewProviderRegistry(store)

	registry, err := CreateModelTagRegistry(store, providerRegistry)
	require.NoError(t, err)

	tests := []struct {
		modelName     string
		expectedTag   string
		shouldContain bool
	}{
		{modelName: "kimi/kimi-for-coding", expectedTag: "kimi", shouldContain: true},
		{modelName: "kimi/moonshot-v1-8k", expectedTag: "kimi", shouldContain: true},
		{modelName: "openai/gpt-4", expectedTag: "openai", shouldContain: true},
		{modelName: "anthropic/claude-3-opus", expectedTag: "anthropic", shouldContain: true},
	}

	for _, tc := range tests {
		t.Run(tc.modelName, func(t *testing.T) {
			var provider, model string
			parts := strings.Split(tc.modelName, "/")
			if len(parts) == 2 {
				provider = parts[0]
				model = parts[1]
			} else {
				model = tc.modelName
			}

			tags := registry.GetTagsForModel(provider, model)
			if tc.shouldContain {
				assert.Contains(t, tags, tc.expectedTag, "Model %s (provider=%s, model=%s) should have tag %s. Got: %v", tc.modelName, provider, model, tc.expectedTag, tags)
			} else {
				assert.NotContains(t, tags, tc.expectedTag, "Model %s should NOT have tag %s", tc.modelName, tc.expectedTag)
			}
		})
	}
}

func TestResolveWorkDir_UsesExplicitDirOverride(t *testing.T) {
	overrideDir := t.TempDir()

	resolved, err := ResolveWorkDir(overrideDir)
	require.NoError(t, err)

	expected, err := filepath.Abs(overrideDir)
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestResolveWorkDir_FindsNearestProjectDirWithDotCsw(t *testing.T) {
	rootDir := t.TempDir()
	markerDir := filepath.Join(rootDir, ".csw")
	require.NoError(t, os.MkdirAll(markerDir, 0755))

	nestedDir := filepath.Join(rootDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, rootDir, resolved)
}

func TestResolveWorkDir_FindsNearestProjectDirWithDotCswdata(t *testing.T) {
	rootDir := t.TempDir()
	markerDir := filepath.Join(rootDir, ".cswdata")
	require.NoError(t, os.MkdirAll(markerDir, 0755))

	nestedDir := filepath.Join(rootDir, "deep", "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, rootDir, resolved)
}

func TestResolveWorkDir_ReturnsCurrentDirWhenNoProjectMarkerFound(t *testing.T) {
	baseDir := t.TempDir()
	nestedDir := filepath.Join(baseDir, "x", "y")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	require.NoError(t, os.Chdir(nestedDir))

	resolved, err := ResolveWorkDir("")
	require.NoError(t, err)
	assert.Equal(t, nestedDir, resolved)
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
		{name: "propagates generator error", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", generatorError: errors.New("generation failed"), expectError: "generation failed", generateCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &conf.CswConfig{ModelProviderConfigs: map[string]*conf.ModelProviderConfig{
				"mock": {Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}},
			}}
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
				provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
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

func TestCreateProviderMapConfigUpdaterWiring(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", tmpHome))
	defer os.Setenv("HOME", oldHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	store := &conf.CswConfig{ModelProviderConfigs: map[string]*conf.ModelProviderConfig{
		"resp": {
			Name:         "resp",
			Type:         "responses",
			URL:          server.URL,
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "old-refresh-token",
		},
	}}

	registry := models.NewProviderRegistry(store)
	providers, err := CreateProviderMap(registry)
	require.NoError(t, err)
	require.Len(t, providers, 1)

	provider, exists := providers["resp"]
	require.True(t, exists)

	responsesClient, ok := provider.(*models.ResponsesClient)
	require.True(t, ok)

	err = responsesClient.RefreshTokenIfNeeded()
	require.NoError(t, err)

	updatedConfigPath := filepath.Join(tmpHome, ".config", "csw", "models", "resp.json")
	updatedConfigData, err := os.ReadFile(updatedConfigPath)
	require.NoError(t, err)

	var updatedConfig conf.ModelProviderConfig
	require.NoError(t, json.Unmarshal(updatedConfigData, &updatedConfig))
	assert.Equal(t, "new-access-token", updatedConfig.APIKey)
	assert.Equal(t, "new-refresh-token", updatedConfig.RefreshToken)
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
