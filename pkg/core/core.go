package core

import (
	"context"
	"fmt"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/shared"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/ui"
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

	// Roles
	Roles *AgentRoleRegistry
}

func (s *SweSystem) NewSession(model string, outputHandler ui.SessionOutputHandler) (*SweSession, error) {
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
		system:        s,
		provider:      provider,
		model:         modelName,
		messages:      []*models.ChatMessage{},
		role:          nil,
		VFS:           s.VFS,
		Tools:         s.Tools,
		outputHandler: outputHandler,
		workDir:       ".",
	}

	// Add system prompt if provided
	if s.SystemPrompt != "" {
		session.messages = append(session.messages, models.NewTextMessage(models.ChatRoleSystem, s.SystemPrompt))
	}

	return session, nil
}

type SweSession struct {
	system        *SweSystem
	provider      models.ModelProvider
	model         string
	messages      []*models.ChatMessage
	role          *AgentRole
	VFS           vfs.VFS
	Tools         *tool.ToolRegistry
	outputHandler ui.SessionOutputHandler
	workDir       string
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

		// Track tool calls we've seen to handle start events
		seenToolCalls := make(map[string]bool)

		for fragment := range stream {
			// Merge the fragment parts into the accumulated response
			for _, part := range fragment.Parts {
				responseMsg.Parts = append(responseMsg.Parts, part)

				// Notify UI handler about the fragment
				if s.outputHandler != nil {
					if part.Text != "" {
						s.outputHandler.AddMarkdownChunk(part.Text)
					}
					if part.ToolCall != nil {
						// Send start event if this is the first time we see this tool call
						if !seenToolCalls[part.ToolCall.ID] {
							s.outputHandler.AddToolCallStart(part.ToolCall)
							seenToolCalls[part.ToolCall.ID] = true
						}
						// Always send details event as chunks arrive
						s.outputHandler.AddToolCallDetails(part.ToolCall)
					}
				}
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

			// Notify UI handler about tool result
			if s.outputHandler != nil {
				s.outputHandler.AddToolCallResult(&response)
			}
		}

		// Add tool responses to the conversation
		s.messages = append(s.messages, models.NewToolResponseMessage(toolResponses...))
	}

	return nil
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	return s.messages
}

// GetState returns the current agent state for this session.
func (s *SweSession) GetState() AgentState {
	return AgentState{
		Info: AgentStateCommonInfo{
			WorkDir: s.workDir,
		},
	}
}

// SetWorkDir sets the working directory for this session.
func (s *SweSession) SetWorkDir(dir string) {
	s.workDir = dir
}

// Role returns the current agent role for this session.
func (s *SweSession) Role() *AgentRole {
	return s.role
}

// SetRole changes the agent role for this session.
// It updates the VFS and Tools with access controls based on the new role,
// and adds or updates the system prompt at the beginning of the conversation.
func (s *SweSession) SetRole(roleName string) error {
	role, ok := s.system.Roles.Get(roleName)
	if !ok {
		return fmt.Errorf("role not found: %s", roleName)
	}

	// Store the new role
	s.role = &role

	// Wrap VFS with access control based on role privileges
	if role.VFSPrivileges != nil {
		s.VFS = vfs.NewAccessControlVFS(s.system.VFS, role.VFSPrivileges)
	} else {
		s.VFS = s.system.VFS
	}

	// Create a new tool registry with access-controlled tools
	if role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.system.Tools, role.ToolsAccess)
	} else {
		s.Tools = s.system.Tools
	}

	// Update system prompt by rendering the template with current state
	if role.SystemPrompt != "" {
		state := s.GetState()
		renderedPrompt, err := role.RenderSystemPrompt(state)
		if err != nil {
			return fmt.Errorf("failed to render system prompt: %w", err)
		}

		// Check if there's already a system message
		if len(s.messages) > 0 && s.messages[0].Role == models.ChatRoleSystem {
			// Replace the existing system message
			s.messages[0] = models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
		} else {
			// Insert system message at the beginning
			s.messages = append([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)}, s.messages...)
		}
	}

	return nil
}

// wrapToolsWithAccessControl creates a new tool registry with all tools wrapped in access control.
func wrapToolsWithAccessControl(registry *tool.ToolRegistry, privileges map[string]shared.AccessFlag) *tool.ToolRegistry {
	newRegistry := tool.NewToolRegistry()

	// Get all tool names from the original registry
	for _, name := range registry.List() {
		t, err := registry.Get(name)
		if err != nil {
			// This shouldn't happen since we just got the name from List()
			continue
		}

		// Wrap the tool with access control
		wrappedTool := tool.NewAccessControlTool(t, privileges)
		newRegistry.Register(name, wrappedTool)
	}

	return newRegistry
}
