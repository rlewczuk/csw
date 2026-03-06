package models

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJetBrainsClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewJetBrainsClient(&conf.ModelProviderConfig{
			URL:    "https://api.jetbrains.ai",
			APIKey: "jwt-token",
		})
		require.NoError(t, err)
		require.NotNil(t, client)
		require.NotNil(t, client.openaiClient)
		assert.Equal(t, "", client.openaiClient.apiKey)
	})

	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewJetBrainsClient(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create base client")
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewJetBrainsClient(&conf.ModelProviderConfig{URL: ""})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "URL cannot be empty")
	})
}

func TestJetBrainsChatModel_Chat_UsesJetBrainsEndpointAndAuthHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewJetBrainsClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	client.openaiClient.config = &conf.ModelProviderConfig{
		URL:          mock.URL(),
		APIKey:       "jwt-token",
		RefreshToken: "bearer-token",
		MaxTokens:    2048,
		Headers: map[string]string{
			"originator": "csw",
		},
	}

	mock.AddStreamingResponse(jetbrainsChatStreamPath, http.MethodPost, true,
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
		"data: [DONE]",
	)

	chatModel := client.ChatModel("google-chat-gemini-flash-2.0", nil)
	response, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "Hello", response.GetText())

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, jetbrainsChatStreamPath, req.Path)
	assert.Equal(t, "Bearer bearer-token", req.Header.Get("Authorization"))
	assert.Equal(t, "jwt-token", req.Header.Get(jetbrainsAuthenticateJWTHeader))
	assert.Equal(t, "bearer-token", req.Header.Get(jetbrainsAccessTokenHeader))
	assert.Equal(t, "csw", req.Header.Get("originator"))

	var payload ResponsesCreateRequest
	require.NoError(t, json.Unmarshal(req.Body, &payload))
	assert.Equal(t, "google-chat-gemini-flash-2.0", payload.Model)
	assert.True(t, payload.Stream)
	assert.Equal(t, 2048, payload.MaxOutputTokens)
}

func TestJetBrainsChatModel_Chat_UsesAuthorizationHeaderFallbackFromConfiguredHeaders(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewJetBrainsClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)

	client.openaiClient.config = &conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "jwt-token",
		Headers: map[string]string{
			"Authorization": "Bearer browser-token",
		},
	}

	mock.AddStreamingResponse(jetbrainsChatStreamPath, http.MethodPost, true,
		`data: {"type":"response.output_text.delta","delta":"ok"}`,
		"data: [DONE]",
	)

	chatModel := client.ChatModel("model-1", nil)
	response, err := chatModel.Chat(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, response)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 1)
	req := reqs[0]
	assert.Equal(t, "Bearer browser-token", req.Header.Get("Authorization"))
	assert.Equal(t, "browser-token", req.Header.Get(jetbrainsAccessTokenHeader))
}

func TestJetBrainsChatModel_ChatStream_YieldsTextAndToolCallAndUsage(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewJetBrainsClientWithHTTPClient(mock.URL(), mock.Client())
	require.NoError(t, err)
	client.openaiClient.config = &conf.ModelProviderConfig{
		URL:          mock.URL(),
		APIKey:       "jwt-token",
		RefreshToken: "bearer-token",
	}

	mock.AddStreamingResponse(jetbrainsChatStreamPath, http.MethodPost, true,
		`event: response.output_item.added`,
		`data: {"type":"response.output_text.delta","delta":"A"}`,
		`data: {"type":"response.output_item.added","item":{"id":"item_1","type":"function_call","call_id":"call_1","name":"vfsRead"}}`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"item_1","delta":"{\"path\":\"README.md\""}`,
		`data: {"type":"response.function_call_arguments.done","item_id":"item_1","arguments":"{\"path\":\"README.md\"}"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,"input_tokens_details":{"cached_tokens":2}}}}`,
	)

	chatModel := client.ChatModel("model-1", nil)
	stream := chatModel.ChatStream(context.Background(), []*ChatMessage{NewTextMessage(ChatRoleUser, "hi")}, nil, []tool.ToolInfo{{Name: "vfsRead", Description: "Read file"}})

	var fragments []*ChatMessage
	for fragment := range stream {
		fragments = append(fragments, fragment)
	}
	require.NotEmpty(t, fragments)

	assert.Equal(t, "A", fragments[0].GetText())
	toolCalls := fragments[1].GetToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "vfsRead", toolCalls[0].Function)
	assert.Equal(t, "README.md", toolCalls[0].Arguments.Get("path").AsString())

	require.NotNil(t, fragments[len(fragments)-1].TokenUsage)
	assert.Equal(t, 10, fragments[len(fragments)-1].TokenUsage.InputTokens)
	assert.Equal(t, 2, fragments[len(fragments)-1].TokenUsage.InputCachedTokens)
	assert.Equal(t, 8, fragments[len(fragments)-1].TokenUsage.InputNonCachedTokens)
	assert.Equal(t, 5, fragments[len(fragments)-1].TokenUsage.OutputTokens)
	assert.Equal(t, 15, fragments[len(fragments)-1].TokenUsage.TotalTokens)
	assert.Equal(t, 15, fragments[len(fragments)-1].ContextLengthTokens)
}

func TestJetBrainsAuthHelpers(t *testing.T) {
	tests := []struct {
		name            string
		config          *conf.ModelProviderConfig
		expectedBearer  string
		expectedHeader  string
		expectedJWT     string
		preHeader       string
		expectSameValue bool
	}{
		{
			name: "prefers refresh token",
			config: &conf.ModelProviderConfig{
				APIKey:       "jwt",
				RefreshToken: "refresh-token",
				Headers: map[string]string{
					jetbrainsAccessTokenHeader: "header-token",
				},
			},
			expectedBearer: "refresh-token",
			expectedHeader: "refresh-token",
			expectedJWT:    "jwt",
		},
		{
			name: "uses jb-access-token header value",
			config: &conf.ModelProviderConfig{
				Headers: map[string]string{
					jetbrainsAccessTokenHeader: "header-token",
				},
			},
			expectedBearer: "header-token",
			expectedHeader: "header-token",
		},
		{
			name: "uses x-jetbrains-bearer header value",
			config: &conf.ModelProviderConfig{
				Headers: map[string]string{
					jetbrainsFallbackBrowserTokenHeader: "browser-token",
				},
			},
			expectedBearer: "browser-token",
			expectedHeader: "browser-token",
		},
		{
			name: "uses authorization header and strips bearer prefix",
			config: &conf.ModelProviderConfig{
				Headers: map[string]string{
					"Authorization": "Bearer auth-token",
				},
			},
			expectedBearer: "auth-token",
			expectedHeader: "auth-token",
		},
		{
			name: "does not override already present jb access token header",
			config: &conf.ModelProviderConfig{
				RefreshToken: "refresh-token",
			},
			expectedBearer: "refresh-token",
			preHeader:       "preset",
			expectedHeader:  "preset",
			expectSameValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedBearer, extractJetBrainsBearerToken(tt.config))

			req, err := http.NewRequest(http.MethodPost, "http://localhost", nil)
			require.NoError(t, err)
			if tt.preHeader != "" {
				req.Header.Set(jetbrainsAccessTokenHeader, tt.preHeader)
			}

			setJetBrainsAuthHeaders(req, tt.config)

			if tt.expectSameValue {
				assert.Equal(t, tt.preHeader, req.Header.Get(jetbrainsAccessTokenHeader))
			} else {
				assert.Equal(t, tt.expectedHeader, req.Header.Get(jetbrainsAccessTokenHeader))
			}

			if tt.expectedJWT != "" {
				assert.Equal(t, tt.expectedJWT, req.Header.Get(jetbrainsAuthenticateJWTHeader))
			}
		})
	}

	t.Run("normalizeBearerToken strips case-insensitive prefix", func(t *testing.T) {
		assert.Equal(t, "value", normalizeBearerToken("Bearer value"))
		assert.Equal(t, "value", normalizeBearerToken("bearer value"))
		assert.Equal(t, "value", normalizeBearerToken("value"))
	})

	t.Run("getConfiguredHeaderValue uses case-insensitive key", func(t *testing.T) {
		headers := map[string]string{"AUTHORIZATION": "token"}
		assert.Equal(t, "token", getConfiguredHeaderValue(headers, "authorization"))
		assert.Equal(t, "", getConfiguredHeaderValue(nil, "authorization"))
	})
}
