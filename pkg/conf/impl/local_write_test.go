package impl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalConfigStore_SaveModelProviderConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Create a test provider config
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Test provider",
		APIKey:      "test-key",
	}

	// Save config
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Verify file exists
	providerPath := filepath.Join(tmpDir, "models", "test-provider.json")
	assert.FileExists(t, providerPath)

	// Reload config and verify
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Contains(t, configs, "test-provider")
	assert.Equal(t, "openai", configs["test-provider"].Type)
	assert.Equal(t, "https://api.openai.com/v1", configs["test-provider"].URL)
	assert.Equal(t, "Test provider", configs["test-provider"].Description)
	assert.Equal(t, "test-key", configs["test-provider"].APIKey)

	storedData, err := os.ReadFile(providerPath)
	require.NoError(t, err)
	var stored map[string]any
	require.NoError(t, json.Unmarshal(storedData, &stored))
	assert.NotContains(t, stored, "name")
}

func TestLocalConfigStore_SaveModelProviderConfig_Update(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Create initial config
	config := &conf.ModelProviderConfig{
		Name:        "test-provider",
		Type:        "openai",
		URL:         "https://api.openai.com/v1",
		Description: "Test provider",
	}
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Update config
	config.Description = "Updated provider"
	config.APIKey = "new-key"
	err = store.SaveModelProviderConfig(config)
	require.NoError(t, err)

	// Verify update
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Contains(t, configs, "test-provider")
	assert.Equal(t, "Updated provider", configs["test-provider"].Description)
	assert.Equal(t, "new-key", configs["test-provider"].APIKey)
}


func TestLocalConfigStore_SaveModelProviderConfig_NilConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Try to save nil config
	err = store.SaveModelProviderConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestLocalConfigStore_SaveModelProviderConfig_EmptyName(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Create config with empty name
	config := &conf.ModelProviderConfig{
		Type: "openai",
		URL:  "https://api.openai.com/v1",
	}

	// Try to save
	err = store.SaveModelProviderConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestLocalConfigStore_Save_MultipleProviders(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config store
	store, err := NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Create multiple providers
	providers := []conf.ModelProviderConfig{
		{Name: "provider1", Type: "openai", URL: "http://test1"},
		{Name: "provider2", Type: "ollama", URL: "http://test2"},
		{Name: "provider3", Type: "anthropic", URL: "http://test3"},
	}

	for i := range providers {
		err = store.SaveModelProviderConfig(&providers[i])
		require.NoError(t, err)
	}

	// Verify all exist
	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 3)

	// Verify all were persisted under expected provider names
	configs, err = store.GetModelProviderConfigs()
	require.NoError(t, err)
	assert.Len(t, configs, 3)
	assert.Contains(t, configs, "provider1")
	assert.Contains(t, configs, "provider2")
	assert.Contains(t, configs, "provider3")
}
