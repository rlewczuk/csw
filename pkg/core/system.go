package core

import (
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

	// Roles
	Roles *AgentRoleRegistry
}

func (s *SweSystem) NewSession(model string, outputHandler SessionThreadOutput) (*SweSession, error) {
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
