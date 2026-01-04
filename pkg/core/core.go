package core

import (
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
	// TODO implement me
	return nil, nil
}

type SweSession struct {
	// TODO add missing fields
}

// Prompt adds user prompt to the conversation and starts processing if processing is not already in progress.
// If processing is already in progress, if will be added at the end of conversation after current LLM request is completed,
// its tool calls are executed etc. Returns immediately.
func (s *SweSession) UserPrompt(prompt string) error {
	// TODO implement me
	return nil
}

func (s *SweSession) Run() error {
	// TODO implement me
	return nil
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	// TODO implement me
	return nil
}
