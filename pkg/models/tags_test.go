package models

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelTagRegistry_GetTagsForModel_GlobalMappings(t *testing.T) {
	registry := NewModelTagRegistry()

	// Set global mappings
	err := registry.SetGlobalMappings([]conf.ModelTagMapping{
		{Model: "^claude-.*", Tag: "anthropic"},
		{Model: "^gpt-.*", Tag: "openai"},
		{Model: ".*-instruct$", Tag: "instruct"},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		modelName    string
		wantTags     []string
	}{
		{
			name:         "matches claude model",
			providerName: "anthropic",
			modelName:    "claude-3-opus",
			wantTags:     []string{"anthropic"},
		},
		{
			name:         "matches gpt model",
			providerName: "openai",
			modelName:    "gpt-4-turbo",
			wantTags:     []string{"openai"},
		},
		{
			name:         "matches instruct suffix",
			providerName: "openai",
			modelName:    "gpt-3.5-turbo-instruct",
			wantTags:     []string{"instruct", "openai"},
		},
		{
			name:         "no matches",
			providerName: "ollama",
			modelName:    "llama-7b",
			wantTags:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags := registry.GetTagsForModel(tt.providerName, tt.modelName)
			sort.Strings(gotTags)
			sort.Strings(tt.wantTags)
			assert.Equal(t, tt.wantTags, gotTags)
		})
	}
}

func TestModelTagRegistry_GetTagsForModel_ProviderMappings(t *testing.T) {
	registry := NewModelTagRegistry()

	// Set provider-specific mappings
	err := registry.SetProviderMappings("anthropic", []conf.ModelTagMapping{
		{Model: "^claude-3-opus.*", Tag: "opus"},
		{Model: "^claude-3-sonnet.*", Tag: "sonnet"},
	})
	require.NoError(t, err)

	err = registry.SetProviderMappings("openai", []conf.ModelTagMapping{
		{Model: "^gpt-4.*", Tag: "gpt4"},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		modelName    string
		wantTags     []string
	}{
		{
			name:         "matches opus from anthropic provider",
			providerName: "anthropic",
			modelName:    "claude-3-opus-20240229",
			wantTags:     []string{"opus"},
		},
		{
			name:         "matches sonnet from anthropic provider",
			providerName: "anthropic",
			modelName:    "claude-3-sonnet-20240229",
			wantTags:     []string{"sonnet"},
		},
		{
			name:         "openai mappings dont apply to anthropic",
			providerName: "anthropic",
			modelName:    "gpt-4-turbo",
			wantTags:     []string{},
		},
		{
			name:         "matches gpt4 from openai provider",
			providerName: "openai",
			modelName:    "gpt-4-turbo",
			wantTags:     []string{"gpt4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags := registry.GetTagsForModel(tt.providerName, tt.modelName)
			sort.Strings(gotTags)
			sort.Strings(tt.wantTags)
			assert.Equal(t, tt.wantTags, gotTags)
		})
	}
}

func TestModelTagRegistry_GetTagsForModel_MergedMappings(t *testing.T) {
	registry := NewModelTagRegistry()

	// Set global mappings
	err := registry.SetGlobalMappings([]conf.ModelTagMapping{
		{Model: "^claude-.*", Tag: "anthropic"},
		{Model: "^gpt-.*", Tag: "openai"},
	})
	require.NoError(t, err)

	// Set provider-specific mappings
	err = registry.SetProviderMappings("anthropic", []conf.ModelTagMapping{
		{Model: "^claude-3-opus.*", Tag: "opus"},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		providerName string
		modelName    string
		wantTags     []string
	}{
		{
			name:         "merges global and provider tags",
			providerName: "anthropic",
			modelName:    "claude-3-opus-20240229",
			wantTags:     []string{"anthropic", "opus"},
		},
		{
			name:         "only global tags when no provider match",
			providerName: "anthropic",
			modelName:    "claude-2.1",
			wantTags:     []string{"anthropic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags := registry.GetTagsForModel(tt.providerName, tt.modelName)
			sort.Strings(gotTags)
			sort.Strings(tt.wantTags)
			assert.Equal(t, tt.wantTags, gotTags)
		})
	}
}

func TestModelTagRegistry_LoadGlobalConfig(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Write a test global config
	configPath := filepath.Join(tmpDir, "global.json")
	configContent := `{
  "model_tags": [
    {"model": "^claude-.*", "tag": "anthropic"},
    {"model": "^gpt-.*", "tag": "openai"}
  ]
}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	registry := NewModelTagRegistry()
	err = registry.LoadGlobalConfig(configPath)
	require.NoError(t, err)

	// Test that mappings were loaded
	tags := registry.GetTagsForModel("any", "claude-3-opus")
	assert.Equal(t, []string{"anthropic"}, tags)

	tags = registry.GetTagsForModel("any", "gpt-4")
	assert.Equal(t, []string{"openai"}, tags)
}

func TestModelTagRegistry_LoadGlobalConfig_NonExistent(t *testing.T) {
	registry := NewModelTagRegistry()

	// Should not error on non-existent file
	err := registry.LoadGlobalConfig("/nonexistent/path/global.json")
	require.NoError(t, err)

	// Should return empty tags
	tags := registry.GetTagsForModel("any", "any-model")
	assert.Empty(t, tags)
}

func TestModelTagRegistry_LoadGlobalConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "global.json")

	err := os.WriteFile(configPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	registry := NewModelTagRegistry()
	err = registry.LoadGlobalConfig(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestModelTagRegistry_LoadGlobalConfig_InvalidRegexp(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "global.json")

	// Invalid regexp pattern
	configContent := `{
  "model_tags": [
    {"model": "[invalid", "tag": "test"}
  ]
}`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	registry := NewModelTagRegistry()
	err = registry.LoadGlobalConfig(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regexp")
}

func TestModelTagRegistry_SetGlobalMappings_InvalidRegexp(t *testing.T) {
	registry := NewModelTagRegistry()

	err := registry.SetGlobalMappings([]conf.ModelTagMapping{
		{Model: "[invalid", Tag: "test"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regexp")
}

func TestModelTagRegistry_SetProviderMappings_InvalidRegexp(t *testing.T) {
	registry := NewModelTagRegistry()

	err := registry.SetProviderMappings("test", []conf.ModelTagMapping{
		{Model: "[invalid", Tag: "test"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regexp")
}

func TestModelTagRegistry_GetAllProviderNames(t *testing.T) {
	registry := NewModelTagRegistry()

	// Set some provider mappings
	err := registry.SetProviderMappings("anthropic", []conf.ModelTagMapping{{Model: ".*", Tag: "test"}})
	require.NoError(t, err)
	err = registry.SetProviderMappings("openai", []conf.ModelTagMapping{{Model: ".*", Tag: "test"}})
	require.NoError(t, err)

	names := registry.GetAllProviderNames()
	sort.Strings(names)
	assert.Equal(t, []string{"anthropic", "openai"}, names)
}

func TestModelTagRegistry_DuplicateTags(t *testing.T) {
	registry := NewModelTagRegistry()

	// Both global and provider mappings assign the same tag
	err := registry.SetGlobalMappings([]conf.ModelTagMapping{
		{Model: "^claude-.*", Tag: "claude"},
	})
	require.NoError(t, err)

	err = registry.SetProviderMappings("anthropic", []conf.ModelTagMapping{
		{Model: "^claude-.*", Tag: "claude"},
	})
	require.NoError(t, err)

	// Should not have duplicates
	tags := registry.GetTagsForModel("anthropic", "claude-3-opus")
	assert.Equal(t, []string{"claude"}, tags)
}

func TestModelTagRegistry_MultiplePatternsSameTag(t *testing.T) {
	registry := NewModelTagRegistry()

	// Multiple patterns can assign the same tag
	err := registry.SetGlobalMappings([]conf.ModelTagMapping{
		{Model: "^claude-.*", Tag: "large-model"},
		{Model: "^gpt-4.*", Tag: "large-model"},
	})
	require.NoError(t, err)

	tags := registry.GetTagsForModel("anthropic", "claude-3-opus")
	assert.Equal(t, []string{"large-model"}, tags)

	tags = registry.GetTagsForModel("openai", "gpt-4-turbo")
	assert.Equal(t, []string{"large-model"}, tags)
}
