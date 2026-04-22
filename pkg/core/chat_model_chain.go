package core

import (
	"fmt"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/shared"
)

// DefaultLLMRetryPolicy builds retry policy for non-session generation calls.
// Policy values match SweSession retry behavior.
func DefaultLLMRetryPolicy(config *conf.CswConfig, provider models.ModelProvider) models.RetryPolicy {
	attempts := defaultLLMRetryMaxAttempts
	if config != nil && config.GlobalConfig != nil && config.GlobalConfig.LLMRetryMaxAttempts > 0 {
		attempts = config.GlobalConfig.LLMRetryMaxAttempts
	} else if providerConfigProvider, ok := provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		providerConfig := providerConfigProvider.GetConfig()
		if providerConfig != nil && providerConfig.MaxRetries > 0 {
			attempts = providerConfig.MaxRetries + 1
		}
	}

	retries := attempts - 1
	if retries < 0 {
		retries = 0
	}

	backoffScale := models.DefaultRetryBackoffScale
	if providerConfigProvider, ok := provider.(interface {
		GetConfig() *conf.ModelProviderConfig
	}); ok {
		providerConfig := providerConfigProvider.GetConfig()
		if providerConfig != nil && providerConfig.RateLimitBackoffScale > 0 {
			backoffScale = providerConfig.GetRateLimitBackoffScaleDuration()
		}
	}

	maxBackoffSeconds := defaultLLMRetryMaxBackoffSeconds
	if config != nil && config.GlobalConfig != nil && config.GlobalConfig.LLMRetryMaxBackoffSeconds > 0 {
		maxBackoffSeconds = config.GlobalConfig.LLMRetryMaxBackoffSeconds
	}

	maxDelay := time.Duration(maxBackoffSeconds) * time.Second
	if maxDelay <= 0 {
		maxDelay = 60 * time.Second
	}

	return models.RetryPolicy{
		InitialDelay: backoffScale,
		MaxRetries:   retries,
		MaxDelay:     maxDelay,
	}
}

// NewGenerationChatModelFromSpec builds chat model chain with fallback and retry wrappers.
func NewGenerationChatModelFromSpec(
	modelSpec string,
	providers map[string]models.ModelProvider,
	options *models.ChatOptions,
	config *conf.CswConfig,
	primaryProvider models.ModelProvider,
	aliases map[string][]string,
	retryPolicyOverride *models.RetryPolicy,
	retryLogFn func(string, shared.MessageType),
) (models.ChatModel, error) {
	if primaryProvider == nil {
		return nil, fmt.Errorf("NewGenerationChatModelFromSpec() [chat_model_chain.go]: primary provider cannot be nil")
	}

	retryPolicy := DefaultLLMRetryPolicy(config, primaryProvider)
	if retryPolicyOverride != nil {
		retryPolicy = *retryPolicyOverride
	}
	chatModel, err := models.NewChatModelFromProviderChain(
		modelSpec,
		providers,
		options,
		&retryPolicy,
		retryLogFn,
		aliases,
	)
	if err != nil {
		return nil, fmt.Errorf("NewGenerationChatModelFromSpec() [chat_model_chain.go]: failed to build chat model chain: %w", err)
	}

	return chatModel, nil
}
