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
			chatModel, err := NewChatModelFromProviderChain(tt.modelSpec, tt.providers, nil, nil, nil, nil)
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

func TestNewChatModelFromProviderChain_WithRetryPolicyWrapsModel(t *testing.T) {
	retryPolicy := RetryPolicy{InitialDelay: 0, MaxRetries: 1, MaxDelay: 0}

	chatModel, err := NewChatModelFromProviderChain(
		"provider1/model1",
		map[string]ModelProvider{
			"provider1": NewMockProvider([]ModelInfo{{Name: "model1"}}),
		},
		nil,
		&retryPolicy,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, chatModel)

	_, isRetry := chatModel.(*RetryChatModel)
	assert.True(t, isRetry)
}

func TestExpandProviderModelChain(t *testing.T) {
	tests := []struct {
		name      string
		modelSpec string
		aliases   map[string][]string
		expected  []ProviderModelRef
		errText   string
	}{
		{
			name:      "direct provider model chain",
			modelSpec: "p1/m1,p2/m2",
			expected:  []ProviderModelRef{{Provider: "p1", Model: "m1"}, {Provider: "p2", Model: "m2"}},
		},
		{
			name:      "single alias",
			modelSpec: "fast",
			aliases: map[string][]string{
				"fast": {"p1/m1"},
			},
			expected: []ProviderModelRef{{Provider: "p1", Model: "m1"}},
		},
		{
			name:      "alias with fallback targets",
			modelSpec: "balanced",
			aliases: map[string][]string{
				"balanced": {"p1/m1", "p2/m2"},
			},
			expected: []ProviderModelRef{{Provider: "p1", Model: "m1"}, {Provider: "p2", Model: "m2"}},
		},
		{
			name:      "nested alias",
			modelSpec: "default",
			aliases: map[string][]string{
				"default": {"fast", "p2/m2"},
				"fast":    {"p1/m1"},
			},
			expected: []ProviderModelRef{{Provider: "p1", Model: "m1"}, {Provider: "p2", Model: "m2"}},
		},
		{
			name:      "cyclic aliases",
			modelSpec: "a",
			aliases: map[string][]string{
				"a": {"b"},
				"b": {"a"},
			},
			errText: "cyclic alias reference",
		},
		{
			name:      "unknown alias",
			modelSpec: "unknown",
			aliases: map[string][]string{
				"known": {"p/m"},
			},
			errText: "unknown model alias",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := ExpandProviderModelChain(tt.modelSpec, tt.aliases)
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

func TestComposeProviderModelSpec(t *testing.T) {
	assert.Equal(t, "", ComposeProviderModelSpec(nil))
	assert.Equal(
		t,
		"p1/m1,p2/m2",
		ComposeProviderModelSpec([]ProviderModelRef{{Provider: "p1", Model: "m1"}, {Provider: "p2", Model: "m2"}}),
	)
}
