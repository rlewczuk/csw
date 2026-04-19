package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalAnthropicMessagesRequest_PrefixStableForEquivalentInputs(t *testing.T) {
	requestOne := AnthropicMessagesRequest{
		Model: "claude-test",
		Messages: []AnthropicMessageParam{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type: "tool_use",
						ID:   "call_1",
						Name: "search",
						Input: map[string]interface{}{
							"query": "hello",
							"limit": 3,
						},
					},
				},
			},
		},
		MaxTokens: 1000,
		System:    "system prompt",
		Tools: []AnthropicTool{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
						"limit": map[string]interface{}{"type": "integer"},
					},
					"required": []interface{}{"query", "limit"},
				},
			},
		},
	}

	requestTwo := AnthropicMessagesRequest{
		Model: "claude-test",
		Messages: []AnthropicMessageParam{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type: "tool_use",
						ID:   "call_1",
						Name: "search",
						Input: map[string]interface{}{
							"limit": 3,
							"query": "hello",
						},
					},
				},
			},
		},
		MaxTokens: 1000,
		System:    "system prompt",
		Tools: []AnthropicTool{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]interface{}{
					"required": []interface{}{"query", "limit"},
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{"type": "integer"},
						"query": map[string]interface{}{"type": "string"},
					},
					"type": "object",
				},
			},
		},
	}

	bodyOne, err := marshalAnthropicMessagesRequest(requestOne)
	require.NoError(t, err)
	bodyTwo, err := marshalAnthropicMessagesRequest(requestTwo)
	require.NoError(t, err)

	assert.Equal(t, string(bodyOne), string(bodyTwo))
}

func TestMarshalAnthropicMessagesRequest_AddsPromptCachingBreakpoints(t *testing.T) {
	t.Run("marks last tool when tools are present", func(t *testing.T) {
		request := AnthropicMessagesRequest{
			Model:     "claude-test",
			MaxTokens: 1000,
			Messages:  []AnthropicMessageParam{{Role: "user", Content: "hello"}},
			System:    "system prompt",
			Tools: []AnthropicTool{
				{Name: "first", InputSchema: map[string]interface{}{"type": "object"}},
				{Name: "second", InputSchema: map[string]interface{}{"type": "object"}},
			},
		}

		body, err := marshalAnthropicMessagesRequest(request)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		toolsPayload, ok := payload["tools"].([]interface{})
		require.True(t, ok)
		require.Len(t, toolsPayload, 2)

		firstTool, ok := toolsPayload[0].(map[string]interface{})
		require.True(t, ok)
		_, hasFirstCacheControl := firstTool["cache_control"]
		assert.False(t, hasFirstCacheControl)

		secondTool, ok := toolsPayload[1].(map[string]interface{})
		require.True(t, ok)
		cacheControl, ok := secondTool["cache_control"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ephemeral", cacheControl["type"])
	})

	t.Run("marks system when no tools are present", func(t *testing.T) {
		request := AnthropicMessagesRequest{
			Model:     "claude-test",
			MaxTokens: 1000,
			Messages:  []AnthropicMessageParam{{Role: "user", Content: "hello"}},
			System:    "system prompt",
		}

		body, err := marshalAnthropicMessagesRequest(request)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &payload))

		systemPayload, ok := payload["system"].([]interface{})
		require.True(t, ok)
		require.Len(t, systemPayload, 1)

		block, ok := systemPayload[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "text", block["type"])
		assert.Equal(t, "system prompt", block["text"])

		cacheControl, ok := block["cache_control"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "ephemeral", cacheControl["type"])
	})
}
