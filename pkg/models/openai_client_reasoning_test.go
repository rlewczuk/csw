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

func TestOpenAIClient_ReasoningContent(t *testing.T) {
	t.Run("streams reasoning content in same message as text", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"The user"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" wants"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" help."}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":"Sure!"}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":" I can help."}}]}`,
			`data: {"id":"chatcmpl-reasoning-1","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":""}}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Help me"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage

		for fragment := range iterator {
			require.NotNil(t, fragment)
			fragments = append(fragments, fragment)
		}

		assert.NotEmpty(t, fragments, "expected fragments")

		for _, f := range fragments {
			assert.Equal(t, ChatRoleAssistant, f.Role, "all fragments should have assistant role")
		}

		var reasoningContent string
		var textContent string
		for _, f := range fragments {
			for _, part := range f.Parts {
				if part.ReasoningContent != "" {
					reasoningContent += part.ReasoningContent
				}
				if part.Text != "" {
					textContent += part.Text
				}
			}
		}

		assert.Equal(t, "The user wants help.", reasoningContent, "reasoning content should be accumulated")
		assert.Equal(t, "Sure! I can help.", textContent, "text content should be accumulated")
	})

	t.Run("streams reasoning content with tool calls in same message", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"I need"}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" to read"}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"reasoning_content":" the file."}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_123","index":0,"type":"function","function":{"name":"read","arguments":"{\"filePath\":\"/test/file.go\"}"}}]}}]}`,
			`data: {"id":"chatcmpl-reasoning-2","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"finish_reason":"tool_calls","delta":{"role":"assistant","content":""}}]}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Read the file"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*ChatMessage

		for fragment := range iterator {
			require.NotNil(t, fragment)
			fragments = append(fragments, fragment)
		}

		assert.NotEmpty(t, fragments, "expected fragments")

		for _, f := range fragments {
			assert.Equal(t, ChatRoleAssistant, f.Role, "all fragments should have assistant role")
		}

		var reasoningContent string
		var toolCalls []*tool.ToolCall
		for _, f := range fragments {
			for _, part := range f.Parts {
				if part.ReasoningContent != "" {
					reasoningContent += part.ReasoningContent
				}
				if part.ToolCall != nil {
					toolCalls = append(toolCalls, part.ToolCall)
				}
			}
		}

		assert.Equal(t, "I need to read the file.", reasoningContent, "reasoning content should be accumulated")
		assert.Len(t, toolCalls, 1, "expected one tool call")
		assert.Equal(t, "call_123", toolCalls[0].ID)
		assert.Equal(t, "read", toolCalls[0].Function)
	})

	t.Run("includes stream_options in request", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddStreamingResponse("/chat/completions", "POST", true,
			`data: {"id":"chatcmpl-opts","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
			`data: {"id":"chatcmpl-opts","object":"chat.completion.chunk","created":1771161940,"model":"glm-5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"data: [DONE]",
		)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hi"),
		}

		iterator := chatModel.ChatStream(context.Background(), messages, nil, nil)
		for range iterator {
		}

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		assert.True(t, chatReq.Stream, "stream should be true")
		require.NotNil(t, chatReq.StreamOptions, "stream_options should not be nil")
		assert.True(t, chatReq.StreamOptions.IncludeUsage, "include_usage should be true")
	})

	t.Run("non-streaming response includes reasoning content", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-3","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"I should help.","content":"Sure!"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Help me"),
		}

		response, err := chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, ChatRoleAssistant, response.Role)
		require.NotEmpty(t, response.Parts, "expected at least one part")

		var reasoningContent string
		var textContent string
		for _, part := range response.Parts {
			if part.ReasoningContent != "" {
				reasoningContent += part.ReasoningContent
			}
			if part.Text != "" {
				textContent += part.Text
			}
		}

		assert.Equal(t, "I should help.", reasoningContent, "reasoning content should be extracted")
		assert.Equal(t, "Sure!", textContent, "text content should be extracted")
	})

	t.Run("sends reasoning content back to LLM in subsequent request", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-4","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)

		previousAssistantMsg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{ReasoningContent: "I thought about this.", Text: "Hello"},
			},
		}
		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Hi"),
			previousAssistantMsg,
			NewTextMessage(ChatRoleUser, "Continue"),
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		require.Len(t, chatReq.Messages, 3)
		assistantMsg := chatReq.Messages[1]
		assert.Equal(t, "assistant", assistantMsg.Role)
		assert.Equal(t, "Hello", assistantMsg.Content)
		assert.Equal(t, "I thought about this.", assistantMsg.ReasoningContent, "reasoning content should be sent back to LLM")
	})

	t.Run("sends reasoning content with tool calls back to LLM", func(t *testing.T) {
		mock := testutil.NewMockHTTPServer()
		defer mock.Close()

		client, err := NewOpenAIClient(&conf.ModelProviderConfig{
			URL: mock.URL(),
		})
		require.NoError(t, err)

		mock.AddRestResponse("/chat/completions", "POST", `{"id":"chatcmpl-reasoning-5","object":"chat.completion","created":1771161940,"model":"glm-5","choices":[{"index":0,"message":{"role":"assistant","content":"Done"},"finish_reason":"stop"}]}`)

		chatModel := client.ChatModel("glm-5", nil)

		previousAssistantMsg := &ChatMessage{
			Role: ChatRoleAssistant,
			Parts: []ChatMessagePart{
				{ReasoningContent: "I need to call a tool."},
				{ToolCall: &tool.ToolCall{
					ID:        "call_abc",
					Function:  "test_func",
					Arguments: tool.NewToolValue(map[string]any{"arg": "value"}),
				}},
			},
		}
		toolResultMsg := NewToolResponseMessage(&tool.ToolResponse{
			Call:   &tool.ToolCall{ID: "call_abc"},
			Result: tool.NewToolValue("result"),
			Done:   true,
		})

		messages := []*ChatMessage{
			NewTextMessage(ChatRoleUser, "Do something"),
			previousAssistantMsg,
			toolResultMsg,
		}

		_, err = chatModel.Chat(context.Background(), messages, nil, nil)
		require.NoError(t, err)

		reqs := mock.GetRequests()
		require.Len(t, reqs, 1)

		var chatReq OpenaiChatCompletionRequest
		err = json.Unmarshal(reqs[0].Body, &chatReq)
		require.NoError(t, err)

		require.Len(t, chatReq.Messages, 3)
		assistantMsg := chatReq.Messages[1]
		assert.Equal(t, "assistant", assistantMsg.Role)
		assert.Equal(t, "I need to call a tool.", assistantMsg.ReasoningContent, "reasoning content should be sent back with tool calls")
		require.Len(t, assistantMsg.ToolCalls, 1)
		assert.Equal(t, "call_abc", assistantMsg.ToolCalls[0].ID)
	})
}
