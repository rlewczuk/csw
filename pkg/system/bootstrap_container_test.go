package system

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
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
