package anthropic

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultTestURL = "https://api.anthropic.com"
	testModelName  = "claude-sonnet-4-5-20250929"
	testTimeout    = 30 * time.Second
	connectTimeout = 5 * time.Second
	apiKeyFile     = "../../../.anthropic_api_key"
)

// getAPIKey reads the API key from file and skips test if not found
func getAPIKey(t *testing.T) string {
	data, err := os.ReadFile(apiKeyFile)
	if err != nil || len(data) == 0 {
		t.Skipf("Skipping test: %s file not found or empty", apiKeyFile)
	}
	apiKey := strings.TrimSpace(string(data))
	if apiKey == "" {
		t.Skipf("Skipping test: API key in %s is empty", apiKeyFile)
	}
	return apiKey
}

func TestNewAnthropicClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
			APIKey:         "test-api-key",
			ConnectTimeout: connectTimeout,
			RequestTimeout: testTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("creates client with nil options", func(t *testing.T) {
		client, err := NewAnthropicClient(defaultTestURL, nil)

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		_, err := NewAnthropicClient("", nil)

		assert.Error(t, err)
	})
}

func TestAnthropicClient_ListModels(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	t.Run("lists available models", func(t *testing.T) {
		modelList, err := client.ListModels()

		require.NoError(t, err)
		assert.NotNil(t, modelList)
		assert.NotEmpty(t, modelList, "expected at least one model to be available")

		// Verify model info structure
		for _, model := range modelList {
			assert.NotEmpty(t, model.Name)
			assert.NotEmpty(t, model.Model)
		}
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == testModelName {
				found = true
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testModelName)
	})
}

func TestAnthropicClient_ChatModel(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("creates chat model with model name and options", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		assert.NotNil(t, chatModel)
	})

	t.Run("sends chat message and gets response", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"What is 2+2? Answer with just the number."},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
		assert.NotEmpty(t, response.Parts)
		assert.Greater(t, len(response.Parts[0]), 0)
	})

	t.Run("handles context with timeout", func(t *testing.T) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Say hello"},
			},
		}

		response, err := chatModel.Chat(ctxWithTimeout, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles system and user messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleSystem,
				Parts: []string{"You are a helpful assistant that always responds in uppercase."},
			},
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"hello"},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
	})

	t.Run("returns error for empty messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		response, err := chatModel.Chat(ctx, []*models.ChatMessage{}, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		response, err := chatModel.Chat(ctx, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("uses default options when none provided to Chat", func(t *testing.T) {
		defaultOptions := &models.ChatOptions{
			Temperature: 0.5,
			TopP:        0.8,
			TopK:        30,
		}

		chatModel := client.ChatModel(testModelName, defaultOptions)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Say hello"},
			},
		}

		response, err := chatModel.Chat(ctx, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})
}

func TestAnthropicClient_ChatModelStream(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("streams chat message and gets fragments", func(t *testing.T) {
		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		chatModel := client.ChatModel(testModelName, options)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Count from 1 to 5, one number per line."},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			assert.Equal(t, models.ChatRoleAssistant, fragment.Role)
			assert.NotEmpty(t, fragment.Parts)
			fragments = append(fragments, fragment)
		}

		// Should have received multiple fragments
		assert.Greater(t, len(fragments), 0, "expected to receive at least one fragment")
	})

	t.Run("handles context cancellation during streaming", func(t *testing.T) {
		ctxWithCancel, cancel := context.WithCancel(ctx)

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Write a long story about a cat."},
			},
		}

		iterator := chatModel.ChatStream(ctxWithCancel, messages, nil)
		require.NotNil(t, iterator)

		// Read first fragment
		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			// Cancel the context after first fragment
			cancel()
			// Iterator should stop gracefully
			break
		}

		assert.True(t, fragmentReceived, "expected to receive at least one fragment before cancellation")
	})

	t.Run("handles context with timeout during streaming", func(t *testing.T) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Say hello"},
			},
		}

		iterator := chatModel.ChatStream(ctxWithTimeout, messages, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Greater(t, len(fragments), 0)
	})

	t.Run("returns no fragments for empty messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		iterator := chatModel.ChatStream(ctx, []*models.ChatMessage{}, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("returns no fragments for nil messages", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		iterator := chatModel.ChatStream(ctx, nil, nil)
		require.NotNil(t, iterator)

		var fragments []*models.ChatMessage
		for fragment := range iterator {
			fragments = append(fragments, fragment)
		}

		assert.Empty(t, fragments)
	})

	t.Run("iterator can be stopped early", func(t *testing.T) {
		chatModel := client.ChatModel(testModelName, nil)

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Say hello"},
			},
		}

		iterator := chatModel.ChatStream(ctx, messages, nil)
		require.NotNil(t, iterator)

		// Stop reading after first fragment (if any)
		fragmentReceived := false
		for fragment := range iterator {
			assert.NotNil(t, fragment)
			fragmentReceived = true
			break
		}

		// Breaking from range should work gracefully
		assert.True(t, fragmentReceived, "expected to receive at least one fragment")
	})
}

func TestAnthropicClient_EmbeddingModel(t *testing.T) {
	apiKey := getAPIKey(t)

	client, err := NewAnthropicClient(defaultTestURL, &models.ModelConnectionOptions{
		APIKey:         apiKey,
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("returns error for embedding model", func(t *testing.T) {
		embedModel := client.EmbeddingModel("any-model")

		assert.NotNil(t, embedModel)

		// Anthropic doesn't support embeddings, should return an error
		_, err := embedModel.Embed(ctx, "Hello, world!")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

func TestAnthropicClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint unavailable", func(t *testing.T) {
		client, err := NewAnthropicClient("http://nonexistent-host:11434", &models.ModelConnectionOptions{
			APIKey:         "test-key",
			ConnectTimeout: 1 * time.Second,
			RequestTimeout: 2 * time.Second,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, models.ErrEndpointUnavailable)
	})
}
