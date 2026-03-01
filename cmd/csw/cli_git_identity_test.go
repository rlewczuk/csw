package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIGitIdentityPropagation verifies that csw cli resolves host git identity and forwards it to runtime params.
func TestCLIGitIdentityPropagation(t *testing.T) {
	tests := []struct {
		name          string
		lookPathErr   error
		gitName       string
		gitNameErr    error
		gitEmail      string
		gitEmailErr   error
		expectedName  string
		expectedEmail string
	}{
		{
			name:          "uses host git config values",
			gitName:       "CLI User",
			gitEmail:      "cli.user@example.com",
			expectedName:  "CLI User",
			expectedEmail: "cli.user@example.com",
		},
		{
			name:          "returns empty values when git binary is missing",
			lookPathErr:   errors.New("not found"),
			expectedName:  "",
			expectedEmail: "",
		},
		{
			name:          "returns empty value when git config key is missing",
			gitName:       "CLI User",
			gitEmailErr:   errors.New("missing user.email"),
			expectedName:  "CLI User",
			expectedEmail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := impl.NewMockConfigStore()
			mockStore.SetGlobalConfig(&conf.GlobalConfig{})

			originalRun := runCLIFunc
			originalConfigStoreBuilder := newCompositeConfigStoreFunc
			originalLookPath := gitLookPathFunc
			originalConfigValue := gitConfigValueFunc
			t.Cleanup(func() {
				runCLIFunc = originalRun
				newCompositeConfigStoreFunc = originalConfigStoreBuilder
				gitLookPathFunc = originalLookPath
				gitConfigValueFunc = originalConfigValue
			})

			newCompositeConfigStoreFunc = func(projDir string, configPath string) (conf.ConfigStore, error) {
				return mockStore, nil
			}

			gitLookPathFunc = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			gitConfigValueFunc = func(key string) (string, error) {
				switch key {
				case "user.name":
					return tt.gitName, tt.gitNameErr
				case "user.email":
					return tt.gitEmail, tt.gitEmailErr
				default:
					return "", errors.New("unexpected key")
				}
			}

			capturedName := ""
			capturedEmail := ""
			runCLIFunc = func(params *CLIParams) error {
				capturedName = params.GitUserName
				capturedEmail = params.GitUserEmail
				return nil
			}

			cmd := CliCommand()
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs([]string{"prompt"})

			err := cmd.Execute()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedName, capturedName)
			assert.Equal(t, tt.expectedEmail, capturedEmail)
		})
	}
}

// TestResolveGitIdentity tests the resolveGitIdentity function.
func TestResolveGitIdentity(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		gitConfigKey   string
		lookPathErr    error
		gitConfigValue string
		gitConfigErr   error
		expected       string
	}{
		{
			name:         "returns provided value when not empty",
			value:        "Provided User",
			gitConfigKey: "user.name",
			expected:     "Provided User",
		},
		{
			name:           "falls back to git config when value is empty",
			value:          "",
			gitConfigKey:   "user.name",
			gitConfigValue: "Git Config User",
			expected:       "Git Config User",
		},
		{
			name:           "returns empty when both value and git config are empty",
			value:          "",
			gitConfigKey:   "user.email",
			gitConfigValue: "",
			expected:       "",
		},
		{
			name:          "returns empty when git is not available",
			value:         "",
			gitConfigKey:  "user.name",
			lookPathErr:   errors.New("git not found"),
			expected:      "",
		},
		{
			name:           "returns empty when git config fails",
			value:          "",
			gitConfigKey:   "user.email",
			gitConfigValue: "",
			gitConfigErr:   errors.New("config error"),
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLookPath := gitLookPathFunc
			originalConfigValue := gitConfigValueFunc
			t.Cleanup(func() {
				gitLookPathFunc = originalLookPath
				gitConfigValueFunc = originalConfigValue
			})

			gitLookPathFunc = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			gitConfigValueFunc = func(key string) (string, error) {
				return tt.gitConfigValue, tt.gitConfigErr
			}

			result := resolveGitIdentity(tt.value, tt.gitConfigKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
