package system

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContainerMountSpec(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectError   bool
		expectedHost  string
		expectedMount string
	}{
		{name: "valid", input: "/host:/container", expectedHost: "/host", expectedMount: "/container"},
		{name: "invalid format", input: "/host", expectError: true},
		{name: "empty host", input: ":/container", expectError: true},
		{name: "empty container", input: "/host:", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hostPath, containerPath, err := ParseContainerMountSpec(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedHost, hostPath)
			assert.Equal(t, tc.expectedMount, containerPath)
		})
	}
}

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
	store, err := confimpl.NewEmbeddedConfigStore()
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

func TestParseContainerEnvSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedKey string
		expectedVal string
	}{
		{name: "valid", input: "KEY=value", expectedKey: "KEY", expectedVal: "value"},
		{name: "valid empty value", input: "KEY=", expectedKey: "KEY", expectedVal: ""},
		{name: "invalid format", input: "KEY", expectError: true},
		{name: "empty key", input: "=value", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, value, err := ParseContainerEnvSpec(tc.input)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedKey, key)
			assert.Equal(t, tc.expectedVal, value)
		})
	}
}

func TestResolveContainerRuntimeConfig(t *testing.T) {
	effectiveWorkDir := t.TempDir()
	extraMountHost := t.TempDir()
	shadowDir := t.TempDir()

	tests := []struct {
		name          string
		global        *conf.GlobalConfig
		params        BuildSystemParams
		expectError   bool
		expectEnabled bool
		expectImage   string
		expectEnvKey  string
		expectEnvVal  string
		expectMount   bool
	}{
		{name: "disabled by default", global: &conf.GlobalConfig{}, params: BuildSystemParams{}},
		{
			name: "enabled from global config",
			global: &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{
				Enabled: true,
				Image:   "busybox:latest",
				Env:     []string{"TEST_ENV=global"},
			}}},
			params:        BuildSystemParams{},
			expectEnabled: true,
			expectImage:   "busybox:latest",
			expectEnvKey:  "TEST_ENV",
			expectEnvVal:  "global",
		},
		{
			name:          "container enabled uses global image",
			global:        &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Image: "alpine:latest"}}},
			params:        BuildSystemParams{ContainerEnabled: true},
			expectEnabled: true,
			expectImage:   "alpine:latest",
		},
		{
			name:          "container disabled overrides global enabled",
			global:        &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Enabled: true, Image: "alpine:latest"}}},
			params:        BuildSystemParams{ContainerDisabled: true},
			expectEnabled: false,
		},
		{
			name:          "cli image overrides global image",
			global:        &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Enabled: true, Image: "alpine:latest"}}},
			params:        BuildSystemParams{ContainerImage: "busybox:1.36"},
			expectEnabled: true,
			expectImage:   "busybox:1.36",
		},
		{
			name:          "additional mounts are included",
			global:        &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Enabled: true, Image: "busybox:latest"}}},
			params:        BuildSystemParams{ContainerMounts: []string{extraMountHost + ":/mnt/extra"}},
			expectEnabled: true,
			expectImage:   "busybox:latest",
			expectMount:   true,
		},
		{
			name:        "enabled without image returns error",
			global:      &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Enabled: true}}},
			params:      BuildSystemParams{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := ResolveContainerRuntimeConfig(tc.global, tc.params, effectiveWorkDir, shadowDir)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectEnabled, resolved.Enabled)
			assert.Equal(t, tc.expectImage, resolved.Image)

			if tc.expectEnabled {
				hostPath, hasWorkdirMount := resolved.Mounts[effectiveWorkDir]
				require.True(t, hasWorkdirMount)
				assert.Equal(t, effectiveWorkDir, hostPath)

				shadowHostPath, hasShadowMount := resolved.Mounts[shadowDir]
				require.True(t, hasShadowMount)
				assert.Equal(t, shadowDir, shadowHostPath)
			}

			if tc.expectEnvKey != "" {
				require.NotNil(t, resolved.Env)
				assert.Equal(t, tc.expectEnvVal, resolved.Env[tc.expectEnvKey])
			}

			if tc.expectMount {
				absHost, absErr := filepath.Abs(extraMountHost)
				require.NoError(t, absErr)
				require.NotNil(t, resolved.Mounts)
				assert.Equal(t, absHost, resolved.Mounts["/mnt/extra"])
			}
		})
	}
}

func TestResolveContainerRuntimeConfig_ShadowDirNotMountedWhenEmpty(t *testing.T) {
	effectiveWorkDir := t.TempDir()

	resolved, err := ResolveContainerRuntimeConfig(
		&conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{Container: &conf.ContainerConfig{Enabled: true, Image: "busybox:latest"}}},
		BuildSystemParams{},
		effectiveWorkDir,
		"",
	)
	require.NoError(t, err)

	_, hasWorkdirMount := resolved.Mounts[effectiveWorkDir]
	require.True(t, hasWorkdirMount)
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

func TestResolveContainerGitAuthorIdentity(t *testing.T) {
	tests := []struct {
		name            string
		lookPathErr     error
		nameValue       string
		nameErr         error
		emailValue      string
		emailErr        error
		expectedName    string
		expectedEmail   string
		expectedQueries []string
	}{
		{name: "falls back to defaults when git is not available", lookPathErr: errors.New("not found"), expectedName: defaultGitAuthorName, expectedEmail: defaultGitAuthorEmail, expectedQueries: []string{}},
		{name: "uses git config values when available", nameValue: "Alice", emailValue: "alice@example.com", expectedName: "Alice", expectedEmail: "alice@example.com", expectedQueries: []string{"user.name", "user.email"}},
		{name: "uses defaults for missing git config values", nameErr: errors.New("missing name"), emailErr: errors.New("missing email"), expectedName: defaultGitAuthorName, expectedEmail: defaultGitAuthorEmail, expectedQueries: []string{"user.name", "user.email"}},
		{name: "mixes configured name with default email", nameValue: "Bob", emailErr: errors.New("missing email"), expectedName: "Bob", expectedEmail: defaultGitAuthorEmail, expectedQueries: []string{"user.name", "user.email"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLookPath := gitLookPathFunc
			originalConfigValue := gitConfigValueFunc
			t.Cleanup(func() {
				gitLookPathFunc = originalLookPath
				gitConfigValueFunc = originalConfigValue
			})

			queries := make([]string, 0)
			gitLookPathFunc = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			gitConfigValueFunc = func(key string) (string, error) {
				queries = append(queries, key)
				switch key {
				case "user.name":
					return tt.nameValue, tt.nameErr
				case "user.email":
					return tt.emailValue, tt.emailErr
				default:
					return "", errors.New("unexpected key")
				}
			}

			name, email := ResolveContainerGitAuthorIdentity()
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedEmail, email)
			assert.Equal(t, tt.expectedQueries, queries)
		})
	}
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
			store := confimpl.NewMockConfigStore()
			store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
				"mock": {Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}},
			})
			if len(tt.aliases) > 0 {
				store.SetModelAliases(tt.aliases)
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

			newCompositeConfigStoreFunc = func(rootPath, configPath string) (conf.ConfigStore, error) {
				return store, nil
			}
			resolveModelNameFunc = func(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
				return ResolveModelName(modelName, configStore, providerRegistry)
			}
			createProviderMapFunc = func(providerRegistry *models.ProviderRegistry) (map[string]models.ModelProvider, error) {
				provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
				return map[string]models.ModelProvider{"mock": provider}, nil
			}

			generateCalls := 0
			generateWorktreeBranchNameFunc = func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, model string, inputPrompt string) (string, error) {
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
	store := confimpl.NewMockConfigStore()
	store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"mock": {Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}},
	})
	store.SetHookConfigs(map[string]*conf.HookConfig{
		"branch_name": {
			Name:    "branch_name",
			Hook:    "branch_name",
			Enabled: true,
			Type:    conf.HookTypeLLM,
			Model:   "mock/test-model",
			Prompt:  "{{.user_prompt}}",
		},
	})

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

	newCompositeConfigStoreFunc = func(rootPath, configPath string) (conf.ConfigStore, error) {
		return store, nil
	}
	resolveModelNameFunc = func(modelName string, configStore conf.ConfigStore, providerRegistry *models.ProviderRegistry) (string, error) {
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
	generateWorktreeBranchNameFunc = func(ctx context.Context, modelProviders map[string]models.ModelProvider, configStore conf.ConfigStore, model string, inputPrompt string) (string, error) {
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

	store := confimpl.NewMockConfigStore()
	store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"resp": {
			Name:         "resp",
			Type:         "responses",
			URL:          server.URL,
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "old-refresh-token",
		},
	})

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

	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	updatedConfig, exists := configs["resp"]
	require.True(t, exists)
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
