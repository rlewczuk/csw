package impl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
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
		ContextCompactionThreshold: 0.8,
		Container: conf.ContainerConfig{
			Enabled: true,
			Image:   "busybox:latest",
			Mounts:  []string{"/tmp:/mnt/tmp"},
			Env:     []string{"TEST_ENV=value"},
		},
		ModelTags: []conf.ModelTagMapping{
			{Model: "^claude-.*", Tag: "anthropic"},
			{Model: "^gpt-.*", Tag: "openai"},
		},
		ToolSelection: conf.ToolSelectionConfig{
			Default: map[string]bool{
				"runBash": false,
			},
			Tags: map[string]map[string]bool{
				"safe": {
					"vfsRead": true,
				},
			},
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
	assert.Equal(t, 0.8, config.ContextCompactionThreshold)
	assert.Equal(t, "^claude-.*", config.ModelTags[0].Model)
	assert.Equal(t, "anthropic", config.ModelTags[0].Tag)
	assert.Equal(t, false, config.ToolSelection.Default["runBash"])
	assert.True(t, config.Container.Enabled)
	assert.Equal(t, "busybox:latest", config.Container.Image)
	assert.Equal(t, []string{"/tmp:/mnt/tmp"}, config.Container.Mounts)
	assert.Equal(t, []string{"TEST_ENV=value"}, config.Container.Env)
	safeRule, exists := config.ToolSelection.Tags["safe"]
	require.True(t, exists)
	assert.True(t, safeRule["vfsRead"])

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

func TestLocalConfigStore_ModelTemplateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "models", "families"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "models", "vendors"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "models", "templates"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "models", "families", "openai-gpt.yaml"), []byte("max_tokens: 4096\nmodel_tags:\n  - model: '^gpt'\n    tag: 'openai'\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "models", "vendors", "openai.yaml"), []byte("type: openai\nurl: https://api.openai.com/v1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "models", "templates", "openai.yaml"), []byte("gpt-4.1:\n  family: openai-gpt\n  max_tokens: 8192\n"), 0644))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	global, err := store.GetGlobalConfig()
	require.NoError(t, err)

	_, hasFamily := global.ModelFamilies["openai-gpt"]
	_, hasVendor := global.ModelVendors["openai"]
	_, hasTemplateGroup := global.ModelTemplates["openai"]
	require.True(t, hasFamily)
	require.True(t, hasVendor)
	require.True(t, hasTemplateGroup)
	assert.Equal(t, 4096, global.ModelFamilies["openai-gpt"].MaxTokens)
	assert.Equal(t, "openai", global.ModelVendors["openai"].Type)
	assert.Equal(t, 8192, global.ModelTemplates["openai"]["gpt-4.1"].MaxTokens)
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

func TestLocalConfigStore_HookConfigs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "merge.yml"), []byte("name: merge-hook\nhook: merge\ncommand: echo merge\nrun-on: host\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "summary.json"), []byte(`{"name":"summary-hook","hook":"summary","command":"echo summary"}`), 0644))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	hooks, err := store.GetHookConfigs()
	require.NoError(t, err)
	require.Len(t, hooks, 2)
	require.Contains(t, hooks, "merge-hook")
	require.Contains(t, hooks, "summary-hook")
	assert.Equal(t, conf.HookRunOnHost, hooks["merge-hook"].RunOn)
	assert.Equal(t, conf.HookTypeShell, hooks["summary-hook"].Type)

	updateTime, err := store.LastHookConfigsUpdate()
	require.NoError(t, err)
	assert.False(t, updateTime.IsZero())
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

func TestLocalConfigStore_GetAgentConfigFile(t *testing.T) {
	t.Run("reads file from agent namespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "agent", "commit"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "agent", "commit", "system.md"), []byte("local-system"), 0o644))

		store, err := NewLocalConfigStore(tmpDir)
		require.NoError(t, err)
		defer store.Close()

		data, err := store.GetAgentConfigFile("commit", "system.md")
		require.NoError(t, err)
		assert.Equal(t, "local-system", string(data))
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewLocalConfigStore(tmpDir)
		require.NoError(t, err)
		defer store.Close()

		_, err = store.GetAgentConfigFile("commit", "missing.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read")
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
	time.Sleep(10 * time.Millisecond)

	// Modify global.json
	globalConfig.ModelTags = append(globalConfig.ModelTags, conf.ModelTagMapping{
		Model: "^gpt-.*", Tag: "openai",
	})
	globalData, err = json.MarshalIndent(globalConfig, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(globalPath, globalData, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(20 * time.Millisecond)

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
	time.Sleep(10 * time.Millisecond)

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
	time.Sleep(20 * time.Millisecond)

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
	time.Sleep(10 * time.Millisecond)

	// Modify existing role
	test1Config.Description = "Modified description"
	test1Data, err = json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(test1Path, test1Data, 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(20 * time.Millisecond)

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
	time.Sleep(10 * time.Millisecond)

	// Add a new role directory
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))

	// Wait for directory creation to be detected
	time.Sleep(10 * time.Millisecond)

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
	time.Sleep(10 * time.Millisecond)

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

func TestLocalConfigStore_PromptFragments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create test1 role with prompt fragments
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "Test role 1",
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	// Create prompt fragment files
	err = os.WriteFile(filepath.Join(test1Dir, "10-system.md"), []byte("You are a helpful assistant."), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "20-context.md"), []byte("This is context information."), 0644)
	require.NoError(t, err)

	// Create a non-md file that should be ignored
	err = os.WriteFile(filepath.Join(test1Dir, "readme.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetAgentRoleConfigs
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	test1, ok := configs["test1"]
	require.True(t, ok)

	// Verify prompt fragments were loaded
	assert.NotNil(t, test1.PromptFragments)
	assert.Len(t, test1.PromptFragments, 2)

	systemPrompt, hasSystem := test1.PromptFragments["10-system"]
	assert.True(t, hasSystem)
	assert.Equal(t, "You are a helpful assistant.", systemPrompt)

	contextPrompt, hasContext := test1.PromptFragments["20-context"]
	assert.True(t, hasContext)
	assert.Equal(t, "This is context information.", contextPrompt)

	// Verify readme.txt was ignored
	_, hasReadme := test1.PromptFragments["readme"]
	assert.False(t, hasReadme)
}

func TestLocalConfigStore_PromptFragments_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create test1 role without any .md files
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "Test role 1",
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)

	test1, ok := configs["test1"]
	require.True(t, ok)

	// Verify prompt fragments is empty but not nil
	assert.NotNil(t, test1.PromptFragments)
	assert.Empty(t, test1.PromptFragments)
}

func TestLocalConfigStore_ToolFragments(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "roles", "all"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "roles", "all", "10-system.md"), []byte("all prompt"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "tools", "myTool"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "tools", "myTool", "myTool.md"), []byte("desc"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "tools", "myTool", "myTool.schema.json"), []byte(`{"type":"object"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "tools", "myTool", "myTool.yaml"), []byte("command: echo hi\n"), 0o644))

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	roles, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	allRole, ok := roles["all"]
	require.True(t, ok)
	require.NotNil(t, allRole.ToolFragments)

	assert.Equal(t, "desc", allRole.ToolFragments["myTool/myTool.md"])
	assert.Equal(t, `{"type":"object"}`, allRole.ToolFragments["myTool/myTool.schema.json"])
	assert.Equal(t, "command: echo hi\n", allRole.ToolFragments["myTool/myTool.yaml"])
	assert.Equal(t, filepath.Join(tmpDir, "tools", "myTool"), allRole.ToolFragments["myTool/.tooldir"])
}

func TestLocalConfigStore_FileWatching_PromptFragments(t *testing.T) {
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
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	// Create initial prompt fragment
	err = os.WriteFile(filepath.Join(test1Dir, "10-system.md"), []byte("Initial prompt."), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get initial timestamp
	initialTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// Verify initial prompt fragment
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1, ok := configs["test1"]
	require.True(t, ok)
	assert.Equal(t, "Initial prompt.", test1.PromptFragments["10-system"])

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Modify the prompt fragment
	err = os.WriteFile(filepath.Join(test1Dir, "10-system.md"), []byte("Updated prompt."), 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(20 * time.Millisecond)

	// Verify prompt fragment was reloaded
	configs, err = store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1, ok = configs["test1"]
	require.True(t, ok)
	assert.Equal(t, "Updated prompt.", test1.PromptFragments["10-system"])

	// Verify timestamp was updated
	newTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}

func TestLocalConfigStore_FileWatching_NewPromptFragment(t *testing.T) {
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
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Verify no prompt fragments initially
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1, ok := configs["test1"]
	require.True(t, ok)
	assert.Empty(t, test1.PromptFragments)

	// Get initial timestamp
	initialTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Add a new prompt fragment
	err = os.WriteFile(filepath.Join(test1Dir, "10-system.md"), []byte("New prompt fragment."), 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(20 * time.Millisecond)

	// Verify prompt fragment was loaded
	configs, err = store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1, ok = configs["test1"]
	require.True(t, ok)
	assert.Len(t, test1.PromptFragments, 1)
	assert.Equal(t, "New prompt fragment.", test1.PromptFragments["10-system"])

	// Verify timestamp was updated
	newTime, err := store.LastAgentRoleConfigsUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}

func TestLocalConfigStore_PromptFragments_Copy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create test1 role with prompt fragments
	test1Dir := filepath.Join(rolesDir, "test1")
	require.NoError(t, os.Mkdir(test1Dir, 0755))
	test1Config := conf.AgentRoleConfig{
		Name:        "test1",
		Description: "Test role 1",
	}
	test1Data, err := json.MarshalIndent(test1Config, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(test1Dir, "config.json"), test1Data, 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(test1Dir, "10-system.md"), []byte("Original prompt."), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get configs and modify prompt fragments
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1, ok := configs["test1"]
	require.True(t, ok)
	assert.Equal(t, "Original prompt.", test1.PromptFragments["10-system"])

	// Modify the returned config
	test1.PromptFragments["10-system"] = "Modified prompt."

	// Get configs again and verify it wasn't modified
	configs2, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	test1_2, ok := configs2["test1"]
	require.True(t, ok)
	assert.Equal(t, "Original prompt.", test1_2.PromptFragments["10-system"])
}

func TestLocalConfigStore_YAMLGlobalConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create global.yml
	globalYAML := `model_tags:
  - model: "^claude-.*"
    tag: "anthropic"
default_provider: "test-provider"
default_role: "test-role"
`
	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetGlobalConfig
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 1)
	assert.Equal(t, "^claude-.*", config.ModelTags[0].Model)
	assert.Equal(t, "anthropic", config.ModelTags[0].Tag)
	assert.Equal(t, "test-provider", config.DefaultProvider)
	assert.Equal(t, "test-role", config.DefaultRole)
}

func TestLocalConfigStore_YAMLPrecedence_Global(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create both global.yml and global.json
	globalYAML := `model_tags:
  - model: "^gpt-.*"
    tag: "openai"
default_provider: "yaml-provider"
`
	globalJSON := `{
  "model_tags": [
    {"model": "^claude-.*", "tag": "anthropic"}
  ],
  "default_provider": "json-provider"
}`

	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "global.json"), []byte(globalJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 1)
	assert.Equal(t, "^gpt-.*", config.ModelTags[0].Model)
	assert.Equal(t, "openai", config.ModelTags[0].Tag)
	assert.Equal(t, "yaml-provider", config.DefaultProvider)
}

func TestLocalConfigStore_YAMLModelProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create YAML model provider config
	providerYAML := `type: openai
name: openai-yaml
description: OpenAI via YAML
url: https://api.openai.com/v1
api_key: yaml-key
`
	err = os.WriteFile(filepath.Join(modelsDir, "openai.yml"), []byte(providerYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetModelProviderConfigs
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	openai, ok := configs["openai-yaml"]
	require.True(t, ok)
	assert.Equal(t, "openai", openai.Type)
	assert.Equal(t, "OpenAI via YAML", openai.Description)
	assert.Equal(t, "https://api.openai.com/v1", openai.URL)
	assert.Equal(t, "yaml-key", openai.APIKey)
}

func TestLocalConfigStore_YAMLPrecedence_ModelProvider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.Mkdir(modelsDir, 0755))

	// Create both YAML and JSON for the same provider
	providerYAML := `type: openai
name: test-provider
description: YAML version
url: https://yaml.example.com
`
	providerJSON := `{
  "type": "anthropic",
  "name": "test-provider",
  "description": "JSON version",
  "url": "https://json.example.com"
}`

	err = os.WriteFile(filepath.Join(modelsDir, "test-provider.yml"), []byte(providerYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modelsDir, "test-provider.json"), []byte(providerJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	provider, ok := configs["test-provider"]
	require.True(t, ok)
	assert.Equal(t, "openai", provider.Type)
	assert.Equal(t, "YAML version", provider.Description)
	assert.Equal(t, "https://yaml.example.com", provider.URL)
}

func TestLocalConfigStore_YAMLRoleConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create YAML role config
	roleDir := filepath.Join(rolesDir, "yaml-role")
	require.NoError(t, os.Mkdir(roleDir, 0755))

	roleYAML := `name: yaml-role
description: A role defined in YAML
vfs-privileges:
  "**":
    read: allow
    write: ask
tools-access:
  read: allow
  write: deny
`
	err = os.WriteFile(filepath.Join(roleDir, "config.yml"), []byte(roleYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Test GetAgentRoleConfigs
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	role, ok := configs["yaml-role"]
	require.True(t, ok)
	assert.Equal(t, "yaml-role", role.Name)
	assert.Equal(t, "A role defined in YAML", role.Description)
	assert.Equal(t, conf.AccessAllow, role.VFSPrivileges["**"].Read)
	assert.Equal(t, conf.AccessAsk, role.VFSPrivileges["**"].Write)
	assert.Equal(t, conf.AccessAllow, role.ToolsAccess["read"])
	assert.Equal(t, conf.AccessDeny, role.ToolsAccess["write"])
}

func TestLocalConfigStore_YAMLPrecedence_RoleConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rolesDir := filepath.Join(tmpDir, "roles")
	require.NoError(t, os.Mkdir(rolesDir, 0755))

	// Create both YAML and JSON config for the same role
	roleDir := filepath.Join(rolesDir, "test-role")
	require.NoError(t, os.Mkdir(roleDir, 0755))

	roleYAML := `name: test-role
description: YAML version
vfs-privileges: {}
tools-access:
  read: allow
`
	roleJSON := `{
  "name": "test-role",
  "description": "JSON version",
  "vfs-privileges": {},
  "tools-access": {
    "read": "deny"
  }
}`

	err = os.WriteFile(filepath.Join(roleDir, "config.yml"), []byte(roleYAML), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(roleDir, "config.json"), []byte(roleJSON), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// YAML should take precedence
	configs, err := store.GetAgentRoleConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 1)

	role, ok := configs["test-role"]
	require.True(t, ok)
	assert.Equal(t, "YAML version", role.Description)
	assert.Equal(t, conf.AccessAllow, role.ToolsAccess["read"])
}

func TestLocalConfigStore_YAMLConfigFileWatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create initial global.yml
	globalYAML := `model_tags:
  - model: "^gpt-.*"
    tag: "openai"
`
	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)

	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Get initial timestamp
	initialTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Modify global.yml
	globalYAML = `model_tags:
  - model: "^claude-.*"
    tag: "anthropic"
  - model: "^gpt-.*"
    tag: "openai"
`
	err = os.WriteFile(filepath.Join(tmpDir, "global.yml"), []byte(globalYAML), 0644)
	require.NoError(t, err)

	// Wait for file watcher to process the event
	time.Sleep(20 * time.Millisecond)

	// Verify config was reloaded
	config, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Len(t, config.ModelTags, 2)

	// Verify timestamp was updated
	newTime, err := store.LastGlobalConfigUpdate()
	require.NoError(t, err)
	assert.True(t, newTime.After(initialTime))
}
