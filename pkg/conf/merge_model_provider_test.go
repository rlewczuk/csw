package conf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelProviderConfig_Clone_NilReceiver(t *testing.T) {
	var cfg *ModelProviderConfig
	assert.Nil(t, cfg.Clone())
}

func TestModelProviderConfig_Clone_PreservesDurations(t *testing.T) {
	cfg := &ModelProviderConfig{
		ConnectTimeout:        5,
		RequestTimeout:        15,
		RateLimitBackoffScale: 2,
	}

	clone := cfg.Clone()
	require.NotNil(t, clone)
	assert.Equal(t, 5, clone.ConnectTimeout)
	assert.Equal(t, 15, clone.RequestTimeout)
	assert.Equal(t, 2, clone.RateLimitBackoffScale)
}

func TestModelProviderConfig_Merge(t *testing.T) {
	streaming := true
	streamingOverride := false
	base := &ModelProviderConfig{
		Type:      "openai",
		Name:      "base",
		URL:       "https://base",
		ModelTags: []ModelTagMapping{{Model: "^gpt", Tag: "openai"}},
		Cost:      []ModelProviderCost{{Context: 0, Input: 1.0, Output: 2.0}},
		Reasoning: map[string]string{"low": "minimal"},
		Headers:   map[string]string{"X-A": "1"},
		Streaming: &streaming,
	}
	override := &ModelProviderConfig{
		URL:       "https://override",
		ModelTags: []ModelTagMapping{{Model: "^gpt", Tag: "general"}, {Model: "^o", Tag: "reasoning"}},
		Cost: []ModelProviderCost{
			{Context: 0, Input: 1.5},
			{Context: 200000, Input: 3.0, Output: 4.0},
		},
		Reasoning:      map[string]string{"high": "deep"},
		Headers:        map[string]string{"X-B": "2"},
		Streaming:      &streamingOverride,
		DisableRefresh: true,
	}

	base.Merge(override)

	assert.Equal(t, "https://override", base.URL)
	assert.Len(t, base.ModelTags, 2)
	assert.Equal(t, "general", base.ModelTags[0].Tag)
	assert.Equal(t, "reasoning", base.ModelTags[1].Tag)
	assert.Len(t, base.Cost, 2)
	assert.Equal(t, 0, base.Cost[0].Context)
	assert.Equal(t, 1.5, base.Cost[0].Input)
	assert.Equal(t, 200000, base.Cost[1].Context)
	assert.Equal(t, "minimal", base.Reasoning["low"])
	assert.Equal(t, "deep", base.Reasoning["high"])
	assert.Equal(t, "1", base.Headers["X-A"])
	assert.Equal(t, "2", base.Headers["X-B"])
	assert.False(t, *base.Streaming)
	assert.True(t, base.DisableRefresh)
}

func TestConfigCloneMethods_DeepCopy_ModelProvider(t *testing.T) {
	streaming := true
	temperature := true
	experimental := true
	cfg := &ModelProviderConfig{
		Name:         "provider",
		ModelTags:    []ModelTagMapping{{Model: ".*", Tag: "x"}},
		Streaming:    &streaming,
		Temperature:  &temperature,
		Experimental: &experimental,
		Reasoning:    map[string]string{"low": "minimal"},
		Headers:      map[string]string{"X": "1"},
		QueryParams:  map[string]string{"q": "1"},
		Options:      map[string]any{"a": 1},
		Cost:         []ModelProviderCost{{Context: 0, Input: 1.0}, {Context: 200000, Input: 2.0}},
	}

	clone := cfg.Clone()
	require.NotNil(t, clone)

	clone.ModelTags[0].Tag = "y"
	*clone.Streaming = false
	*clone.Temperature = false
	*clone.Experimental = false
	clone.Reasoning["low"] = "changed"
	clone.Headers["X"] = "2"
	clone.QueryParams["q"] = "2"
	clone.Options["a"] = 2
	clone.Cost[0].Input = 99

	assert.Equal(t, "x", cfg.ModelTags[0].Tag)
	assert.True(t, *cfg.Streaming)
	assert.True(t, *cfg.Temperature)
	assert.True(t, *cfg.Experimental)
	assert.Equal(t, "minimal", cfg.Reasoning["low"])
	assert.Equal(t, "1", cfg.Headers["X"])
	assert.Equal(t, "1", cfg.QueryParams["q"])
	assert.Equal(t, 1, cfg.Options["a"])
	assert.Equal(t, 1.0, cfg.Cost[0].Input)
}
