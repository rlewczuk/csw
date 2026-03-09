package system

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/vfs"
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
			global: &conf.GlobalConfig{Container: conf.ContainerConfig{
				Enabled: true,
				Image:   "busybox:latest",
				Env:     []string{"TEST_ENV=global"},
			}},
			params:        BuildSystemParams{},
			expectEnabled: true,
			expectImage:   "busybox:latest",
			expectEnvKey:  "TEST_ENV",
			expectEnvVal:  "global",
		},
		{
			name:          "container enabled uses global image",
			global:        &conf.GlobalConfig{Container: conf.ContainerConfig{Image: "alpine:latest"}},
			params:        BuildSystemParams{ContainerEnabled: true},
			expectEnabled: true,
			expectImage:   "alpine:latest",
		},
		{
			name:          "container disabled overrides global enabled",
			global:        &conf.GlobalConfig{Container: conf.ContainerConfig{Enabled: true, Image: "alpine:latest"}},
			params:        BuildSystemParams{ContainerDisabled: true},
			expectEnabled: false,
		},
		{
			name:          "cli image overrides global image",
			global:        &conf.GlobalConfig{Container: conf.ContainerConfig{Enabled: true, Image: "alpine:latest"}},
			params:        BuildSystemParams{ContainerImage: "busybox:1.36"},
			expectEnabled: true,
			expectImage:   "busybox:1.36",
		},
		{
			name:          "additional mounts are included",
			global:        &conf.GlobalConfig{Container: conf.ContainerConfig{Enabled: true, Image: "busybox:latest"}},
			params:        BuildSystemParams{ContainerMounts: []string{extraMountHost + ":/mnt/extra"}},
			expectEnabled: true,
			expectImage:   "busybox:latest",
			expectMount:   true,
		},
		{
			name:        "enabled without image returns error",
			global:      &conf.GlobalConfig{Container: conf.ContainerConfig{Enabled: true}},
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
		&conf.GlobalConfig{Container: conf.ContainerConfig{Enabled: true, Image: "busybox:latest"}},
		BuildSystemParams{},
		effectiveWorkDir,
		"",
	)
	require.NoError(t, err)

	_, hasWorkdirMount := resolved.Mounts[effectiveWorkDir]
	require.True(t, hasWorkdirMount)
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

	repo, selectedVFS, err := PrepareSessionVFS(tmpDir, tmpDir, "", false, nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isNull := repo.(*vfs.NullVCS)
	assert.True(t, isNull)
	assert.Equal(t, tmpDir, selectedVFS.WorktreePath())
}

func TestPrepareSessionVFSWithWorktreeCreatesBranchAndWorktree(t *testing.T) {
	repoDir := initTestGitRepository(t)

	repo, selectedVFS, err := PrepareSessionVFS(repoDir, repoDir, "feature/worktree", false, nil, "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, repo)
	require.NotNil(t, selectedVFS)

	_, isGit := repo.(*vfs.GitVCS)
	assert.True(t, isGit)

	expectedWorktreePath := filepath.Join(repoDir, ".cswdata", "work", "feature", "worktree")
	assert.Equal(t, expectedWorktreePath, selectedVFS.WorktreePath())

	_, err = selectedVFS.ReadFile("README.md")
	assert.NoError(t, err)

	branchCheck := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", "refs/heads/feature/worktree")
	require.NoError(t, branchCheck.Run())
}

func TestPrepareSessionVFSContinueModeFailsWhenBranchDoesNotExist(t *testing.T) {
	repoDir := initTestGitRepository(t)

	_, _, err := PrepareSessionVFS(repoDir, repoDir, "feature/missing", true, nil, "", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree branch \"feature/missing\" not found")
}

func TestResolveWorktreeBranchName(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		modelName      string
		worktree       string
		generatorError error
		expected       string
		expectError    string
		generateCalls  int
	}{
		{name: "returns unchanged branch when no placeholder suffix", prompt: "Implement feature", modelName: "mock/test-model", worktree: "feature/fixed", expected: "feature/fixed", generateCalls: 0},
		{name: "returns error when placeholder used with empty prompt", prompt: "   ", modelName: "mock/test-model", worktree: "sp-1234-%", expectError: "requires non-empty prompt"},
		{name: "generates and appends branch suffix", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", expected: "sp-1234-worktree-cleanup", generateCalls: 1},
		{name: "propagates generator error", prompt: "Fix worktree cleanup issue", modelName: "mock/test-model", worktree: "sp-1234-%", generatorError: errors.New("generation failed"), expectError: "generation failed", generateCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := confimpl.NewMockConfigStore()
			store.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
				"mock": {Name: "mock", Type: "openai", URL: "http://example.com", ModelTags: []conf.ModelTagMapping{}},
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
