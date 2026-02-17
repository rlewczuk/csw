package main

import (
	"strings"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/codesnort/codesnort-swe/pkg/core"
	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cli_model_integ_test.go contains integration tests for model tag resolution
// and system prompt generation. These tests verify that the CLI properly handles
// model-specific configurations and generates appropriate prompts for different
// model providers like Kimi, OpenAI, and Anthropic.

func TestKimiTagResolution(t *testing.T) {
	// Create embedded config store (which should have our modified global.json)
	store, err := impl.NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Create empty provider registry
	providerRegistry := models.NewProviderRegistry(store)

	// Create model tag registry
	registry, err := CreateModelTagRegistry(store, providerRegistry)
	require.NoError(t, err)

	// Test cases
	tests := []struct {
		modelName     string
		expectedTag   string
		shouldContain bool
	}{
		{
			modelName:     "kimi/kimi-for-coding",
			expectedTag:   "kimi",
			shouldContain: true,
		},
		{
			modelName:     "kimi/moonshot-v1-8k",
			expectedTag:   "kimi",
			shouldContain: true,
		},
		{
			modelName:     "openai/gpt-4",
			expectedTag:   "openai",
			shouldContain: true,
		},
		{
			modelName:     "anthropic/claude-3-opus",
			expectedTag:   "anthropic",
			shouldContain: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.modelName, func(t *testing.T) {
			// Split provider/model
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

func TestSystemPromptGenerationForKimi(t *testing.T) {
	// Setup config store with embedded config (including our fix)
	store, err := impl.NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Setup VFS
	vfsInstance := vfs.NewMockVFS()

	// Setup prompt generator
	generator, err := core.NewConfPromptGenerator(store, vfsInstance)
	require.NoError(t, err)

	// Setup tags for kimi
	tags := []string{"kimi"}

	// Get developer role
	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	developerRole := roleConfigs["developer"]

	// Get state
	state := core.AgentState{
		Info: core.AgentStateCommonInfo{
			AgentName: "Kimi",
		},
	}

	// Generate prompt
	prompt, err := generator.GetPrompt(tags, developerRole, &state)
	require.NoError(t, err)

	// Verify it contains Kimi specific instructions
	// Content from 10-system-kimi.md
	assert.Contains(t, prompt, "interactive general AI agent")

	// Verify it does NOT contain generic instructions (50-system-generic.md is tagged 'generic')
	// The prompt generator excludes fragments with tags that are not in the provided tags list
	// 50-system-generic.md starts with "You are {{.Info.AgentName}}, an interactive CLI tool"
	assert.NotContains(t, prompt, "an interactive CLI tool")
}
