package models

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultResponsesTestURL = "https://api.openai.com/v1"
	testResponsesModelName  = "gpt-4o-mini"
	testResponsesTimeout    = 30
	connectResponsesTimeout = 5
)

// responsesTestClient holds either a real or mock client and provides cleanup.
type responsesTestClient struct {
	Client *ResponsesClient
	Mock   *testutil.MockHTTPServer
}

// Close cleans up the test client resources.
func (tc *responsesTestClient) Close() {
	if tc.Mock != nil {
		tc.Mock.Close()
	}
}

func getResponsesModelName() string {
	if model := testutil.IntegCfgReadFile("responses.model"); model != "" {
		return model
	}
	return testResponsesModelName
}

// getResponsesTestClient returns a client for testing - either real or mock based on integration mode.
// For mock mode, it also returns the mock server for adding responses.
func getResponsesTestClient(t *testing.T) *responsesTestClient {
	t.Helper()

	if testutil.IntegTestEnabled("responses") {
		url := testutil.IntegCfgReadFile("responses.url")
		if url == "" {
			url = defaultResponsesTestURL
		}
		apiKey := testutil.IntegCfgReadFile("responses.key")
		if apiKey == "" {
			t.Skip("Skipping test: _integ/responses.key not configured")
		}

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:            url,
			APIKey:         apiKey,
			ConnectTimeout: connectResponsesTimeout,
			RequestTimeout: testResponsesTimeout,
		})
		require.NoError(t, err)

		return &responsesTestClient{Client: client}
	}

	mock := testutil.NewMockHTTPServer()
	client, err := NewResponsesClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	return &responsesTestClient{Client: client, Mock: mock}
}

func TestNewResponsesClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:            defaultResponsesTestURL,
			APIKey:         "test-api-key",
			ConnectTimeout: connectResponsesTimeout,
			RequestTimeout: testResponsesTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewResponsesClient(nil)

		assert.Error(t, err)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewResponsesClient(&conf.ModelProviderConfig{URL: ""})

		assert.Error(t, err)
	})
}

func TestNewResponsesClientWithHTTPClient(t *testing.T) {
	t.Run("creates client with custom HTTP client", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewResponsesClientWithHTTPClient(mock.URL(), mock.Client())

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		_, err := NewResponsesClientWithHTTPClient("", mock.Client())

		assert.Error(t, err)
	})

	t.Run("returns error for nil HTTP client", func(t *testing.T) {
		_, err := NewResponsesClientWithHTTPClient(defaultResponsesTestURL, nil)

		assert.Error(t, err)
	})
}

func TestResponsesClient_ListModels(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	if tc.Mock != nil {
		modelsResponse := `{"data":[{"id":"gpt-4o-mini","object":"model","created":1640000000,"owned_by":"openai"},{"id":"gpt-4o","object":"model","created":1640000000,"owned_by":"openai"}]}`
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
		tc.Mock.AddRestResponse("/models", "GET", modelsResponse)
	}

	t.Run("lists available models", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)
		assert.NotNil(t, modelList)
		assert.NotEmpty(t, modelList)
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := tc.Client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == getResponsesModelName() {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", getResponsesModelName())
	})
}

func TestResponsesClient_ListModels_ResponsesModelsPayload(t *testing.T) {
	tc := getResponsesTestClient(t)
	defer tc.Close()

	if tc.Mock == nil {
		t.Skip("Skipping responses payload assertions against real provider")
	}

	tc.Mock.AddRestResponse("/models", "GET", `{"models":[{"slug":"gpt-5.2-codex","display_name":"gpt-5.2-codex"},{"slug":"gpt-5.2","display_name":"gpt-5.2"}]}`)

	modelList, err := tc.Client.ListModels()
	require.NoError(t, err)
	require.Len(t, modelList, 2)

	assert.Equal(t, "gpt-5.2-codex", modelList[0].Name)
	assert.Equal(t, "gpt-5.2-codex", modelList[0].Model)
	assert.Equal(t, "gpt-5.2", modelList[1].Name)
	assert.Equal(t, "gpt-5.2", modelList[1].Model)
}

func TestResponsesClient_QueryParams(t *testing.T) {
	t.Run("list models includes configured query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "0.1.0", r.URL.Query().Get("client_version"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"data":[{"id":"test-model","object":"model","created":1640000000,"owned_by":"openai"}]}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
			QueryParams: map[string]string{
				"client_version": "0.1.0",
			},
		})
		require.NoError(t, err)

		models, err := client.ListModels()
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "test-model", models[0].Name)
	})

	t.Run("list models includes multiple query params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "0.1.0", r.URL.Query().Get("client_version"))
			assert.Equal(t, "test-value", r.URL.Query().Get("custom_param"))

			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"data":[{"id":"test-model","object":"model","created":1640000000,"owned_by":"openai"}]}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewResponsesClient(&conf.ModelProviderConfig{
			URL:    server.URL,
			APIKey: "test-key",
			QueryParams: map[string]string{
				"client_version": "0.1.0",
				"custom_param":   "test-value",
			},
		})
		require.NoError(t, err)

		models, err := client.ListModels()
		require.NoError(t, err)
		require.Len(t, models, 1)
	})
}
