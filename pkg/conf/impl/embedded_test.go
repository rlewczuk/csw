package impl

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmbeddedConfigStore(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestEmbeddedConfigStore_GetGlobalConfig(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify global config was loaded (should have model tags from conf/global.json)
	assert.NotEmpty(t, config.ModelTags)

	// Verify that returned config is a copy (modification doesn't affect cached data)
	originalLen := len(config.ModelTags)
	if originalLen > 0 {
		config.ModelTags[0].Tag = "modified"
		config2, err := store.GetGlobalConfig()
		require.NoError(t, err)
		assert.NotEqual(t, "modified", config2.ModelTags[0].Tag)
	}
}

func TestEmbeddedConfigStore_LastGlobalConfigUpdate(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	updateTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)

	// Should return the constant embedded timestamp
	assert.Equal(t, embeddedTimestamp, updateTime)
	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), updateTime)

	// Multiple calls should return the same timestamp
	updateTime2, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)
	assert.Equal(t, updateTime, updateTime2)
}

func TestEmbeddedConfigStore_GetModelProviderConfigs(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.NotNil(t, configs)

	// Verify at least one provider is loaded from conf/models/
	assert.NotEmpty(t, configs)

	// Check for ollama provider (from conf/models/ollama.json)
	ollama, ok := configs["ollama-local"]
	if ok {
		assert.Equal(t, "ollama", ollama.Type)
		assert.Equal(t, "ollama-local", ollama.Name)
		assert.NotEmpty(t, ollama.URL)
	}

	// Verify that returned configs are copies
	if len(configs) > 0 {
		for name, cfg := range configs {
			originalType := cfg.Type
			cfg.Type = "modified"
			configs2, err := store.GetModelProviderConfigs()
			require.NoError(t, err)
			assert.Equal(t, originalType, configs2[name].Type)
			break
		}
	}
}

func TestEmbeddedConfigStore_LastModelProviderConfigsUpdate(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	updateTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)

	// Should return the constant embedded timestamp
	assert.Equal(t, embeddedTimestamp, updateTime)

	// Multiple calls should return the same timestamp
	updateTime2, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)
	assert.Equal(t, updateTime, updateTime2)
}

func TestEmbeddedConfigStore_GetAgentRoleConfigs(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	require.NotNil(t, configs)

	// Verify at least one role is loaded from conf/roles/
	assert.NotEmpty(t, configs)

	// Check for developer role (from conf/roles/developer/config.json)
	developer, ok := configs["developer"]
	if ok {
		assert.Equal(t, "developer", developer.Name)
		assert.NotEmpty(t, developer.Description)
		assert.NotNil(t, developer.VFSPrivileges)
		assert.NotNil(t, developer.ToolsAccess)
	}

	// Verify that returned configs are copies
	if len(configs) > 0 {
		for name, cfg := range configs {
			originalName := cfg.Name
			cfg.Name = "modified"
			configs2, err := store.GetAgentRoleConfigs()
			require.NoError(t, err)
			assert.Equal(t, originalName, configs2[name].Name)
			break
		}
	}

	// Verify deep copy of maps
	if developer != nil && len(developer.VFSPrivileges) > 0 {
		for path := range developer.VFSPrivileges {
			access := developer.VFSPrivileges[path]
			originalRead := access.Read
			// Change to a different value
			if originalRead == conf.AccessAllow {
				access.Read = conf.AccessDeny
			} else {
				access.Read = conf.AccessAllow
			}
			developer.VFSPrivileges[path] = access

			configs2, err := store.GetAgentRoleConfigs()
			require.NoError(t, err)
			developer2 := configs2["developer"]
			assert.Equal(t, originalRead, developer2.VFSPrivileges[path].Read)
			break
		}
	}
}

func TestEmbeddedConfigStore_LastAgentRoleConfigsUpdate(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	updateTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// Should return the constant embedded timestamp
	assert.Equal(t, embeddedTimestamp, updateTime)

	// Multiple calls should return the same timestamp
	updateTime2, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.Equal(t, updateTime, updateTime2)
}

func TestEmbeddedConfigStore_ConcurrentAccess(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Perform concurrent reads to verify thread safety
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, err := store.GetGlobalConfig()
				assert.NoError(t, err)
				_, err = store.GetModelProviderConfigs()
				assert.NoError(t, err)
				_, err = store.GetAgentRoleConfigs()
				assert.NoError(t, err)
				_, err = store.LastGlobalConfigUpdate()
				assert.NoError(t, err)
				_, err = store.LastModelProviderConfigsUpdate()
				assert.NoError(t, err)
				_, err = store.LastAgentRoleConfigsUpdate()
				assert.NoError(t, err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestEmbeddedConfigStore_ConstantTimestamp(t *testing.T) {
	// Verify that all timestamp methods return the same constant value
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	globalTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)

	modelTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)

	roleTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// All timestamps should be the same constant
	assert.Equal(t, globalTime, modelTime)
	assert.Equal(t, modelTime, roleTime)
	assert.Equal(t, embeddedTimestamp, globalTime)
}

func TestEmbeddedConfigStore_ReadOnly(t *testing.T) {
	// This test verifies that the embedded config store is truly read-only
	// by ensuring modifications to returned data don't affect the store

	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Test global config immutability
	config1, err := store.GetGlobalConfig()
	require.NoError(t, err)
	originalTagCount := len(config1.ModelTags)
	config1.ModelTags = append(config1.ModelTags, conf.ModelTagMapping{
		Model: "test", Tag: "test",
	})
	config2, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, originalTagCount, len(config2.ModelTags))

	// Test model provider configs immutability
	providers1, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	providers1["test-new"] = &conf.ModelProviderConfig{
		Name: "test-new",
		Type: "test",
	}
	providers2, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	_, exists := providers2["test-new"]
	assert.False(t, exists)

	// Test agent role configs immutability
	roles1, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	roles1["test-new-role"] = &conf.AgentRoleConfig{
		Name: "test-new-role",
	}
	roles2, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	_, exists = roles2["test-new-role"]
	assert.False(t, exists)
}

func TestEmbeddedConfigStore_MultipleInstances(t *testing.T) {
	// Verify that creating multiple instances works correctly
	store1, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	store2, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	// Both instances should return the same data
	config1, err := store1.GetGlobalConfig()
	require.NoError(t, err)

	config2, err := store2.GetGlobalConfig()
	require.NoError(t, err)

	assert.Equal(t, len(config1.ModelTags), len(config2.ModelTags))

	// Verify they are independent (modifying one doesn't affect the other)
	if len(config1.ModelTags) > 0 {
		config1.ModelTags[0].Tag = "modified"
		assert.NotEqual(t, config1.ModelTags[0].Tag, config2.ModelTags[0].Tag)
	}
}

func TestEmbeddedConfigStore_InterfaceCompliance(t *testing.T) {
	// Verify that EmbeddedConfigStore implements conf.ConfigStore interface
	var _ conf.ConfigStore = (*EmbeddedConfigStore)(nil)

	// Also verify through constructor
	var store conf.ConfigStore
	var err error
	store, err = NewEmbeddedConfigStore()
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestEmbeddedConfigStore_PromptFragments(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	require.NotEmpty(t, configs)

	// Check for developer role prompt fragments
	developer, ok := configs["developer"]
	if ok {
		assert.NotNil(t, developer.PromptFragments)
		assert.NotEmpty(t, developer.PromptFragments)

		// Check for 10-system.md fragment
		systemPrompt, hasSystem := developer.PromptFragments["10-system"]
		assert.True(t, hasSystem, "developer role should have 10-system prompt fragment")
		assert.NotEmpty(t, systemPrompt, "10-system prompt fragment should not be empty")
		assert.Contains(t, systemPrompt, "You are", "prompt should contain expected content")
	}

	// Verify that returned configs with fragments are copies
	if developer != nil && len(developer.PromptFragments) > 0 {
		for key := range developer.PromptFragments {
			originalValue := developer.PromptFragments[key]
			developer.PromptFragments[key] = "modified"

			configs2, err := store.GetAgentRoleConfigs()
			require.NoError(t, err)
			developer2 := configs2["developer"]
			assert.Equal(t, originalValue, developer2.PromptFragments[key])
			break
		}
	}
}

func TestEmbeddedConfigStore_EmptyPromptFragments(t *testing.T) {
	store, err := NewEmbeddedConfigStore()
	require.NoError(t, err)

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Check for all role (which should have no .md files)
	all, ok := configs["all"]
	if ok {
		// PromptFragments should be initialized but empty
		assert.NotNil(t, all.PromptFragments)
		assert.Empty(t, all.PromptFragments)
	}
}
