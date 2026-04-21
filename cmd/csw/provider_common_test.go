package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
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
	localProvider := &conf.ModelProviderConfig{
		Name:        "local-provider",
		Type:        "openai",
		URL:         "https://local.example.com/v1",
		Description: "Local provider",
	}
	err = models.SaveProviderConfigToModelsDir(localProvider, filepath.Join(localConfigDir, "models"))
	require.NoError(t, err)

	// Create global config directory with a provider
	globalConfigDir := filepath.Join(tmpHome, ".config", "csw")
	err = os.MkdirAll(globalConfigDir, 0755)
	require.NoError(t, err)
	globalProvider := &conf.ModelProviderConfig{
		Name:        "global-provider",
		Type:        "anthropic",
		URL:         "https://api.anthropic.com/v1",
		Description: "Global provider",
	}
	err = models.SaveProviderConfigToModelsDir(globalProvider, filepath.Join(globalConfigDir, "models"))
	require.NoError(t, err)

	// Get composite config store
	store, err := GetCompositeConfigStore()
	require.NoError(t, err)

	// Get all provider configs
	configs := store.ModelProviderConfigs

	// Verify both providers are available
	assert.Contains(t, configs, "local-provider")
	assert.Contains(t, configs, "global-provider")
	assert.Equal(t, "openai", configs["local-provider"].Type)
	assert.Equal(t, "anthropic", configs["global-provider"].Type)
}

func TestProviderCommand_Add_List_Show(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	configDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Test adding a provider
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Test provider",
		APIKey:      "test-key",
	}
	err = models.SaveProviderConfigToModelsDir(config, filepath.Join(configDir, "models"))
	require.NoError(t, err)

	store, err := conf.CswConfigLoad(configDir)
	require.NoError(t, err)

	// Test listing providers
	configs := store.ModelProviderConfigs
	assert.Contains(t, configs, "test-provider")

	// Test showing provider details
	assert.Equal(t, "openai", configs["test-provider"].Type)
	assert.Equal(t, "https://api.openai.com/v1", configs["test-provider"].URL)

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

	// Add a provider
	config := &conf.ModelProviderConfig{
		Name: "test-provider",
		Type: "openai",
		URL:  "https://api.openai.com/v1",
	}
	err = models.SaveProviderConfigToModelsDir(config, filepath.Join(configDir, "models"))
	require.NoError(t, err)

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
	config := &conf.ModelProviderConfig{
		Name:        "custom-provider",
		Type:        "openai",
		URL:         "https://custom.example.com/v1",
		Description: "Custom path provider",
		APIKey:      "custom-key",
	}
	err = models.SaveProviderConfigToModelsDir(config, filepath.Join(customConfigPath, "models"))
	require.NoError(t, err)

	store, err := GetConfigStore(ConfigScope(customConfigPath))
	require.NoError(t, err)

	// Verify the provider was saved to custom path
	configs := store.ModelProviderConfigs
	assert.Contains(t, configs, "custom-provider")
	assert.Equal(t, "openai", configs["custom-provider"].Type)
	assert.Equal(t, "https://custom.example.com/v1", configs["custom-provider"].URL)

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
	localProvider := &conf.ModelProviderConfig{
		Name:        "local-test",
		Type:        "ollama",
		URL:         "http://localhost:11434",
		Description: "Local test provider",
	}
	err = models.SaveProviderConfigToModelsDir(localProvider, filepath.Join(localConfigDir, "models"))
	require.NoError(t, err)

	// Create global config with provider
	globalConfigDir := filepath.Join(tmpHome, ".config", "csw")
	err = os.MkdirAll(globalConfigDir, 0755)
	require.NoError(t, err)
	globalProvider := &conf.ModelProviderConfig{
		Name:        "global-test",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Global test provider",
	}
	err = models.SaveProviderConfigToModelsDir(globalProvider, filepath.Join(globalConfigDir, "models"))
	require.NoError(t, err)

	// Get composite store and verify both providers are listed
	compositeStore, err := GetCompositeConfigStore()
	require.NoError(t, err)

	configs := compositeStore.ModelProviderConfigs

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
