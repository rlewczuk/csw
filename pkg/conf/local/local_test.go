package local

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalConfigStore_InvalidDirectory(t *testing.T) {
	t.Run("non-existent directory", func(t *testing.T) {
		store, err := NewLocalConfigStore("/non/existent/path")
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "invalid config directory")
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-file-*")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		store, err := NewLocalConfigStore(tmpFile.Name())
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

func TestNewLocalConfigStore_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	// Verify empty configurations
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.NotNil(t, globalConfig)
	assert.Empty(t, globalConfig.ModelTags)

	modelConfigs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Empty(t, modelConfigs)

	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Empty(t, roleConfigs)
}

func TestLocalConfigStore_GlobalConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create global.json
	globalConfig := conf.GlobalConfig{
		ModelTags: []conf.ModelTagMapping{
			{Model: "^claude-.*", Tag: "anthropic"},
			{Model: "^gpt-.*", Tag: "openai"},
		},
	}
	globalData, err := json.MarshalIndent(globalConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "global.json"), globalData, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetGlobalConfig
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 2)
	assert.Equal(t, "^claude-.*", config.ModelTags[0].Model)
	assert.Equal(t, "anthropic", config.ModelTags[0].Tag)

	// Test LastGlobalConfigUpdate
	updateTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)
	assert.False(t, updateTime.IsZero())

	// Test that returned config is a copy (modification doesn't affect cached data)
	config.ModelTags[0].Tag = "modified"
	config2, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "anthropic", config2.ModelTags[0].Tag)
}

func TestLocalConfigStore_ModelProviderConfigs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create model provider configs
	anthropicConfig := conf.ModelProviderConfig{
		Type:        "anthropic",
		Name:        "anthropic",
		Description: "Anthropic Claude API",
		URL:         "https://api.anthropic.com",
		APIKey:      "test-key",
		ModelTags: []conf.ModelTagMapping{
			{Model: "^claude-3-opus.*", Tag: "opus"},
		},
	}
	anthropicData, err := json.MarshalIndent(anthropicConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modelsDir, "anthropic.json"), anthropicData, 0644)
	require.NoError(t, err)

	openaiConfig := conf.ModelProviderConfig{
		Type: "openai",
		Name: "openai",
		URL:  "https://api.openai.com",
	}
	openaiData, err := json.MarshalIndent(openaiConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modelsDir, "openai.json"), openaiData, 0644)
	require.NoError(t, err)

	// Create a non-JSON file that should be ignored
	err = os.WriteFile(filepath.Join(modelsDir, "readme.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetModelProviderConfigs
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	anthropic, ok := configs["anthropic"]
	require.True(t, ok)
	assert.Equal(t, "anthropic", anthropic.Type)
	assert.Equal(t, "Anthropic Claude API", anthropic.Description)
	assert.Len(t, anthropic.ModelTags, 1)

	openai, ok := configs["openai"]
	require.True(t, ok)
	assert.Equal(t, "openai", openai.Type)

	// Test LastModelProviderConfigsUpdate
	updateTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)
	assert.False(t, updateTime.IsZero())

	// Test that returned configs are copies
	anthropic.Type = "modified"
	configs2, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Equal(t, "anthropic", configs2["anthropic"].Type)
}

func TestLocalConfigStore_AgentRoleConfigs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create test1 role
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "Test role 1",
		VFSPrivileges: map[string]conf.FileAccess{
			"/tmp": {Read: conf.AccessAllow, Write: conf.AccessDeny},
		},
		ToolsAccess: map[string]conf.AccessFlag{
			"read":  conf.AccessAllow,
			"write": conf.AccessAllow,
		},
		RunPrivileges: map[string]conf.AccessFlag{
			".*": conf.AccessAsk,
		},
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	// Create test2 role
	test2Dir := filepath.Join(rolesDir, "test2")
	require.NoError(t, os.Mkdir(test2Dir, 0755))
	test2Config := conf.AgentRoleConfig{
		Name:        "test2",
		Description: "Test role 2",
		ToolsAccess: map[string]conf.AccessFlag{
			"bash": conf.AccessDeny,
		},
	}
	test2Data, err := json.MarshalIndent(test2Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test2Dir, "config.json"), test2Data, 0644)
	require.NoError(t, err)

	// Create a non-directory file that should be ignored
	err = os.WriteFile(filepath.Join(rolesDir, "readme.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetAgentRoleConfigs
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	test1, ok := configs["test1"]
	require.True(t, ok)
	assert.Equal(t, "test1", test1.Name)
	assert.Equal(t, "Test role 1", test1.Description)
	assert.Len(t, test1.VFSPrivileges, 1)
	assert.Equal(t, conf.AccessAllow, test1.VFSPrivileges["/tmp"].Read)
	assert.Len(t, test1.ToolsAccess, 2)
	assert.Len(t, test1.RunPrivileges, 1)

	test2, ok := configs["test2"]
	require.True(t, ok)
	assert.Equal(t, "test2", test2.Name)
	assert.Len(t, test2.ToolsAccess, 1)
	assert.Equal(t, conf.AccessDeny, test2.ToolsAccess["bash"])

	// Test LastAgentRoleConfigsUpdate
	updateTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.False(t, updateTime.IsZero())

	// Test that returned configs are copies
	test1.Name = "modified"
	configs2, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Equal(t, "test1", configs2["test1"].Name)
}

func TestLocalConfigStore_MissingRoleConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create a role directory without config.json
	invalidRoleDir := filepath.Join(rolesDir, "invalid")
	require.NoError(t, os.Mkdir(invalidRoleDir, 0755))

	store, err := NewLocalConfigStore(tmpDir)
	assert.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "missing config.json")
}

func TestLocalConfigStore_InvalidJSON(t *testing.T) {
	t.Run("invalid global.json", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test-config-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		err = os.WriteFile(filepath.Join(tmpDir, "global.json"), []byte("invalid json"), 0644)
		require.NoError(t, err)

		store, err := NewLocalConfigStore(tmpDir)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("invalid model provider config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test-config-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		modelsDir := filepath.Join(tmpDir, "models")
		require.NoError(t, os.Mkdir(modelsDir, 0755))

		err = os.WriteFile(filepath.Join(modelsDir, "invalid.json"), []byte("invalid json"), 0644)
		require.NoError(t, err)

		store, err := NewLocalConfigStore(tmpDir)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("invalid role config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test-config-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		rolesDir := filepath.Join(tmpDir, "roles")
		require.NoError(t, os.Mkdir(rolesDir, 0755))

		roleDir := filepath.Join(rolesDir, "test")
		require.NoError(t, os.Mkdir(roleDir, 0755))

		err = os.WriteFile(filepath.Join(roleDir, "config.json"), []byte("invalid json"), 0644)
		require.NoError(t, err)

		store, err := NewLocalConfigStore(tmpDir)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestLocalConfigStore_EmptyName(t *testing.T) {
	t.Run("model provider with empty name uses filename", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test-config-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		modelsDir := filepath.Join(tmpDir, "models")
		require.NoError(t, os.Mkdir(modelsDir, 0755))

		config := conf.ModelProviderConfig{
			Type: "test",
			Name: "", // Empty name - should use filename
		}
		data, err := json.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(modelsDir, "mytest.json"), data, 0644)
		require.NoError(t, err)

		store, err := NewLocalConfigStore(tmpDir)
		require.NoError(t, err)
		defer store.Close()

		configs, err := store.GetModelProviderConfigs()
		require.NoError(t, err)

		provider, ok := configs["mytest"]
		assert.True(t, ok)
		assert.Equal(t, "mytest", provider.Name)
	})

	t.Run("role with empty name uses directory name", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test-config-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		rolesDir := filepath.Join(tmpDir, "roles")
		require.NoError(t, os.Mkdir(rolesDir, 0755))

		roleDir := filepath.Join(rolesDir, "myrole")
		require.NoError(t, os.Mkdir(roleDir, 0755))

		config := conf.AgentRoleConfig{
			Name: "", // Empty name - should use directory name
		}
		data, err := json.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(roleDir, "config.json"), data, 0644)
		require.NoError(t, err)

		store, err := NewLocalConfigStore(tmpDir)
		require.NoError(t, err)
		defer store.Close()

		configs, err := store.GetAgentRoleConfigs()
		require.NoError(t, err)

		role, ok := configs["myrole"]
		assert.True(t, ok)
		assert.Equal(t, "myrole", role.Name)
	})
}

func TestLocalConfigStore_FileWatching_GlobalConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create initial global.json
	globalConfig := conf.GlobalConfig{
		ModelTags: []conf.ModelTagMapping{
			{Model: "^claude-.*", Tag: "anthropic"},
		},
	}
	globalData, err := json.MarshalIndent(globalConfig, "", "  ")
	require.NoError(t, err)
	globalPath := filepath.Join(tmpDir, "global.json")
	err = os.WriteFile(globalPath, globalData, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get initial timestamp
	initialTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)

	// Wait a bit to ensure timestamp difference
	time.Sleep(100 * time.Millisecond)

	// Modify global.json
	globalConfig.ModelTags = append(globalConfig.ModelTags, conf.ModelTagMapping{
		Model: "^gpt-.*", Tag: "openai",
	})
	globalData, err = json.MarshalIndent(globalConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(globalPath, globalData, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(200 * time.Millisecond)

	// Verify config was reloaded
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 2)
	assert.Equal(t, "openai", config.ModelTags[1].Tag)

	// Verify timestamp was updated
	newTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}

func TestLocalConfigStore_FileWatching_ModelProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create initial model provider
	anthropicConfig := conf.ModelProviderConfig{
		Type: "anthropic",
		Name: "anthropic",
		URL:  "https://api.anthropic.com",
	}
	anthropicData, err := json.MarshalIndent(anthropicConfig, "", "  ")
	require.NoError(t, err)
	anthropicPath := filepath.Join(modelsDir, "anthropic.json")
	err = os.WriteFile(anthropicPath, anthropicData, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get initial timestamp
	initialTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)

	// Wait a bit to ensure timestamp difference
	time.Sleep(100 * time.Millisecond)

	// Add a new model provider
	openaiConfig := conf.ModelProviderConfig{
		Type: "openai",
		Name: "openai",
		URL:  "https://api.openai.com",
	}
	openaiData, err := json.MarshalIndent(openaiConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modelsDir, "openai.json"), openaiData, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(200 * time.Millisecond)

	// Verify config was reloaded
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	_, ok := configs["openai"]
	assert.True(t, ok)

	// Verify timestamp was updated
	newTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}

func TestLocalConfigStore_FileWatching_AgentRole(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create initial role
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "Test role 1",
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	test1Path := filepath.Join(test1Dir, "config.json")
	err = os.WriteFile(test1Path, test1Data, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get initial timestamp
	initialTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// Wait a bit to ensure timestamp difference
	time.Sleep(100 * time.Millisecond)

	// Modify existing role
	test1Config.Description = "Modified description"
	test1Data, err = json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(test1Path, test1Data, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(200 * time.Millisecond)

	// Verify config was reloaded
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Equal(t, "Modified description", configs["test1"].Description)

	// Verify timestamp was updated
	newTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}

func TestLocalConfigStore_FileWatching_NewRole(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Verify no roles initially
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Empty(t, configs)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Add a new role directory
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))

	// Wait for directory creation to be detected
	time.Sleep(200 * time.Millisecond)

	// Add config.json to the new role
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "New test role",
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(200 * time.Millisecond)

	// Verify config was loaded
	configs, err = store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	test1, ok := configs["test1"]
	assert.True(t, ok)
	assert.Equal(t, "New test role", test1.Description)
}

func TestLocalConfigStore_Close(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)

	// Close should not return error
	err = store.Close()
	assert.NoError(t, err)

	// Calling Close again should not panic
	err = store.Close()
	assert.NoError(t, err)
}

func TestLocalConfigStore_ConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test config
	globalConfig := conf.GlobalConfig{
		ModelTags: []conf.ModelTagMapping{
			{Model: "^claude-.*", Tag: "anthropic"},
		},
	}
	globalData, err := json.MarshalIndent(globalConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "global.json"), globalData, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Perform concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, err := store.GetGlobalConfig()
				assert.NoError(t, err)
				_, err = store.LastGlobalConfigUpdate()
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

func TestLocalConfigStore_WithTestdataConfig(t *testing.T) {
	// Test with actual testdata configuration
	configDir := filepath.Join("..", "..", "..", "testdata", "conf")

	// Check if testdata exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Skip("testdata/conf directory not found")
		return
	}

	store, err := NewLocalConfigStore(configDir)
	require.NoError(t, err)
	defer store.Close()

	// Test global config
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.NotNil(t, globalConfig)
	assert.NotEmpty(t, globalConfig.ModelTags)

	// Test model provider configs
	modelConfigs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.NotEmpty(t, modelConfigs)

	// Verify specific providers exist
	_, hasAnthropic := modelConfigs["anthropic"]
	assert.True(t, hasAnthropic, "anthropic provider should exist")

	// Test agent role configs
	roleConfigs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.NotEmpty(t, roleConfigs)

	// Verify specific roles exist
	_, hasTest1 := roleConfigs["test1"]
	assert.True(t, hasTest1, "test1 role should exist")

	// Test timestamps
	globalTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)
	assert.False(t, globalTime.IsZero())

	modelTime, err := store.LastModelProviderConfigsUpdate()
	require.NoError(t, err)
	assert.False(t, modelTime.IsZero())

	roleTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.False(t, roleTime.IsZero())
}
