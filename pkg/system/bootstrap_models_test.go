package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDisableRefreshToProviders(t *testing.T) {
	mockProvider := models.NewMockProvider(nil)
	mockProvider.Config = &conf.ModelProviderConfig{Name: "mock-provider"}

	providers := map[string]models.ModelProvider{
		"mock": mockProvider,
	}

	applyDisableRefreshToProviders(providers)

	assert.NotNil(t, mockProvider.GetConfig())
	assert.True(t, mockProvider.GetConfig().DisableRefresh)
}

func TestKimiTagResolution(t *testing.T) {
	store, err := conf.CswConfigLoad("@DEFAULTS")
	require.NoError(t, err)

	providerRegistry := models.NewProviderRegistry(store)

	registry, err := CreateModelTagRegistry(store, providerRegistry)
	require.NoError(t, err)

	tests := []struct {
		modelName     string
		expectedTag   string
		shouldContain bool
	}{
		{modelName: "kimi/kimi-for-coding", expectedTag: "kimi", shouldContain: true},
		{modelName: "kimi/moonshot-v1-8k", expectedTag: "kimi", shouldContain: true},
		{modelName: "openai/gpt-4", expectedTag: "openai", shouldContain: true},
		{modelName: "anthropic/claude-3-opus", expectedTag: "anthropic", shouldContain: true},
	}

	for _, tc := range tests {
		t.Run(tc.modelName, func(t *testing.T) {
			var provider, model string
			parts := strings.Split(tc.modelName, "/")
			if len(parts) == 2 {
				provider = parts[0]
				model = parts[1]
			} else {
				model = tc.modelName
			}

			tags := registry.GetTagsForModel(provider, model)
			if tc.shouldContain {
				assert.Contains(t, tags, tc.expectedTag, "Model %s (provider=%s, model=%s) should have tag %s. Got: %v", tc.modelName, provider, model, tc.expectedTag, tags)
			} else {
				assert.NotContains(t, tags, tc.expectedTag, "Model %s should NOT have tag %s", tc.modelName, tc.expectedTag)
			}
		})
	}
}

func TestCreateProviderMapConfigUpdaterWiring(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "csw-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", tmpHome))
	defer os.Setenv("HOME", oldHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"access_token":"new-access-token","refresh_token":"new-refresh-token","expires_in":3600}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	store := &conf.CswConfig{ModelProviderConfigs: map[string]*conf.ModelProviderConfig{
		"resp": {
			Name:         "resp",
			Type:         "responses",
			URL:          server.URL,
			AuthMode:     conf.AuthModeOAuth2,
			TokenURL:     server.URL + "/oauth/token",
			ClientID:     "test-client-id",
			RefreshToken: "old-refresh-token",
		},
	}}

	registry := models.NewProviderRegistry(store)
	providers, err := CreateProviderMap(registry)
	require.NoError(t, err)
	require.Len(t, providers, 1)

	provider, exists := providers["resp"]
	require.True(t, exists)

	responsesClient, ok := provider.(*models.ResponsesClient)
	require.True(t, ok)

	err = responsesClient.RefreshTokenIfNeeded()
	require.NoError(t, err)

	updatedConfigPath := filepath.Join(tmpHome, ".config", "csw", "models", "resp.json")
	updatedConfigData, err := os.ReadFile(updatedConfigPath)
	require.NoError(t, err)

	var updatedConfig conf.ModelProviderConfig
	require.NoError(t, json.Unmarshal(updatedConfigData, &updatedConfig))
	assert.Equal(t, "new-access-token", updatedConfig.APIKey)
	assert.Equal(t, "new-refresh-token", updatedConfig.RefreshToken)
}
