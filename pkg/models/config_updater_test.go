package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	confimpl "github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mockWritableStore is a mock implementation of conf.WritableConfigStore for testing.
type mockWritableStore struct {
	savedConfig  *conf.ModelProviderConfig
	saveCalled   bool
	saveError    error
	deletedName  string
	deleteCalled bool
	deleteError  error
	globalConfig *conf.GlobalConfig
	globalSaved  bool
	globalError  error
}

func (m *mockWritableStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetMCPServerConfigs() (map[string]*conf.MCPServerConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastMCPServerConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetHookConfigs() (map[string]*conf.HookConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastHookConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetAgentRoleConfigs() (map[string]*conf.AgentRoleConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastAgentRoleConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetGlobalConfig() (*conf.GlobalConfig, error) {
	return m.globalConfig, nil
}

func (m *mockWritableStore) LastGlobalConfigUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetAgentConfigFile(subdir, filename string) ([]byte, error) {
	return nil, nil
}

func (m *mockWritableStore) SaveModelProviderConfig(config *conf.ModelProviderConfig) error {
	m.saveCalled = true
	if m.saveError != nil {
		return m.saveError
	}
	m.savedConfig = config
	return nil
}

func (m *mockWritableStore) DeleteModelProviderConfig(name string) error {
	m.deleteCalled = true
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedName = name
	return nil
}

func (m *mockWritableStore) SaveGlobalConfig(config *conf.GlobalConfig) error {
	m.globalSaved = true
	if m.globalError != nil {
		return m.globalError
	}
	m.globalConfig = config
	return nil
}

func TestNewConfigUpdater(t *testing.T) {
	t.Run("creates config updater with store and provider name", func(t *testing.T) {
		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")

		require.NotNil(t, updater)
		assert.Equal(t, "test-provider", updater.ProviderName())
	})
}

func TestConfigUpdaterImpl_Update(t *testing.T) {
	t.Run("saves config to store", func(t *testing.T) {
		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		config := &conf.ModelProviderConfig{
			Name:   "test-provider",
			URL:    "https://api.example.com",
			Type:   "responses",
			APIKey: "new-api-key",
		}

		err := callback(config)
		require.NoError(t, err)
		assert.True(t, mockStore.saveCalled)
		assert.Equal(t, config, mockStore.savedConfig)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		err := callback(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config cannot be nil")
		assert.False(t, mockStore.saveCalled)
	})

	t.Run("returns error for empty config name", func(t *testing.T) {
		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		config := &conf.ModelProviderConfig{
			URL:    "https://api.example.com",
			Type:   "responses",
			APIKey: "new-api-key",
		}

		err := callback(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config name cannot be empty")
		assert.False(t, mockStore.saveCalled)
	})

	t.Run("propagates store save error", func(t *testing.T) {
		mockStore := &mockWritableStore{
			saveError: assert.AnError,
		}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		config := &conf.ModelProviderConfig{
			Name: "test-provider",
			URL:  "https://api.example.com",
		}

		err := callback(config)
		assert.Error(t, err)
		assert.True(t, mockStore.saveCalled)
	})
}

func TestConfigUpdaterImpl_Update_PreservesYAMLFormatAndCreatesBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-config-updater-yaml-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.MkdirAll(modelsDir, 0o755))

	originalConfig := &conf.ModelProviderConfig{
		Name:   "provider-yaml",
		Type:   "openai",
		URL:    "https://api.example.com/v1",
		APIKey: "old-token",
	}
	originalData, err := yaml.Marshal(originalConfig)
	require.NoError(t, err)

	providerPath := filepath.Join(modelsDir, "provider-yaml.yaml")
	require.NoError(t, os.WriteFile(providerPath, originalData, 0o644))

	backupPath := providerPath + ".bkp"
	require.NoError(t, os.WriteFile(backupPath, []byte("old-backup"), 0o644))

	store, err := confimpl.NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	updater := NewConfigUpdater(store, "provider-yaml")
	callback := updater.Update()

	updatedConfig := &conf.ModelProviderConfig{
		Name:   "provider-yaml",
		Type:   "openai",
		URL:    "https://api.example.com/v1",
		APIKey: "new-token",
	}

	err = callback(updatedConfig)
	require.NoError(t, err)

	backupData, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, string(originalData), string(backupData))

	currentData, err := os.ReadFile(providerPath)
	require.NoError(t, err)

	var persisted conf.ModelProviderConfig
	require.NoError(t, yaml.Unmarshal(currentData, &persisted))
	assert.Equal(t, "new-token", persisted.APIKey)
	assert.Equal(t, "provider-yaml", persisted.Name)

	jsonPath := filepath.Join(modelsDir, "provider-yaml.json")
	assert.NoFileExists(t, jsonPath)
}

func TestConfigUpdaterImpl_Update_PreservesJSONFormatAndCreatesBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-config-updater-json-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.MkdirAll(modelsDir, 0o755))

	originalConfig := &conf.ModelProviderConfig{
		Name:   "provider-json",
		Type:   "openai",
		URL:    "https://api.example.com/v1",
		APIKey: "old-token",
	}
	originalData, err := json.MarshalIndent(originalConfig, "", "  ")
	require.NoError(t, err)

	providerPath := filepath.Join(modelsDir, "provider-json.json")
	require.NoError(t, os.WriteFile(providerPath, originalData, 0o644))

	store, err := confimpl.NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	updater := NewConfigUpdater(store, "provider-json")
	callback := updater.Update()

	updatedConfig := &conf.ModelProviderConfig{
		Name:   "provider-json",
		Type:   "openai",
		URL:    "https://api.example.com/v1",
		APIKey: "new-token",
	}

	err = callback(updatedConfig)
	require.NoError(t, err)

	backupData, err := os.ReadFile(providerPath + ".bkp")
	require.NoError(t, err)
	assert.Equal(t, string(originalData), string(backupData))

	currentData, err := os.ReadFile(providerPath)
	require.NoError(t, err)

	var persisted conf.ModelProviderConfig
	require.NoError(t, json.Unmarshal(currentData, &persisted))
	assert.Equal(t, "new-token", persisted.APIKey)
	assert.Equal(t, "provider-json", persisted.Name)

	yamlPath := filepath.Join(modelsDir, "provider-json.yaml")
	assert.NoFileExists(t, yamlPath)
}

func TestConfigUpdaterImpl_Update_PreservesDurationFieldsAfterReadBack(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-config-updater-durations-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	modelsDir := filepath.Join(tmpDir, "models")
	require.NoError(t, os.MkdirAll(modelsDir, 0o755))

	providerPath := filepath.Join(modelsDir, "provider-duration.json")
	originalRaw := []byte(`{
  "name": "provider-duration",
  "type": "openai",
  "url": "https://api.example.com/v1",
  "api_key": "old-token",
  "connect_timeout": "3600s",
  "request_timeout": "120s"
}`)
	require.NoError(t, os.WriteFile(providerPath, originalRaw, 0o644))

	store, err := confimpl.NewLocalConfigStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	configs, err := store.GetModelProviderConfigs()
	require.NoError(t, err)

	config, ok := configs["provider-duration"]
	require.True(t, ok)
	assert.Equal(t, 3600*time.Second, config.ConnectTimeout)
	assert.Equal(t, 120*time.Second, config.RequestTimeout)

	updater := NewConfigUpdater(store, "provider-duration")
	callback := updater.Update()

	config.APIKey = "new-token"
	require.NoError(t, callback(config))

	configsAfterUpdate, err := store.GetModelProviderConfigs()
	require.NoError(t, err)

	updatedConfig, ok := configsAfterUpdate["provider-duration"]
	require.True(t, ok)
	assert.Equal(t, 3600*time.Second, updatedConfig.ConnectTimeout)
	assert.Equal(t, 120*time.Second, updatedConfig.RequestTimeout)

	updatedRaw, err := os.ReadFile(providerPath)
	require.NoError(t, err)

	var persisted map[string]any
	require.NoError(t, json.Unmarshal(updatedRaw, &persisted))
	assert.Equal(t, "3600s", persisted["connect_timeout"])
	assert.Equal(t, "120s", persisted["request_timeout"])
}
