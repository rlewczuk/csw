package models

import (
	"fmt"
	"strings"

	"github.com/rlewczuk/csw/pkg/shared"
)

// ProviderModelRef represents a single provider/model selection.
type ProviderModelRef struct {
	Provider string
	Model    string
}

// ParseProviderModelChain parses comma-separated provider/model values.
func ParseProviderModelChain(modelSpec string) ([]ProviderModelRef, error) {
	trimmedSpec := strings.TrimSpace(modelSpec)
	if trimmedSpec == "" {
		return nil, fmt.Errorf("ParseProviderModelChain() [model_chain_factory.go]: model spec cannot be empty")
	}

	segments := strings.Split(trimmedSpec, ",")
	refs := make([]ProviderModelRef, 0, len(segments))
	for _, segment := range segments {
		trimmedSegment := strings.TrimSpace(segment)
		if trimmedSegment == "" {
			return nil, fmt.Errorf("ParseProviderModelChain() [model_chain_factory.go]: model spec contains empty model segment")
		}

		parts := strings.SplitN(trimmedSegment, "/", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("ParseProviderModelChain() [model_chain_factory.go]: invalid model format, expected 'provider/model', got '%s'", trimmedSegment)
		}

		refs = append(refs, ProviderModelRef{
			Provider: strings.TrimSpace(parts[0]),
			Model:    strings.TrimSpace(parts[1]),
		})
	}

	return refs, nil
}

// NewChatModelFromProviderChain creates chat model from provider/model chain.
func NewChatModelFromProviderChain(
	modelSpec string,
	providers map[string]ModelProvider,
	options *ChatOptions,
	retryPolicy *RetryPolicy,
	retryLogFn func(string, shared.MessageType),
) (ChatModel, error) {
	refs, err := ParseProviderModelChain(modelSpec)
	if err != nil {
		return nil, fmt.Errorf("NewChatModelFromProviderChain() [model_chain_factory.go]: %w", err)
	}

	modelsList := make([]ChatModel, 0, len(refs))
	for _, ref := range refs {
		provider, ok := providers[ref.Provider]
		if !ok {
			return nil, fmt.Errorf("NewChatModelFromProviderChain() [model_chain_factory.go]: provider not found: %s", ref.Provider)
		}

		modelsList = append(modelsList, provider.ChatModel(ref.Model, options))
	}

	var model ChatModel
	if len(modelsList) == 1 {
		model = modelsList[0]
	} else {
		model = NewFallbackChatModel(modelsList, 0)
	}

	if retryPolicy != nil {
		model = NewRetryChatModel(model, retryPolicy, retryLogFn)
	}

	return model, nil
}
