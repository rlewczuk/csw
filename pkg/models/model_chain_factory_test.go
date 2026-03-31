package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProviderModelChain(t *testing.T) {
	tests := []struct {
		name      string
		modelSpec string
		expected  []ProviderModelRef
		errText   string
	}{
		{
			name:      "single model",
			modelSpec: "provider1/model1",
			expected: []ProviderModelRef{
				{Provider: "provider1", Model: "model1"},
			},
		},
		{
			name:      "multiple models with spaces",
			modelSpec: "provider1/model1, provider2/model2",
			expected: []ProviderModelRef{
				{Provider: "provider1", Model: "model1"},
				{Provider: "provider2", Model: "model2"},
			},
		},
		{
			name:      "empty spec",
			modelSpec: "   ",
			errText:   "model spec cannot be empty",
		},
		{
			name:      "invalid segment",
			modelSpec: "provider1/model1,invalid",
			errText:   "invalid model format",
		},
		{
			name:      "empty segment",
			modelSpec: "provider1/model1,   ",
			errText:   "empty model segment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := ParseProviderModelChain(tt.modelSpec)
			if tt.errText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, refs)
		})
	}
}

func TestNewChatModelFromProviderChain(t *testing.T) {
	tests := []struct {
		name            string
		modelSpec       string
		providers       map[string]ModelProvider
		expectFallback  bool
		expectedModels  []string
		expectedErrText string
	}{
		{
			name:      "single model returns direct model",
			modelSpec: "provider1/model1",
			providers: map[string]ModelProvider{
				"provider1": NewMockProvider([]ModelInfo{{Name: "model1"}}),
			},
			expectFallback: false,
		},
		{
			name:      "multiple models returns fallback wrapper",
			modelSpec: "provider1/model1,provider2/model2",
			providers: map[string]ModelProvider{
				"provider1": NewMockProvider([]ModelInfo{{Name: "model1"}}),
				"provider2": NewMockProvider([]ModelInfo{{Name: "model2"}}),
			},
			expectFallback: true,
			expectedModels: []string{"model1", "model2"},
		},
		{
			name:      "missing provider returns error",
			modelSpec: "provider1/model1,provider2/model2",
			providers: map[string]ModelProvider{
				"provider1": NewMockProvider([]ModelInfo{{Name: "model1"}}),
			},
			expectedErrText: "provider not found: provider2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatModel, err := NewChatModelFromProviderChain(tt.modelSpec, tt.providers, nil)
			if tt.expectedErrText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrText)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, chatModel)

			fallback, isFallback := chatModel.(*FallbackChatModel)
			assert.Equal(t, tt.expectFallback, isFallback)
			if !tt.expectFallback {
				assert.NotNil(t, chatModel)
				return
			}

			require.NotNil(t, fallback)
			require.Len(t, fallback.models, len(tt.expectedModels))
			for i := range fallback.models {
				mockModel, ok := fallback.models[i].(*MockChatModel)
				require.True(t, ok)
				assert.Equal(t, tt.expectedModels[i], mockModel.model)
			}
		})
	}
}
