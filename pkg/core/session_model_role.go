package core

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/rlewczuk/csw/pkg/apis"
	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/lsp"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

func composeProviderModel(providerName string, modelName string) string {
	trimmedProvider := strings.TrimSpace(providerName)
	trimmedModel := strings.TrimSpace(modelName)
	if trimmedProvider == "" {
		return trimmedModel
	}
	if trimmedModel == "" {
		return trimmedProvider
	}

	return trimmedProvider + "/" + trimmedModel
}

// applyModelTagToolSelection rebuilds tools and applies model-tag based tool selection rules.
func (s *SweSession) applyModelTagToolSelection() {
	baseTools := buildSessionToolRegistry(s.systemTools, s.VFS, s.LSP, s)
	if s.modelTags == nil {
		s.Tools = filterToolsForRole(baseTools.FilterByModelTags(nil, s.toolSelection), s.role)
		return
	}
	tags := s.modelTags.GetTagsForModel(s.providerName, s.model)
	s.Tools = filterToolsForRole(baseTools.FilterByModelTags(tags, s.toolSelection), s.role)
}

func (s *SweSession) updateSystemPromptForRole(role conf.AgentRoleConfig) error {
	if s.promptGenerator == nil {
		return nil
	}

	state := s.GetState()
	tags := s.GetModelTags()
	if tags == nil {
		tags = []string{}
	}

	renderedPrompt, err := s.promptGenerator.GetPrompt(tags, &role, &state)
	if err != nil {
		return fmt.Errorf("SweSession.updateSystemPromptForRole() [session_model_role.go]: failed to generate system prompt: %w", err)
	}

	if len(s.messages) > 0 && s.messages[0].Role == models.ChatRoleSystem {
		s.messages[0] = models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
		s.persistSessionState()
		return nil
	}

	s.messages = append([]*models.ChatMessage{models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)}, s.messages...)
	s.persistSessionState()

	return nil
}

// wrapToolsWithAccessControl creates a new tool registry with all tools wrapped in access control.
func wrapToolsWithAccessControl(registry *tool.ToolRegistry, privileges map[string]conf.AccessFlag) *tool.ToolRegistry {
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

func filterToolsForRole(registry *tool.ToolRegistry, role *conf.AgentRoleConfig) *tool.ToolRegistry {
	if registry == nil || role == nil {
		return registry
	}

	filtered := tool.NewToolRegistry()
	for _, name := range registry.List() {
		t, err := registry.Get(name)
		if err != nil {
			continue
		}
		if restricted, ok := t.(tool.RoleRestrictedTool); ok {
			if !restricted.IsRoleAllowed(role.Name) {
				continue
			}
		}
		filtered.Register(name, t)
	}

	return filtered
}

// registerSessionTools registers session-specific tools that need access to the session.
func (s *SweSession) registerSessionTools(registry *tool.ToolRegistry) {
	// Register todo tools
	registry.Register("todoRead", tool.NewTodoReadTool(s))
	registry.Register("todoWrite", tool.NewTodoWriteTool(s))
	if s.taskBackend != nil {
		registry.Register("taskNew", tool.NewTaskNewTool(s.taskBackend, s))
		registry.Register("taskUpdate", tool.NewTaskUpdateTool(s.taskBackend, s))
		registry.Register("taskGet", tool.NewTaskGetTool(s.taskBackend, s))
		registry.Register("taskList", tool.NewTaskListTool(s.taskBackend, s))
		registry.Register("taskMerge", tool.NewTaskMergeTool(s.taskBackend, s))
	}
}

func buildSessionToolRegistry(systemTools *tool.ToolRegistry, vfsImpl apis.VFS, lspClient lsp.LSP, session *SweSession) *tool.ToolRegistry {
	registry := tool.NewToolRegistry()
	if systemTools != nil {
		for _, name := range systemTools.List() {
			t, _ := systemTools.Get(name)
			registry.Register(name, t)
		}
	}

	var logger *slog.Logger
	if session != nil {
		logger = session.logger
	}
	tool.RegisterVFSTools(registry, vfsImpl, lspClient, logger)

	if session != nil {
		session.registerSessionTools(registry)
	}

	registry.ApplyLogger(logger)

	return registry
}
