package models

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIClient_RequestPrefixStability(t *testing.T) {
	mock := testutil.NewMockHTTPServer()
	defer mock.Close()

	client, err := NewOpenAIClient(&conf.ModelProviderConfig{
		URL:    mock.URL(),
		APIKey: "test-key",
		Options: map[string]any{
			"prompt_cache_retention": "24h",
		},
	})
	require.NoError(t, err)

	mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-stable-1","object":"chat.completion","created":1640000000,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-stable-2","object":"chat.completion","created":1640000001,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)

	chatModel := client.ChatModel("test-model", &ChatOptions{SessionID: "session-prefix-a"})

	toolsA := []tool.ToolInfo{
		buildOpenAIRequestStabilityToolInfo("zeta_tool", []string{"beta", "alpha"}),
		buildOpenAIRequestStabilityToolInfo("alpha_tool", []string{"gamma", "alpha"}),
	}
	toolsB := []tool.ToolInfo{
		buildOpenAIRequestStabilityToolInfo("alpha_tool", []string{"alpha", "gamma"}),
		buildOpenAIRequestStabilityToolInfo("zeta_tool", []string{"alpha", "beta"}),
	}

	_, err = chatModel.Chat(context.Background(), buildOpenAIRequestStabilityMessages(true), nil, toolsA)
	require.NoError(t, err)
	_, err = chatModel.Chat(context.Background(), buildOpenAIRequestStabilityMessages(false), nil, toolsB)
	require.NoError(t, err)

	reqs := mock.GetRequests()
	require.Len(t, reqs, 2)
	assert.Equal(t, string(reqs[0].Body), string(reqs[1].Body), "equivalent inputs should produce byte-identical request prefix")

	var requestBody map[string]any
	err = json.Unmarshal(reqs[0].Body, &requestBody)
	require.NoError(t, err)
	assert.Equal(t, "session-prefix-a", requestBody["prompt_cache_key"])
	assert.Equal(t, "24h", requestBody["prompt_cache_retention"])

	messagesRaw, ok := requestBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messagesRaw, 3)

	assistantMessage, ok := messagesRaw[1].(map[string]any)
	require.True(t, ok)
	toolCallsRaw, ok := assistantMessage["tool_calls"].([]any)
	require.True(t, ok)
	require.Len(t, toolCallsRaw, 1)

	firstCall, ok := toolCallsRaw[0].(map[string]any)
	require.True(t, ok)
	firstCallFunction, ok := firstCall["function"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, `{"alpha":"A","beta":"B"}`, firstCallFunction["arguments"])

	toolResponseMessage, ok := messagesRaw[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, `{"alpha":1,"beta":2}`, toolResponseMessage["content"])

	toolsRaw, ok := requestBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, toolsRaw, 2)

	firstTool, ok := toolsRaw[0].(map[string]any)
	require.True(t, ok)
	firstToolFunction, ok := firstTool["function"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alpha_tool", firstToolFunction["name"])

	firstToolParams, ok := firstToolFunction["parameters"].(map[string]any)
	require.True(t, ok)
	firstToolRequired, ok := firstToolParams["required"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"alpha", "gamma"}, firstToolRequired)
}

func buildOpenAIRequestStabilityMessages(alphaFirst bool) []*ChatMessage {
	toolCallArgs := map[string]any{}
	toolResult := map[string]any{}
	if alphaFirst {
		toolCallArgs["alpha"] = "A"
		toolCallArgs["beta"] = "B"
		toolResult["alpha"] = 1
		toolResult["beta"] = 2
	} else {
		toolCallArgs["beta"] = "B"
		toolCallArgs["alpha"] = "A"
		toolResult["beta"] = 2
		toolResult["alpha"] = 1
	}

	toolCall := &tool.ToolCall{
		ID:        "call_stable_1",
		Function:  "calculate",
		Arguments: tool.NewToolValue(toolCallArgs),
	}

	toolResponse := &tool.ToolResponse{
		Call:   &tool.ToolCall{ID: "call_stable_1"},
		Result: tool.NewToolValue(toolResult),
		Done:   true,
	}

	return []*ChatMessage{
		NewTextMessage(ChatRoleUser, "hello"),
		NewToolCallMessage(toolCall),
		NewToolResponseMessage(toolResponse),
	}
}

func buildOpenAIRequestStabilityToolInfo(name string, required []string) tool.ToolInfo {
	allowAdditionalProperties := false
	return tool.ToolInfo{
		Name:        name,
		Description: "request stability test tool",
		Schema: tool.ToolSchema{
			Type:                 tool.SchemaTypeObject,
			AdditionalProperties: false,
			Required:             required,
			Properties: map[string]tool.PropertySchema{
				"alpha": {
					Type:        tool.SchemaTypeString,
					Description: "alpha argument",
				},
				"gamma": {
					Type:        tool.SchemaTypeArray,
					Description: "gamma list",
					Items: &tool.PropertySchema{
						Type:                 tool.SchemaTypeObject,
						AdditionalProperties: &allowAdditionalProperties,
						Required:             []string{"zeta", "beta"},
						Properties: map[string]tool.PropertySchema{
							"zeta": {
								Type:        tool.SchemaTypeString,
								Description: "zeta field",
							},
							"beta": {
								Type:        tool.SchemaTypeString,
								Description: "beta field",
							},
						},
					},
				},
			},
		},
	}
}
