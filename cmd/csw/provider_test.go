package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/conf"
	"github.com/codesnort/codesnort-swe/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigStore_Local(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Get local config store
	store, err := GetConfigStore(ConfigScopeLocal)
	require.NoError(t, err)
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Verify directory was created
	configDir := filepath.Join(tmpDir, ".csw", "config")
	assert.DirExists(t, configDir)
}

func TestGetConfigStore_CustomPath(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	customPath := filepath.Join(tmpDir, "custom", "config")

	// Get config store with custom path
	store, err := GetConfigStore(ConfigScope(customPath))
	require.NoError(t, err)
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Verify directory was created
	assert.DirExists(t, customPath)
}

func TestGetConfigStore_Global(t *testing.T) {
	// Create temporary home directory
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Get global config store
	store, err := GetConfigStore(ConfigScopeGlobal)
	require.NoError(t, err)
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Verify directory was created
	configDir := filepath.Join(tmpHome, ".config", "csw")
	assert.DirExists(t, configDir)
}

func TestGetCompositeConfigStore(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create local config directory with a provider
	localConfigDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(localConfigDir, 0755)
	require.NoError(t, err)
	localStore, err := impl.NewLocalConfigStore(localConfigDir)
	require.NoError(t, err)

	localProvider := &conf.ModelProviderConfig{
		Name:        "local-provider",
		Type:        "openai",
		URL:         "https://local.example.com/v1",
		Description: "Local provider",
	}
	err = localStore.SaveModelProviderConfig(localProvider)
	require.NoError(t, err)
	localStore.Close()

	// Create global config directory with a provider
	globalConfigDir := filepath.Join(tmpHome, ".config", "csw")
	err = os.MkdirAll(globalConfigDir, 0755)
	require.NoError(t, err)
	globalStore, err := impl.NewLocalConfigStore(globalConfigDir)
	require.NoError(t, err)

	globalProvider := &conf.ModelProviderConfig{
		Name:        "global-provider",
		Type:        "anthropic",
		URL:         "https://api.anthropic.com/v1",
		Description: "Global provider",
	}
	err = globalStore.SaveModelProviderConfig(globalProvider)
	require.NoError(t, err)
	globalStore.Close()

	// Get composite config store
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	// Get all provider configs
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)

	// Verify both providers are available
	assert.Contains(t, configs, "local-provider")
	assert.Contains(t, configs, "global-provider")
	assert.Equal(t, "openai", configs["local-provider"].Type)
	assert.Equal(t, "anthropic", configs["global-provider"].Type)
}

func TestProviderCommand_Add_List_Show_Remove(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	configDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	store, err := impl.NewLocalConfigStore(configDir)
	require.NoError(t, err)
	defer store.Close()

	// Test adding a provider
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Test provider",
		APIKey:      "test-key",
	}
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Test listing providers
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Contains(t, configs, "test-provider")

	// Test showing provider details
	assert.Equal(t, "openai", configs["test-provider"].Type)
	assert.Equal(t, "https://api.openai.com/v1", configs["test-provider"].URL)

	// Test removing provider
	err = store.DeleteModelProviderConfig("test-provider")
	require.NoError(t, err)

	configs, err = store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.NotContains(t, configs, "test-provider")
}

func TestProviderCommand_SetDefault(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	configDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	store, err := impl.NewLocalConfigStore(configDir)
	require.NoError(t, err)
	defer store.Close()

	// Add a provider
	config := &conf.ModelProviderConfig{
		Name: "test-provider",
		Type: "openai",
		URL:  "https://api.openai.com/v1",
	}
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Set as default
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	globalConfig.DefaultProvider = "test-provider"
	err = store.SaveGlobalConfig(globalConfig)
	require.NoError(t, err)

	// Verify default is set
	loadedConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "test-provider", loadedConfig.DefaultProvider)
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "long key",
			key:      "sk-1234567890abcdef",
			expected: "sk-1****cdef",
		},
		{
			name:     "short key",
			key:      "short",
			expected: "********",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "********",
		},
		{
			name:     "8 char key",
			key:      "12345678",
			expected: "********",
		},
		{
			name:     "9 char key",
			key:      "123456789",
			expected: "1234****6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputProviderList(t *testing.T) {
	configs := map[string]*conf.ModelProviderConfig{
		"provider1": {
			Name:        "provider1",
			Type:        "openai",
			Description: "Provider 1",
		},
		"provider2": {
			Name:        "provider2",
			Type:        "ollama",
			Description: "Provider 2",
		},
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputProviderList(configs)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "provider1")
	assert.Contains(t, output, "provider2")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "ollama")
}

func TestOutputProviderDetails(t *testing.T) {
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Test provider",
		APIKey:      "sk-1234567890abcdef",
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputProviderDetails(config)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "test-provider")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "https://api.openai.com/v1")
	assert.Contains(t, output, "Test provider")
	// Should show masked API key
	assert.Contains(t, output, "sk-1****cdef")
}

func TestPromptProviderConfig(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		url          string
		description  string
		apiKey       string
	}{
		{
			name:         "all provided",
			providerType: "openai",
			url:          "https://api.openai.com/v1",
			description:  "Test",
			apiKey:       "test-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := promptProviderConfig("test", tt.providerType, tt.url, tt.description, tt.apiKey)
			require.NoError(t, err)
			assert.Equal(t, "test", config.Name)
			assert.Equal(t, tt.providerType, config.Type)
			assert.Equal(t, tt.url, config.URL)
			assert.Equal(t, tt.description, config.Description)
			assert.Equal(t, tt.apiKey, config.APIKey)
		})
	}
}

func TestProviderCommandWithCustomPath(t *testing.T) {
	// Create temporary directory for custom config
	tmpDir, err := os.MkdirTemp("", "csw-custom-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	customConfigPath := filepath.Join(tmpDir, "custom-config")

	// Test adding a provider to custom path
	store, err := GetConfigStore(ConfigScope(customConfigPath))
	require.NoError(t, err)
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	config := &conf.ModelProviderConfig{
		Name:        "custom-provider",
		Type:        "openai",
		URL:         "https://custom.example.com/v1",
		Description: "Custom path provider",
		APIKey:      "custom-key",
	}
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Verify the provider was saved to custom path
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Contains(t, configs, "custom-provider")
	assert.Equal(t, "openai", configs["custom-provider"].Type)
	assert.Equal(t, "https://custom.example.com/v1", configs["custom-provider"].URL)

	// Test removing provider from custom path
	err = store.DeleteModelProviderConfig("custom-provider")
	require.NoError(t, err)

	configs, err = store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.NotContains(t, configs, "custom-provider")
}

func TestProviderCommandWithCustomPathSetDefault(t *testing.T) {
	// Create temporary directory for custom config
	tmpDir, err := os.MkdirTemp("", "csw-custom-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	customConfigPath := filepath.Join(tmpDir, "custom-config")

	// Create config store with custom path
	store, err := GetConfigStore(ConfigScope(customConfigPath))
	require.NoError(t, err)
	defer func() {
		if closer, ok := store.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Add a provider
	config := &conf.ModelProviderConfig{
		Name:        "custom-default-provider",
		Type:        "anthropic",
		URL:         "https://api.anthropic.com/v1",
		Description: "Custom default provider",
	}
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Set as default
	globalConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	globalConfig.DefaultProvider = "custom-default-provider"
	err = store.SaveGlobalConfig(globalConfig)
	require.NoError(t, err)

	// Verify default is set
	loadedConfig, err := store.GetGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "custom-default-provider", loadedConfig.DefaultProvider)
}

func TestProviderListShowComposite(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Change to temp directory
	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(oldDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create local config with provider
	localConfigDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(localConfigDir, 0755)
	require.NoError(t, err)
	localStore, err := impl.NewLocalConfigStore(localConfigDir)
	require.NoError(t, err)

	localProvider := &conf.ModelProviderConfig{
		Name:        "local-test",
		Type:        "ollama",
		URL:         "http://localhost:11434",
		Description: "Local test provider",
	}
	err = localStore.SaveModelProviderConfig(localProvider)
	require.NoError(t, err)
	localStore.Close()

	// Create global config with provider
	globalConfigDir := filepath.Join(tmpHome, ".config", "csw")
	err = os.MkdirAll(globalConfigDir, 0755)
	require.NoError(t, err)
	globalStore, err := impl.NewLocalConfigStore(globalConfigDir)
	require.NoError(t, err)

	globalProvider := &conf.ModelProviderConfig{
		Name:        "global-test",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Global test provider",
	}
	err = globalStore.SaveModelProviderConfig(globalProvider)
	require.NoError(t, err)
	globalStore.Close()

	// Get composite store and verify both providers are listed
	compositeStore, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs, err := compositeStore.GetModelProviderConfigs()
	require.NoError(t, err)

	// Both providers should be available
	assert.Contains(t, configs, "local-test")
	assert.Contains(t, configs, "global-test")

	// Verify provider details
	localConfig, exists := configs["local-test"]
	require.True(t, exists)
	assert.Equal(t, "ollama", localConfig.Type)
	assert.Equal(t, "http://localhost:11434", localConfig.URL)

	globalConfig, exists := configs["global-test"]
	require.True(t, exists)
	assert.Equal(t, "openai", globalConfig.Type)
	assert.Equal(t, "https://api.openai.com/v1", globalConfig.URL)
}

func TestOutputModelsList(t *testing.T) {
	modelsList := []modelEntry{
		{Provider: "provider1", Model: "model1"},
		{Provider: "provider1", Model: "model2"},
		{Provider: "provider2", Model: "model3"},
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputModelsList(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "MODEL")
	assert.Contains(t, output, "provider1")
	assert.Contains(t, output, "provider2")
	assert.Contains(t, output, "model1")
	assert.Contains(t, output, "model2")
	assert.Contains(t, output, "model3")
}

func TestOutputModelsListEmpty(t *testing.T) {
	var modelsList []modelEntry

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputModelsList(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	// Should only have header
	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "MODEL")
}

func TestOutputModelsListJSON(t *testing.T) {
	modelsList := []modelEntry{
		{Provider: "provider1", Model: "model1"},
		{Provider: "provider2", Model: "model2"},
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputJSON(modelsList)
	w.Close()
	os.Stdout = oldStdout

	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	// Verify JSON structure
	assert.Contains(t, output, `"provider"`)
	assert.Contains(t, output, `"model"`)
	assert.Contains(t, output, `"provider1"`)
	assert.Contains(t, output, `"model1"`)
	assert.Contains(t, output, `"provider2"`)
	assert.Contains(t, output, `"model2"`)

	// Verify it's valid JSON by unmarshaling
	var decoded []modelEntry
	err = json.Unmarshal([]byte(output), &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded, 2)
	assert.Equal(t, "provider1", decoded[0].Provider)
	assert.Equal(t, "model1", decoded[0].Model)
}
