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

// ExpandProviderModelChain expands comma-separated model spec into provider/model refs.
// Each segment may be a direct provider/model ref or an alias key from aliases map.
func ExpandProviderModelChain(modelSpec string, aliases map[string][]string) ([]ProviderModelRef, error) {
	trimmedSpec := strings.TrimSpace(modelSpec)
	if trimmedSpec == "" {
		return nil, fmt.Errorf("ExpandProviderModelChain() [model_chain_factory.go]: model spec cannot be empty")
	}

	segments := strings.Split(trimmedSpec, ",")
	refs := make([]ProviderModelRef, 0, len(segments))
	for _, segment := range segments {
		resolvedRefs, err := expandProviderModelSegment(strings.TrimSpace(segment), aliases, nil)
		if err != nil {
			return nil, err
		}
		refs = append(refs, resolvedRefs...)
	}

	if len(refs) == 0 {
		return nil, fmt.Errorf("ExpandProviderModelChain() [model_chain_factory.go]: model spec cannot be empty")
	}

	return refs, nil
}

func expandProviderModelSegment(segment string, aliases map[string][]string, stack map[string]struct{}) ([]ProviderModelRef, error) {
	if segment == "" {
		return nil, fmt.Errorf("expandProviderModelSegment() [model_chain_factory.go]: model spec contains empty model segment")
	}

	if ref, ok := parseSingleProviderModelRef(segment); ok {
		return []ProviderModelRef{ref}, nil
	}

	if len(aliases) == 0 {
		return nil, fmt.Errorf("expandProviderModelSegment() [model_chain_factory.go]: invalid model format, expected 'provider/model', got '%s'", segment)
	}

	aliasValues, ok := aliases[segment]
	if !ok {
		return nil, fmt.Errorf("expandProviderModelSegment() [model_chain_factory.go]: unknown model alias: %s", segment)
	}
	if len(aliasValues) == 0 {
		return nil, fmt.Errorf("expandProviderModelSegment() [model_chain_factory.go]: alias %q has no targets", segment)
	}

	if stack == nil {
		stack = make(map[string]struct{})
	}
	if _, exists := stack[segment]; exists {
		return nil, fmt.Errorf("expandProviderModelSegment() [model_chain_factory.go]: cyclic alias reference detected for %q", segment)
	}

	nextStack := make(map[string]struct{}, len(stack)+1)
	for key := range stack {
		nextStack[key] = struct{}{}
	}
	nextStack[segment] = struct{}{}

	refs := make([]ProviderModelRef, 0, len(aliasValues))
	for _, aliasValue := range aliasValues {
		parts := strings.Split(aliasValue, ",")
		for _, part := range parts {
			resolvedRefs, err := expandProviderModelSegment(strings.TrimSpace(part), aliases, nextStack)
			if err != nil {
				return nil, err
			}
			refs = append(refs, resolvedRefs...)
		}
	}

	return refs, nil
}

func parseSingleProviderModelRef(segment string) (ProviderModelRef, bool) {
	parts := strings.SplitN(segment, "/", 2)
	if len(parts) != 2 {
		return ProviderModelRef{}, false
	}
	providerName := strings.TrimSpace(parts[0])
	modelName := strings.TrimSpace(parts[1])
	if providerName == "" || modelName == "" {
		return ProviderModelRef{}, false
	}

	return ProviderModelRef{Provider: providerName, Model: modelName}, true
}

// ComposeProviderModelSpec joins provider/model refs into a comma-separated model spec.
func ComposeProviderModelSpec(refs []ProviderModelRef) string {
	if len(refs) == 0 {
		return ""
	}

	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, ref.Provider+"/"+ref.Model)
	}

	return strings.Join(parts, ",")
}

// NewChatModelFromProviderChain creates chat model from provider/model chain.
func NewChatModelFromProviderChain(
	modelSpec string,
	providers map[string]ModelProvider,
	options *ChatOptions,
	retryPolicy *RetryPolicy,
	retryLogFn func(string, shared.MessageType),
	aliases map[string][]string,
) (ChatModel, error) {
	refs, err := ExpandProviderModelChain(modelSpec, aliases)
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
