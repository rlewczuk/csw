package core

import (
	"context"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLLMRetryPolicy(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.CswConfig
		provider models.ModelProvider
		assertFn func(t *testing.T, policy models.RetryPolicy)
	}{
		{
			name:     "uses session defaults when config is empty",
			config:   &conf.CswConfig{},
			provider: models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}}),
			assertFn: func(t *testing.T, policy models.RetryPolicy) {
				assert.Equal(t, defaultLLMRetryMaxAttempts-1, policy.MaxRetries)
				assert.Equal(t, models.DefaultRetryBackoffScale, policy.InitialDelay)
				assert.Equal(t, time.Duration(defaultLLMRetryMaxBackoffSeconds)*models.DefaultRetryBackoffScale, policy.MaxDelay)
			},
		},
		{
			name: "uses global config retry settings",
			config: &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{
				LLMRetryMaxAttempts:       4,
				LLMRetryMaxBackoffSeconds: 17,
			}},
			provider: models.NewMockProvider([]models.ModelInfo{{Name: "test-model"}}),
			assertFn: func(t *testing.T, policy models.RetryPolicy) {
				assert.Equal(t, 3, policy.MaxRetries)
				assert.Equal(t, models.DefaultRetryBackoffScale, policy.InitialDelay)
				assert.Equal(t, 17*time.Second, policy.MaxDelay)
			},
		},
		{
			name:   "uses provider retry settings when global is unset",
			config: &conf.CswConfig{},
			provider: &models.MockClient{
				Config: &conf.ModelProviderConfig{
					MaxRetries:             2,
					RateLimitBackoffScale: 3,
				},
			},
			assertFn: func(t *testing.T, policy models.RetryPolicy) {
				assert.Equal(t, 2, policy.MaxRetries)
				assert.Equal(t, 3*time.Second, policy.InitialDelay)
				assert.Equal(t, time.Duration(defaultLLMRetryMaxBackoffSeconds)*3*time.Second, policy.MaxDelay)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := DefaultLLMRetryPolicy(tt.config, tt.provider)
			tt.assertFn(t, policy)
		})
	}
}

func TestNewGenerationChatModelFromSpecUsesFallbackAndRetry(t *testing.T) {
	primary := models.NewMockProvider([]models.ModelInfo{{Name: "primary"}})
	fallback := models.NewMockProvider([]models.ModelInfo{{Name: "fallback"}})

	primary.SetChatResponse("primary", &models.MockChatResponse{Error: &models.NetworkError{Message: "temporary network issue", IsRetryable: true}})
	fallback.SetChatResponse("fallback", &models.MockChatResponse{Response: models.NewTextMessage(models.ChatRoleAssistant, "fallback success")})

	chatModel, err := NewGenerationChatModelFromSpec(
		"p1/primary,p2/fallback",
		map[string]models.ModelProvider{"p1": primary, "p2": fallback},
		nil,
		&conf.CswConfig{GlobalConfig: &conf.GlobalConfig{LLMRetryMaxAttempts: 2}},
		primary,
		nil,
		nil,
	)
	require.NoError(t, err)

	response, chatErr := chatModel.Chat(context.Background(), []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "test")}, nil, nil)
	require.NoError(t, chatErr)
	assert.Equal(t, "fallback success", response.GetText())
	require.Len(t, primary.RecordedMessages, 1)
	require.Len(t, fallback.RecordedMessages, 1)
}
