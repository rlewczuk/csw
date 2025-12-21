package mock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/models"
)

func TestMockProvider_ListModels(t *testing.T) {
	testModels := []models.ModelInfo{
		{
			Name:       "mock-model-1",
			Model:      "mock-model-1",
			ModifiedAt: "2024-01-01",
			Size:       1000,
			Family:     "mock",
		},
		{
			Name:       "mock-model-2",
			Model:      "mock-model-2",
			ModifiedAt: "2024-01-02",
			Size:       2000,
			Family:     "mock",
		},
	}

	provider := NewMockProvider(testModels)
	models, err := provider.ListModels()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if models[0].Name != "mock-model-1" {
		t.Errorf("expected first model name 'mock-model-1', got '%s'", models[0].Name)
	}

	if models[1].Name != "mock-model-2" {
		t.Errorf("expected second model name 'mock-model-2', got '%s'", models[1].Name)
	}
}

func TestMockChatModel_Chat(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})

	testCases := []struct {
		name          string
		modelName     string
		setupResponse *ChatResponse
		expectError   bool
		expectedParts []string
	}{
		{
			name:          "default response",
			modelName:     "test-model",
			expectedParts: []string{"mock response"},
		},
		{
			name:      "custom response",
			modelName: "test-model",
			setupResponse: &ChatResponse{
				Response: &models.ChatMessage{
					Role:  models.ChatRoleAssistant,
					Parts: []string{"Hello, ", "world!"},
				},
			},
			expectedParts: []string{"Hello, ", "world!"},
		},
		{
			name:      "error response",
			modelName: "test-model",
			setupResponse: &ChatResponse{
				Error: errors.New("test error"),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupResponse != nil {
				provider.SetChatResponse(tc.modelName, tc.setupResponse)
			}

			chatModel := provider.ChatModel(tc.modelName, nil)
			ctx := context.Background()

			messages := []*models.ChatMessage{
				{Role: models.ChatRoleUser, Parts: []string{"test"}},
			}

			response, err := chatModel.Chat(ctx, messages, nil)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(response.Parts) != len(tc.expectedParts) {
				t.Fatalf("expected %d parts, got %d", len(tc.expectedParts), len(response.Parts))
			}

			for i, part := range tc.expectedParts {
				if response.Parts[i] != part {
					t.Errorf("part %d: expected '%s', got '%s'", i, part, response.Parts[i])
				}
			}
		})
	}
}

func TestMockChatModel_Chat_Cancellation(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})
	chatModel := provider.ChatModel("test-model", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []*models.ChatMessage{
		{Role: models.ChatRoleUser, Parts: []string{"test"}},
	}

	_, err := chatModel.Chat(ctx, messages, nil)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestMockChatModel_ChatStream(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})

	testCases := []struct {
		name            string
		modelName       string
		setupResponse   *ChatResponse
		expectError     bool
		expectedFrags   int
		expectedContent []string
	}{
		{
			name:            "default stream",
			modelName:       "test-model",
			expectedFrags:   2,
			expectedContent: []string{"mock", " stream"},
		},
		{
			name:      "custom stream",
			modelName: "test-model",
			setupResponse: &ChatResponse{
				StreamFragments: []*models.ChatMessage{
					{Role: models.ChatRoleAssistant, Parts: []string{"Hello"}},
					{Role: models.ChatRoleAssistant, Parts: []string{", "}},
					{Role: models.ChatRoleAssistant, Parts: []string{"world"}},
					{Role: models.ChatRoleAssistant, Parts: []string{"!"}},
				},
			},
			expectedFrags:   4,
			expectedContent: []string{"Hello", ", ", "world", "!"},
		},
		{
			name:      "error on stream creation",
			modelName: "test-model",
			setupResponse: &ChatResponse{
				Error: errors.New("stream error"),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupResponse != nil {
				provider.SetChatResponse(tc.modelName, tc.setupResponse)
			}

			chatModel := provider.ChatModel(tc.modelName, nil)
			ctx := context.Background()

			messages := []*models.ChatMessage{
				{Role: models.ChatRoleUser, Parts: []string{"test"}},
			}

			iter, err := chatModel.ChatStream(ctx, messages, nil)

			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer iter.Close()

			// Read all fragments
			var fragments []*models.ChatMessage
			for {
				msg, err := iter.Next()
				if err == models.ErrEndOfStream {
					break
				}
				if err != nil {
					t.Fatalf("unexpected error during iteration: %v", err)
				}
				fragments = append(fragments, msg)
			}

			if len(fragments) != tc.expectedFrags {
				t.Fatalf("expected %d fragments, got %d", tc.expectedFrags, len(fragments))
			}

			for i, content := range tc.expectedContent {
				if fragments[i].Parts[0] != content {
					t.Errorf("fragment %d: expected '%s', got '%s'", i, content, fragments[i].Parts[0])
				}
			}

			// Verify ErrEndOfStream after all fragments
			_, err = iter.Next()
			if err != models.ErrEndOfStream {
				t.Errorf("expected ErrEndOfStream, got %v", err)
			}
		})
	}
}

func TestMockChatModel_ChatStream_Cancellation(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})
	provider.SetChatResponse("test-model", &ChatResponse{
		StreamFragments: []*models.ChatMessage{
			{Role: models.ChatRoleAssistant, Parts: []string{"fragment 1"}},
			{Role: models.ChatRoleAssistant, Parts: []string{"fragment 2"}},
			{Role: models.ChatRoleAssistant, Parts: []string{"fragment 3"}},
		},
	})

	chatModel := provider.ChatModel("test-model", nil)

	ctx, cancel := context.WithCancel(context.Background())

	messages := []*models.ChatMessage{
		{Role: models.ChatRoleUser, Parts: []string{"test"}},
	}

	iter, err := chatModel.ChatStream(ctx, messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer iter.Close()

	// Read first fragment
	_, err = iter.Next()
	if err != nil {
		t.Fatalf("unexpected error on first Next(): %v", err)
	}

	// Cancel context
	cancel()

	// Try to read next fragment - should get cancellation error
	_, err = iter.Next()
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestMockChatModel_ChatStream_ContextTimeout(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})
	provider.SetChatResponse("test-model", &ChatResponse{
		StreamFragments: []*models.ChatMessage{
			{Role: models.ChatRoleAssistant, Parts: []string{"fragment 1"}},
		},
	})

	chatModel := provider.ChatModel("test-model", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(10 * time.Millisecond)

	messages := []*models.ChatMessage{
		{Role: models.ChatRoleUser, Parts: []string{"test"}},
	}

	iter, err := chatModel.ChatStream(ctx, messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer iter.Close()

	// Try to read - should get timeout error
	_, err = iter.Next()
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestMockEmbeddingModel_Embed(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})

	testCases := []struct {
		name              string
		modelName         string
		setupEmbedding    []float64
		expectedEmbedding []float64
	}{
		{
			name:              "default embedding",
			modelName:         "test-embed-model",
			expectedEmbedding: []float64{0.1, 0.2, 0.3},
		},
		{
			name:              "custom embedding",
			modelName:         "test-embed-model",
			setupEmbedding:    []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			expectedEmbedding: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupEmbedding != nil {
				provider.SetEmbedResponse(tc.modelName, tc.setupEmbedding)
			}

			embedModel := provider.EmbeddingModel(tc.modelName)
			ctx := context.Background()

			embedding, err := embedModel.Embed(ctx, "test input")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(embedding) != len(tc.expectedEmbedding) {
				t.Fatalf("expected embedding length %d, got %d", len(tc.expectedEmbedding), len(embedding))
			}

			for i, val := range tc.expectedEmbedding {
				if embedding[i] != val {
					t.Errorf("embedding[%d]: expected %f, got %f", i, val, embedding[i])
				}
			}
		})
	}
}

func TestMockEmbeddingModel_Embed_Cancellation(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})
	embedModel := provider.EmbeddingModel("test-model")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := embedModel.Embed(ctx, "test input")
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestMockProvider_InterfaceCompliance(t *testing.T) {
	// This test verifies that MockProvider implements the ModelProvider interface
	var _ models.ModelProvider = (*MockProvider)(nil)
	var _ models.ChatModel = (*MockChatModel)(nil)
	var _ models.EmbeddingModel = (*MockEmbeddingModel)(nil)
	var _ models.ChatStreamIterator = (*MockStreamIterator)(nil)
}

func TestMockProvider_ConcurrentAccess(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{
		{Name: "test-model", Model: "test-model"},
	})

	// Test concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			provider.SetChatResponse("test-model", &ChatResponse{
				Response: &models.ChatMessage{
					Role:  models.ChatRoleAssistant,
					Parts: []string{"response"},
				},
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			chatModel := provider.ChatModel("test-model", nil)
			ctx := context.Background()
			messages := []*models.ChatMessage{
				{Role: models.ChatRoleUser, Parts: []string{"test"}},
			}
			_, _ = chatModel.Chat(ctx, messages, nil)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestMockStreamIterator_Close(t *testing.T) {
	provider := NewMockProvider([]models.ModelInfo{})
	chatModel := provider.ChatModel("test-model", nil)
	ctx := context.Background()

	messages := []*models.ChatMessage{
		{Role: models.ChatRoleUser, Parts: []string{"test"}},
	}

	iter, err := chatModel.ChatStream(ctx, messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should not return error
	err = iter.Close()
	if err != nil {
		t.Errorf("unexpected error on Close(): %v", err)
	}
}
