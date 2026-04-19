package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderTestCommand_EmptyStreamOnError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csw-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, ".csw", "config")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	var output bytes.Buffer
	output.WriteString("Response: ")

	mockProvider := NewMockProviderWithEmptyStream()
	chatModel := mockProvider.ChatModel("test-model", nil)

	stream := chatModel.ChatStream(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "test"),
	}, nil, nil)

	hasContent := false
	for fragment := range stream {
		output.WriteString(fragment.GetText())
		hasContent = true
	}
	output.WriteString("\n")

	result := output.String()
	assert.Equal(t, "Response: \n", result)
	assert.False(t, hasContent, "Stream should be empty when error occurs")
}

// NewMockProviderWithEmptyStream creates a mock provider that simulates
// an error condition by returning an empty stream (no fragments).
func NewMockProviderWithEmptyStream() *models.MockClient {
	provider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	provider.SetChatResponse("test-model", &models.MockChatResponse{Error: errors.New("simulated authentication error")})
	return provider
}

func TestProviderTestCommand_NonStreamingMode(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, "I am a test assistant."),
	})

	chatModel := mockProvider.ChatModel("test-model", nil)
	response, err := chatModel.Chat(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence."),
	}, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "I am a test assistant.", response.GetText())
}

func TestProviderTestCommand_StreamingMode(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		StreamFragments: []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleAssistant, "I am "),
			models.NewTextMessage(models.ChatRoleAssistant, "a test "),
			models.NewTextMessage(models.ChatRoleAssistant, "assistant."),
		},
	})

	chatModel := mockProvider.ChatModel("test-model", nil)
	stream := chatModel.ChatStream(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence."),
	}, nil, nil)

	var result string
	for fragment := range stream {
		result += fragment.GetText()
	}

	assert.Equal(t, "I am a test assistant.", result)
}

func TestProviderTestCommand_NonStreamingError(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Error: errors.New("authentication failed")})

	chatModel := mockProvider.ChatModel("test-model", nil)
	_, callErr := chatModel.Chat(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence."),
	}, nil, nil)

	assert.Error(t, callErr)
	assert.Contains(t, callErr.Error(), "authentication failed")
}

func TestProviderTestCommand_NonStreamingEmptyResponse(t *testing.T) {
	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, ""),
	})

	chatModel := mockProvider.ChatModel("test-model", nil)
	response, err := chatModel.Chat(context.Background(), []*models.ChatMessage{
		models.NewTextMessage(models.ChatRoleUser, "Please introduce yourself in one sentence."),
	}, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "", response.GetText())
}
