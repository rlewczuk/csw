package models

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWritableStore is a mock implementation of conf.WritableConfigStore for testing.
type mockWritableStore struct {
	savedConfig   *conf.ModelProviderConfig
	saveCalled    bool
	saveError     error
	deletedName   string
	deleteCalled  bool
	deleteError   error
	globalConfig  *conf.GlobalConfig
	globalSaved   bool
	globalError   error
}

func (m *mockWritableStore) GetModelProviderConfigs() (map[string]*conf.ModelProviderConfig, error) {
	return nil, nil
}

func (m *mockWritableStore) LastModelProviderConfigsUpdate() (time.Time, error) {
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
			Name:  "test-provider",
			URL:   "https://api.example.com",
			Type:  "responses",
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
			URL:   "https://api.example.com",
			Type:  "responses",
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
			Name:  "test-provider",
			URL:   "https://api.example.com",
		}

		err := callback(config)
		assert.Error(t, err)
		assert.True(t, mockStore.saveCalled)
	})
}
