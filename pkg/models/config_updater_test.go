package models

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

// mockWritableStore is a mock implementation of conf.WritableConfigStore for testing.
type mockWritableStore struct {
	savedConfig  *conf.ModelProviderConfig
	saveCalled   bool
	saveError    error
	globalConfig *conf.GlobalConfig
}

func (m *mockWritableStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastModelProviderConfigsUpdate() (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockWritableStore) GetModelAliases() (map[string]conf.ModelAliasValue, error) {
	return map[string]conf.ModelAliasValue{}, nil
}

func (m *mockWritableStore) LastModelAliasesUpdate() (time.Time, error) {
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

func TestNewConfigUpdater(t *testing.T) {
	t.Run("creates config updater with store and provider name", func(t *testing.T) {
		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")

		require.NotNil(t, updater)
		assert.Equal(t, "test-provider", updater.ProviderName())
	})
}

func TestConfigUpdaterImpl_Update(t *testing.T) {
	t.Run("saves config to user models directory", func(t *testing.T) {
		tmpHome, err := os.MkdirTemp("", "csw-home-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpHome)

		oldHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tmpHome))
		defer os.Setenv("HOME", oldHome)

		mockStore := &mockWritableStore{}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		config := &conf.ModelProviderConfig{
			Name:   "test-provider",
			URL:    "https://api.example.com",
			Type:   "responses",
			APIKey: "new-api-key",
		}

		require.NoError(t, callback(config))
		assert.False(t, mockStore.saveCalled)

		configPath := filepath.Join(tmpHome, ".config", "csw", "models", "test-provider.json")
		data, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var saved conf.ModelProviderConfig
		require.NoError(t, json.Unmarshal(data, &saved))
		assert.Equal(t, config.Type, saved.Type)
		assert.Equal(t, config.URL, saved.URL)
		assert.Equal(t, config.APIKey, saved.APIKey)
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

	t.Run("does not use writable store save method", func(t *testing.T) {
		tmpHome, err := os.MkdirTemp("", "csw-home-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpHome)

		oldHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tmpHome))
		defer os.Setenv("HOME", oldHome)

		mockStore := &mockWritableStore{
			saveError: assert.AnError,
		}
		updater := NewConfigUpdater(mockStore, "test-provider")
		callback := updater.Update()

		config := &conf.ModelProviderConfig{
			Name: "test-provider",
			URL:  "https://api.example.com",
		}

		require.NoError(t, callback(config))
		assert.False(t, mockStore.saveCalled)
	})
}

func TestSaveProviderConfigToModelsDir(t *testing.T) {
	tests := []struct {
		name          string
		config        *conf.ModelProviderConfig
		prepareDir    func(t *testing.T, root string) string
		expectedError string
		verify        func(t *testing.T, modelsDir string, config *conf.ModelProviderConfig)
	}{
		{
			name: "success writes json and creates directory",
			config: &conf.ModelProviderConfig{
				Name:         "provider-a",
				Type:         "openai",
				URL:          "https://api.example.com/v1",
				APIKey:       "token-1",
				RefreshToken: "refresh-1",
				AuthMode:     conf.AuthModeOAuth2,
			},
			prepareDir: func(t *testing.T, root string) string {
				return filepath.Join(root, "nested", "models")
			},
			verify: func(t *testing.T, modelsDir string, config *conf.ModelProviderConfig) {
				t.Helper()

				configPath := filepath.Join(modelsDir, config.Name+".json")
				data, err := os.ReadFile(configPath)
				require.NoError(t, err)

				var saved conf.ModelProviderConfig
				err = json.Unmarshal(data, &saved)
				require.NoError(t, err)

				assert.Equal(t, config.Type, saved.Type)
				assert.Equal(t, config.URL, saved.URL)
				assert.Equal(t, config.APIKey, saved.APIKey)
				assert.Equal(t, config.RefreshToken, saved.RefreshToken)
				assert.Equal(t, config.AuthMode, saved.AuthMode)

				assert.FileExists(t, filepath.Join(modelsDir, config.Name+".json"))
				assert.NoFileExists(t, filepath.Join(modelsDir, config.Name+".json.bak"))
			},
		},
		{
			name: "creates backup when overwriting existing provider file",
			config: &conf.ModelProviderConfig{
				Name:   "provider-overwrite",
				Type:   "openai",
				URL:    "https://new.example.com/v1",
				APIKey: "new-token",
			},
			prepareDir: func(t *testing.T, root string) string {
				t.Helper()
				modelsDir := filepath.Join(root, "models")
				err := os.MkdirAll(modelsDir, 0755)
				require.NoError(t, err)

				existing := []byte(`{"type":"openai","url":"https://old.example.com/v1","api-key":"old-token"}`)
				err = os.WriteFile(filepath.Join(modelsDir, "provider-overwrite.json"), existing, 0644)
				require.NoError(t, err)
				return modelsDir
			},
			verify: func(t *testing.T, modelsDir string, config *conf.ModelProviderConfig) {
				t.Helper()

				backupPath := filepath.Join(modelsDir, config.Name+".json.bak")
				backupData, err := os.ReadFile(backupPath)
				require.NoError(t, err)
				assert.Contains(t, string(backupData), "https://old.example.com/v1")
				assert.Contains(t, string(backupData), "old-token")

				configData, err := os.ReadFile(filepath.Join(modelsDir, config.Name+".json"))
				require.NoError(t, err)
				assert.Contains(t, string(configData), "https://new.example.com/v1")
				assert.Contains(t, string(configData), "new-token")
			},
		},
		{
			name:          "fails for nil config",
			config:        nil,
			prepareDir:    func(t *testing.T, root string) string { return filepath.Join(root, "models") },
			expectedError: "provider config is nil",
		},
		{
			name: "fails for empty provider name",
			config: &conf.ModelProviderConfig{
				Name: "",
				Type: "openai",
				URL:  "https://api.example.com/v1",
			},
			prepareDir:    func(t *testing.T, root string) string { return filepath.Join(root, "models") },
			expectedError: "provider config name is empty",
		},
		{
			name: "fails when models path is file",
			config: &conf.ModelProviderConfig{
				Name: "provider-b",
				Type: "openai",
				URL:  "https://api.example.com/v1",
			},
			prepareDir: func(t *testing.T, root string) string {
				t.Helper()
				modelsDir := filepath.Join(root, "models-file")
				err := os.WriteFile(modelsDir, []byte("not-a-directory"), 0644)
				require.NoError(t, err)
				return modelsDir
			},
			expectedError: "failed to create models directory",
		},
		{
			name: "fails when config cannot be marshaled",
			config: &conf.ModelProviderConfig{
				Name: "provider-c",
				Type: "openai",
				URL:  "https://api.example.com/v1",
				Options: map[string]any{
					"invalid": make(chan int),
				},
			},
			prepareDir:    func(t *testing.T, root string) string { return filepath.Join(root, "models") },
			expectedError: "failed to marshal provider config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "csw-provider-save-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			modelsDir := tt.prepareDir(t, tmpDir)
			err = SaveProviderConfigToModelsDir(tt.config, modelsDir)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, modelsDir, tt.config)
			}
		})
	}
}

func TestSaveProviderConfigToUserModelsDir(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	providerConfig := &conf.ModelProviderConfig{
		Name:   "provider-user-dir",
		Type:   "openai",
		URL:    "https://api.example.com/v1",
		APIKey: "secret",
	}

	err = SaveProviderConfigToUserModelsDir(providerConfig)
	require.NoError(t, err)

	configPath := filepath.Join(tmpHome, ".config", "csw", "models", providerConfig.Name+".json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var saved conf.ModelProviderConfig
	err = json.Unmarshal(data, &saved)
	require.NoError(t, err)
	assert.Equal(t, providerConfig.Type, saved.Type)
	assert.Equal(t, providerConfig.URL, saved.URL)
	assert.Equal(t, providerConfig.APIKey, saved.APIKey)
}
