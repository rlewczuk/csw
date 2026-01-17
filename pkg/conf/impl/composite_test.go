package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string returns defaults",
			input:    "",
			expected: []string{"@DEFAULTS"},
		},
		{
			name:     "single path",
			input:    "/etc/csw/",
			expected: []string{"/etc/csw/"},
		},
		{
			name:     "multiple paths",
			input:    "@DEFAULTS:/etc/csw/:~/.config/csw/",
			expected: []string{"@DEFAULTS", "/etc/csw/", "~/.config/csw/"},
		},
		{
			name:     "paths with whitespace",
			input:    " @DEFAULTS : /etc/csw/ : ~/.config/csw/ ",
			expected: []string{"@DEFAULTS", "/etc/csw/", "~/.config/csw/"},
		},
		{
			name:     "paths with empty segments",
			input:    "@DEFAULTS::/etc/csw/::~/.config/csw/",
			expected: []string{"@DEFAULTS", "/etc/csw/", "~/.config/csw/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConfigPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	cwd, err := os.Getwd()
	require.NoError(t, err)

	projDir := "/project/root"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path unchanged",
			input:    "/etc/csw/",
			expected: "/etc/csw/",
		},
		{
			name:     "tilde expansion",
			input:    "~/.config/csw/",
			expected: filepath.Join(homeDir, ".config/csw/"),
		},
		{
			name:     "dot expansion",
			input:    "./config/",
			expected: filepath.Join(cwd, "config/"),
		},
		{
			name:     "project expansion",
			input:    "@PROJ/.csw/",
			expected: filepath.Join(projDir, ".csw/"),
		},
		{
			name:     "project expansion with nested path",
			input:    "@PROJ/config/csw/",
			expected: filepath.Join(projDir, "config/csw/"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.input, projDir)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompositeConfigStore_SingleSource(t *testing.T) {
	// Create composite store with only embedded defaults
	store, err := NewCompositeConfigStore("", "@DEFAULTS")
	require.NoError(t, err)
	require.NotNil(t, store)

	// Get global config
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	require.NotNil(t, globalConfig)

	// Get model provider configs
	modelConfigs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.NotNil(t, modelConfigs)

	// Get agent role configs
	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	require.NotNil(t, roleConfigs)
}

func TestCompositeConfigStore_MultipleSources_Merging(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "source1")
	dir2 := filepath.Join(tmpDir, "source2")

	require.NoError(t, os.MkdirAll(filepath.Join(dir1, "models"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "models"), 0755))

	// Create config in source1
	provider1 := `{
		"type": "ollama",
		"name": "provider1",
		"url": "http://localhost:11434",
		"description": "Provider 1 from source 1"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "models", "provider1.json"), []byte(provider1), 0644))

	provider2v1 := `{
		"type": "openai",
		"name": "provider2",
		"url": "https://api.openai.com/v1",
		"description": "Provider 2 from source 1"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "models", "provider2.json"), []byte(provider2v1), 0644))

	// Create config in source2 (overrides provider2)
	provider2v2 := `{
		"type": "openai",
		"name": "provider2",
		"url": "https://custom.openai.com/v1",
		"description": "Provider 2 from source 2 (overridden)"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "models", "provider2.json"), []byte(provider2v2), 0644))

	provider3 := `{
		"type": "anthropic",
		"name": "provider3",
		"url": "https://api.anthropic.com",
		"description": "Provider 3 from source 2"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "models", "provider3.json"), []byte(provider3), 0644))

	// Create composite store with both sources
	configPath := fmt.Sprintf("%s:%s", dir1, dir2)
	store, err := NewCompositeConfigStore(tmpDir, configPath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Get model provider configs
	modelConfigs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)

	// Verify merging: should have all 3 providers
	assert.Len(t, modelConfigs, 3)
	assert.Contains(t, modelConfigs, "provider1")
	assert.Contains(t, modelConfigs, "provider2")
	assert.Contains(t, modelConfigs, "provider3")

	// Verify provider2 was overridden by source2
	assert.Equal(t, "https://custom.openai.com/v1", modelConfigs["provider2"].URL)
	assert.Equal(t, "Provider 2 from source 2 (overridden)", modelConfigs["provider2"].Description)

	// Verify provider1 from source1
	assert.Equal(t, "http://localhost:11434", modelConfigs["provider1"].URL)

	// Verify provider3 from source2
	assert.Equal(t, "https://api.anthropic.com", modelConfigs["provider3"].URL)
}

func TestCompositeConfigStore_AgentRolesMerging(t *testing.T) {
	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "source1")
	dir2 := filepath.Join(tmpDir, "source2")

	// Create role1 in source1
	require.NoError(t, os.MkdirAll(filepath.Join(dir1, "roles", "role1"), 0755))
	role1v1 := `{
		"name": "role1",
		"description": "Role 1 from source 1",
		"vfs-privileges": {
			"/": {"read": "allow", "write": "deny", "delete": "deny", "list": "allow", "find": "allow", "move": "deny"}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "roles", "role1", "config.json"), []byte(role1v1), 0644))

	// Create role1 in source2 (overrides)
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "roles", "role1"), 0755))
	role1v2 := `{
		"name": "role1",
		"description": "Role 1 from source 2 (overridden)",
		"vfs-privileges": {
			"/": {"read": "allow", "write": "allow", "delete": "allow", "list": "allow", "find": "allow", "move": "allow"}
		}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "roles", "role1", "config.json"), []byte(role1v2), 0644))

	// Create role2 in source2
	require.NoError(t, os.MkdirAll(filepath.Join(dir2, "roles", "role2"), 0755))
	role2 := `{
		"name": "role2",
		"description": "Role 2 from source 2"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "roles", "role2", "config.json"), []byte(role2), 0644))

	// Create composite store
	configPath := fmt.Sprintf("%s:%s", dir1, dir2)
	store, err := NewCompositeConfigStore(tmpDir, configPath)
	require.NoError(t, err)

	// Get agent role configs
	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	// Verify merging
	assert.Len(t, roleConfigs, 2)
	assert.Contains(t, roleConfigs, "role1")
	assert.Contains(t, roleConfigs, "role2")

	// Verify role1 was overridden by source2
	assert.Equal(t, "Role 1 from source 2 (overridden)", roleConfigs["role1"].Description)
	assert.Equal(t, conf.AccessAllow, roleConfigs["role1"].VFSPrivileges["/"].Write)
}

func TestCompositeConfigStore_GlobalConfigMerging(t *testing.T) {
	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "source1")
	dir2 := filepath.Join(tmpDir, "source2")

	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))

	// Create global config in source1
	global1 := `{
		"model_tags": [
			{"model": "gpt-.*", "tag": "openai"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "global.json"), []byte(global1), 0644))

	// Create global config in source2
	global2 := `{
		"model_tags": [
			{"model": "claude-.*", "tag": "anthropic"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "global.json"), []byte(global2), 0644))

	// Create composite store
	configPath := fmt.Sprintf("%s:%s", dir1, dir2)
	store, err := NewCompositeConfigStore(tmpDir, configPath)
	require.NoError(t, err)

	// Get global config
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)

	// Verify model tags are additive (both should be present)
	assert.Len(t, globalConfig.ModelTags, 2)
	assert.Equal(t, "gpt-.*", globalConfig.ModelTags[0].Model)
	assert.Equal(t, "openai", globalConfig.ModelTags[0].Tag)
	assert.Equal(t, "claude-.*", globalConfig.ModelTags[1].Model)
	assert.Equal(t, "anthropic", globalConfig.ModelTags[1].Tag)
}

func TestCompositeConfigStore_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "models"), 0755))

	// Create initial config
	provider := `{
		"type": "ollama",
		"name": "test",
		"url": "http://localhost:11434"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "models", "test.json"), []byte(provider), 0644))

	// Create composite store
	store, err := NewCompositeConfigStore(tmpDir, configDir)
	require.NoError(t, err)

	composite, ok := store.(*CompositeConfigStore)
	require.True(t, ok)

	// Get configs (should load from disk)
	configs1, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs1, 1)
	assert.Equal(t, "http://localhost:11434", configs1["test"].URL)

	// Record the update timestamp
	updateTime1 := composite.modelProviderConfigsUpdate

	// Get configs again (should use cache, no disk read)
	configs2, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Equal(t, configs1, configs2)

	// Update timestamp should be the same (used cache)
	assert.Equal(t, updateTime1, composite.modelProviderConfigsUpdate)
}

func TestCompositeConfigStore_ChangeDetection(t *testing.T) {
	// Create mock stores
	store1 := NewMockConfigStore()
	store2 := NewMockConfigStore()

	// Set initial configs
	store1.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"provider1": {
			Type: "ollama",
			Name: "provider1",
			URL:  "http://localhost:11434",
		},
	})

	store2.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"provider2": {
			Type: "openai",
			Name: "provider2",
			URL:  "https://api.openai.com/v1",
		},
	})

	// Create composite store manually
	composite := &CompositeConfigStore{
		stores:             []conf.ConfigStore{store1, store2},
		storeGlobalUpdates: make([]time.Time, 2),
		storeModelUpdates:  make([]time.Time, 2),
		storeRoleUpdates:   make([]time.Time, 2),
	}

	// Initial refresh
	require.NoError(t, composite.refresh())

	// Should have both providers
	configs, err := composite.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Record initial timestamps
	initialUpdate := composite.modelProviderConfigsUpdate

	// Wait a bit to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Update store2's config
	store2.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"provider2": {
			Type: "openai",
			Name: "provider2",
			URL:  "https://custom.openai.com/v1", // Changed URL
		},
	})

	// Get configs again - should detect change and refresh
	configs, err = composite.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 2)
	assert.Equal(t, "https://custom.openai.com/v1", configs["provider2"].URL)

	// Timestamp should be updated
	assert.True(t, composite.modelProviderConfigsUpdate.After(initialUpdate))
}

func TestCompositeConfigStore_NonExistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "does-not-exist")

	// Should succeed but skip the non-existent path
	configPath := fmt.Sprintf("@DEFAULTS:%s", nonExistentDir)
	store, err := NewCompositeConfigStore(tmpDir, configPath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Should still work with just defaults
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.NotNil(t, configs)
}

func TestCompositeConfigStore_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file instead of directory
	filePath := filepath.Join(tmpDir, "notadir")
	require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

	// Should fail because path is not a directory
	configPath := fmt.Sprintf("@DEFAULTS:%s", filePath)
	_, err := NewCompositeConfigStore(tmpDir, configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory")
}

func TestCompositeConfigStore_EmptyConfigPath(t *testing.T) {
	// Empty config path should default to @DEFAULTS
	store, err := NewCompositeConfigStore("", "")
	require.NoError(t, err)
	require.NotNil(t, store)

	// Should work with embedded defaults
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	require.NotNil(t, configs)
}

func TestCompositeConfigStore_LastUpdateTimestamps(t *testing.T) {
	// Create mock stores
	store1 := NewMockConfigStore()
	store2 := NewMockConfigStore()

	// Create composite
	composite := &CompositeConfigStore{
		stores:             []conf.ConfigStore{store1, store2},
		storeGlobalUpdates: make([]time.Time, 2),
		storeModelUpdates:  make([]time.Time, 2),
		storeRoleUpdates:   make([]time.Time, 2),
	}

	// Initial refresh
	require.NoError(t, composite.refresh())

	// Get timestamps
	globalUpdate, err := composite.LastGlobalConfigUpdate()
	require.NoError(t, err)

	modelUpdate, err := composite.LastModelProviderConfigsUpdate()
	require.NoError(t, err)

	roleUpdate, err := composite.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// All should be valid timestamps
	assert.False(t, globalUpdate.IsZero())
	assert.False(t, modelUpdate.IsZero())
	assert.False(t, roleUpdate.IsZero())
}

func TestCompositeConfigStore_CopyProtection(t *testing.T) {
	// Create mock store with config
	mockStore := NewMockConfigStore()
	mockStore.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"test": {
			Type:      "ollama",
			Name:      "test",
			URL:       "http://localhost:11434",
			ModelTags: []conf.ModelTagMapping{{Model: ".*", Tag: "test"}},
		},
	})

	composite := &CompositeConfigStore{
		stores:             []conf.ConfigStore{mockStore},
		storeGlobalUpdates: make([]time.Time, 1),
		storeModelUpdates:  make([]time.Time, 1),
		storeRoleUpdates:   make([]time.Time, 1),
	}

	require.NoError(t, composite.refresh())

	// Get config
	configs, err := composite.GetModelProviderConfigs()
	require.NoError(t, err)

	// Modify returned config
	configs["test"].URL = "modified"
	configs["test"].ModelTags[0].Tag = "modified"

	// Get config again - should not be affected by previous modification
	configs2, err := composite.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", configs2["test"].URL)
	assert.Equal(t, "test", configs2["test"].ModelTags[0].Tag)
}

func TestCompositeConfigStore_MultipleRefreshes(t *testing.T) {
	mockStore := NewMockConfigStore()

	composite := &CompositeConfigStore{
		stores:             []conf.ConfigStore{mockStore},
		storeGlobalUpdates: make([]time.Time, 1),
		storeModelUpdates:  make([]time.Time, 1),
		storeRoleUpdates:   make([]time.Time, 1),
	}

	// Multiple refreshes should work without error
	for i := 0; i < 5; i++ {
		require.NoError(t, composite.refresh())
	}
}

func TestCompositeConfigStore_ConcurrentAccess(t *testing.T) {
	mockStore := NewMockConfigStore()
	mockStore.SetModelProviderConfigs(map[string]*conf.ModelProviderConfig{
		"test": {Type: "ollama", Name: "test", URL: "http://localhost:11434"},
	})

	composite := &CompositeConfigStore{
		stores:             []conf.ConfigStore{mockStore},
		storeGlobalUpdates: make([]time.Time, 1),
		storeModelUpdates:  make([]time.Time, 1),
		storeRoleUpdates:   make([]time.Time, 1),
	}

	require.NoError(t, composite.refresh())

	// Concurrent reads should be safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := composite.GetModelProviderConfigs()
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
