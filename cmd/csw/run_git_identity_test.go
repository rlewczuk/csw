package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/system"
	"github.com/rlewczuk/csw/pkg/vcs"
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
			originalRun := runFunc
			originalDefaultsResolver := resolveRunDefaultsFunc
			originalLookPath := vcs.GitLookPath
			originalConfigValue := vcs.GitConfigValue
			t.Cleanup(func() {
				runFunc = originalRun
				resolveRunDefaultsFunc = originalDefaultsResolver
				vcs.GitLookPath = originalLookPath
				vcs.GitConfigValue = originalConfigValue
			})

			resolveRunDefaultsFunc = func(params system.ResolveRunDefaultsParams) (conf.RunDefaultsConfig, error) {
				_ = params
				return conf.RunDefaultsConfig{}, nil
			}

			vcs.GitLookPath = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			vcs.GitConfigValue = func(key string) (string, error) {
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
			runFunc = func(params *RunParams) error {
				capturedName = params.GitUserName
				capturedEmail = params.GitUserEmail
				return nil
			}

			cmd := RunCommand()
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
