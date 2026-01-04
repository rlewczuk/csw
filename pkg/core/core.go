package core

import (
	"context"
	"fmt"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
)

// SweSystem represents the core system for managing conversations, tools, and models.
type SweSystem struct {

	// Map of model providers by name
	ModelProviders map[string]models.ModelProvider

	// System prompt template
	SystemPrompt string

	// Tool registry
	Tools *tool.ToolRegistry

	// Virtual filesystem
	VFS vfs.VFS
}

func (s *SweSystem) NewSession(model string) (*SweSession, error) {
	// Parse provider/model format (e.g., "ollama/devstral-small-2:latest")
	var providerName, modelName string
	for i, c := range model {
		if c == '/' {
			providerName = model[:i]
			modelName = model[i+1:]
			break
		}
	}

	if providerName == "" || modelName == "" {
		return nil, fmt.Errorf("invalid model format, expected 'provider/model', got '%s'", model)
	}

	provider, ok := s.ModelProviders[providerName]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	session := &SweSession{
		system:   s,
		provider: provider,
		model:    modelName,
		messages: []*models.ChatMessage{},
	}

	// Add system prompt if provided
	if s.SystemPrompt != "" {
		session.messages = append(session.messages, models.NewTextMessage(models.ChatRoleSystem, s.SystemPrompt))
	}

	return session, nil
}

type SweSession struct {
	system   *SweSystem
	provider models.ModelProvider
	model    string
	messages []*models.ChatMessage
}

// Prompt adds user prompt to the conversation and starts processing if processing is not already in progress.
// If processing is already in progress, if will be added at the end of conversation after current LLM request is completed,
// its tool calls are executed etc. Returns immediately.
func (s *SweSession) UserPrompt(prompt string) error {
	s.messages = append(s.messages, models.NewTextMessage(models.ChatRoleUser, prompt))
	return nil
}

func (s *SweSession) Run(ctx context.Context) error {
	chatModel := s.provider.ChatModel(s.model, nil)
	tools := s.system.Tools.ListInfo()

	// Keep processing until the assistant doesn't make any tool calls
	for {
		// Use streaming chat API
		stream := chatModel.ChatStream(ctx, s.messages, nil, tools)

		// Accumulate the response from the stream
		responseMsg := &models.ChatMessage{
			Role:  models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{},
		}

		for fragment := range stream {
			// Merge the fragment parts into the accumulated response
			for _, part := range fragment.Parts {
				responseMsg.Parts = append(responseMsg.Parts, part)
			}
		}

		// Add the accumulated response to messages
		s.messages = append(s.messages, responseMsg)

		// Check if there are any tool calls in the response
		toolCalls := responseMsg.GetToolCalls()
		if len(toolCalls) == 0 {
			// No tool calls, we're done
			break
		}

		// Execute tool calls
		toolResponses := make([]*tool.ToolResponse, 0, len(toolCalls))
		for _, toolCall := range toolCalls {
			response := s.system.Tools.Execute(*toolCall)
			toolResponses = append(toolResponses, &response)
		}

		// Add tool responses to the conversation
		s.messages = append(s.messages, models.NewToolResponseMessage(toolResponses...))
	}

	return nil
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	return s.messages
}
