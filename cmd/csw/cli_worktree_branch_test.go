package main

import (
	"context"
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/core"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{
			name:          "returns unchanged branch when no placeholder suffix",
			prompt:        "Implement feature",
			modelName:     "mock/test-model",
			worktree:      "feature/fixed",
			expected:      "feature/fixed",
			generateCalls: 0,
		},
		{
			name:        "returns error when placeholder used with empty prompt",
			prompt:      "   ",
			modelName:   "mock/test-model",
			worktree:    "sp-1234-%",
			expectError: "requires non-empty prompt",
		},
		{
			name:          "generates and appends branch suffix",
			prompt:        "Fix worktree cleanup issue",
			modelName:     "mock/test-model",
			worktree:      "sp-1234-%",
			expected:      "sp-1234-worktree-cleanup",
			generateCalls: 1,
		},
		{
			name:           "propagates generator error",
			prompt:         "Fix worktree cleanup issue",
			modelName:      "mock/test-model",
			worktree:       "sp-1234-%",
			generatorError: errors.New("generation failed"),
			expectError:    "generation failed",
			generateCalls:  1,
		},
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
			generateWorktreeBranchNameFunc = func(ctx context.Context, sweSystem *core.SweSystem, model string, inputPrompt string) (string, error) {
				generateCalls++
				if tt.generatorError != nil {
					return "", tt.generatorError
				}
				return "worktree-cleanup", nil
			}

			branch, err := resolveWorktreeBranchName(context.Background(), tt.prompt, tt.modelName, "", "", tt.worktree)
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
