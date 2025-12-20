package ollama

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultOllamaHost  = "http://localhost:11434"
	testModelName      = "devstral-small-2:latest"
	testEmbedModelName = "nomic-embed-text:latest"
	testTimeout        = 30 * time.Second
	connectTimeout     = 5 * time.Second
)

// getOllamaHost returns the Ollama host URL from environment variable or default
func getOllamaHost() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return defaultOllamaHost
}

// skipIfNoOllama skips the test if OLLAMA_HOST environment variable is not set
func skipIfNoOllama(t *testing.T) string {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		t.Skip("Skipping test: OLLAMA_HOST environment variable not set")
	}
	return host
}

func TestNewOllamaClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewOllamaClient(getOllamaHost(), &models.ModelConnectionOptions{
			ConnectTimeout: connectTimeout,
			RequestTimeout: testTimeout,
		})

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("creates client with nil options", func(t *testing.T) {
		client, err := NewOllamaClient(getOllamaHost(), nil)

		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty host", func(t *testing.T) {
		_, err := NewOllamaClient("", nil)

		assert.Error(t, err)
	})
}

func TestOllamaClient_ListModels(t *testing.T) {
	host := skipIfNoOllama(t)

	client, err := NewOllamaClient(host, &models.ModelConnectionOptions{
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
			assert.Greater(t, model.Size, int64(0))
		}
	})

	t.Run("finds test model in list", func(t *testing.T) {
		modelList, err := client.ListModels()

		require.NoError(t, err)

		found := false
		for _, model := range modelList {
			if model.Name == testModelName {
				found = true
				assert.NotEmpty(t, model.Family)
				break
			}
		}

		assert.True(t, found, "expected test model %s to be available", testModelName)
	})
}

func TestOllamaClient_ChatModel(t *testing.T) {
	host := skipIfNoOllama(t)

	client, err := NewOllamaClient(host, &models.ModelConnectionOptions{
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

func TestOllamaClient_Chat(t *testing.T) {
	host := skipIfNoOllama(t)

	client, err := NewOllamaClient(host, &models.ModelConnectionOptions{
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)
	client.SetModel(testModelName)

	ctx := context.Background()

	t.Run("sends chat message and gets response (deprecated)", func(t *testing.T) {
		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"What is 2+2? Answer with just the number."},
			},
		}

		options := &models.ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
			TopK:        40,
		}

		response, err := client.Chat(ctx, messages, options)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
		assert.NotEmpty(t, response.Parts)
		assert.Greater(t, len(response.Parts[0]), 0)
	})

	t.Run("handles context with timeout (deprecated)", func(t *testing.T) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		messages := []*models.ChatMessage{
			{
				Role:  models.ChatRoleUser,
				Parts: []string{"Say hello"},
			},
		}

		response, err := client.Chat(ctxWithTimeout, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("handles system and user messages (deprecated)", func(t *testing.T) {
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

		response, err := client.Chat(ctx, messages, nil)

		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, models.ChatRoleAssistant, response.Role)
	})

	t.Run("returns error for empty messages (deprecated)", func(t *testing.T) {
		response, err := client.Chat(ctx, []*models.ChatMessage{}, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})

	t.Run("returns error for nil messages (deprecated)", func(t *testing.T) {
		response, err := client.Chat(ctx, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, response)
	})
}

func TestOllamaClient_EmbeddingModel(t *testing.T) {
	host := skipIfNoOllama(t)

	client, err := NewOllamaClient(host, &models.ModelConnectionOptions{
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("creates embedding model with model name", func(t *testing.T) {
		embedModel := client.EmbeddingModel(testEmbedModelName)

		assert.NotNil(t, embedModel)
	})

	t.Run("generates embeddings for text", func(t *testing.T) {
		embedModel := client.EmbeddingModel(testEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "Hello, world!")

		require.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.NotEmpty(t, embedding)
		assert.Greater(t, len(embedding), 0)
	})

	t.Run("generates embeddings for different texts", func(t *testing.T) {
		embedModel := client.EmbeddingModel(testEmbedModelName)

		embedding1, err := embedModel.Embed(ctx, "The quick brown fox")
		require.NoError(t, err)
		assert.NotNil(t, embedding1)

		embedding2, err := embedModel.Embed(ctx, "jumps over the lazy dog")
		require.NoError(t, err)
		assert.NotNil(t, embedding2)

		// Embeddings should have the same dimension
		assert.Equal(t, len(embedding1), len(embedding2))
	})

	t.Run("returns error for empty input", func(t *testing.T) {
		embedModel := client.EmbeddingModel(testEmbedModelName)

		embedding, err := embedModel.Embed(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, embedding)
	})
}

func TestOllamaClient_Embed(t *testing.T) {
	host := skipIfNoOllama(t)

	client, err := NewOllamaClient(host, &models.ModelConnectionOptions{
		ConnectTimeout: connectTimeout,
		RequestTimeout: testTimeout,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("generates embeddings for text (deprecated)", func(t *testing.T) {
		client.SetModel(testEmbedModelName)
		embedding, err := client.Embed(ctx, "Hello, world!")

		require.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.NotEmpty(t, embedding)
		assert.Greater(t, len(embedding), 0)
	})

	t.Run("returns error for empty input (deprecated)", func(t *testing.T) {
		client.SetModel(testEmbedModelName)
		embedding, err := client.Embed(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, embedding)
	})
}

func TestOllamaClient_ErrorHandling(t *testing.T) {
	t.Run("handles endpoint not found", func(t *testing.T) {
		client, err := NewOllamaClient("http://beha:11434/nonexistent", &models.ModelConnectionOptions{
			ConnectTimeout: connectTimeout,
			RequestTimeout: testTimeout,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, models.ErrEndpointNotFound)
	})

	t.Run("handles endpoint unavailable", func(t *testing.T) {
		client, err := NewOllamaClient("http://nonexistent-host:11434", &models.ModelConnectionOptions{
			ConnectTimeout: 1 * time.Second,
			RequestTimeout: 2 * time.Second,
		})
		require.NoError(t, err)

		_, err = client.ListModels()
		assert.Error(t, err)
		assert.ErrorIs(t, err, models.ErrEndpointUnavailable)
	})
}
