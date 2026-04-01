package vcs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			name:         "returns empty when git is not available",
			value:        "",
			gitConfigKey: "user.name",
			lookPathErr:  errors.New("git not found"),
			expected:     "",
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
			originalLookPath := GitLookPath
			originalConfigValue := GitConfigValue
			t.Cleanup(func() {
				GitLookPath = originalLookPath
				GitConfigValue = originalConfigValue
			})

			GitLookPath = func(file string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/git", nil
			}
			GitConfigValue = func(key string) (string, error) {
				return tt.gitConfigValue, tt.gitConfigErr
			}

			result := ResolveGitIdentity(tt.value, tt.gitConfigKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
