package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveContainerGitAuthorIdentity tests git author identity resolution for container mode.
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
		{
			name:            "falls back to defaults when git is not available",
			lookPathErr:     errors.New("not found"),
			expectedName:    defaultGitAuthorName,
			expectedEmail:   defaultGitAuthorEmail,
			expectedQueries: []string{},
		},
		{
			name:            "uses git config values when available",
			nameValue:       "Alice",
			emailValue:      "alice@example.com",
			expectedName:    "Alice",
			expectedEmail:   "alice@example.com",
			expectedQueries: []string{"user.name", "user.email"},
		},
		{
			name:            "uses defaults for missing git config values",
			nameErr:         errors.New("missing name"),
			emailErr:        errors.New("missing email"),
			expectedName:    defaultGitAuthorName,
			expectedEmail:   defaultGitAuthorEmail,
			expectedQueries: []string{"user.name", "user.email"},
		},
		{
			name:            "mixes configured name with default email",
			nameValue:       "Bob",
			emailErr:        errors.New("missing email"),
			expectedName:    "Bob",
			expectedEmail:   defaultGitAuthorEmail,
			expectedQueries: []string{"user.name", "user.email"},
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

			name, email := resolveContainerGitAuthorIdentity()
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedEmail, email)
			assert.Equal(t, tt.expectedQueries, queries)
		})
	}
}
