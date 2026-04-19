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
	"github.com/rlewczuk/csw/pkg/vfs"
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

// SetModel sets the model used for the session.
// model string should be formatted as `provider/model-name`
// or a comma-separated `provider/model-name` list for fallback.
func (s *SweSession) SetModel(modelStr string) error {
	if s.logger != nil {
		s.logger.Info("set_model", "model", modelStr)
	}

	refs, parseErr := models.ExpandProviderModelChain(modelStr, s.modelAliases)
	if parseErr != nil || len(refs) == 0 {
		if s.logger != nil {
			s.logger.Error("set_model_failed", "model", modelStr, "error", "invalid format")
		}
		return fmt.Errorf("SweSession.SetModel() [session_model_role.go]: invalid model format: %s, expected provider/model, comma-separated provider/model list, or model alias", modelStr)
	}

	for _, ref := range refs {
		if _, exists := s.modelProviders[ref.Provider]; !exists {
			if s.logger != nil {
				s.logger.Error("set_model_failed", "model", modelStr, "error", "provider not found")
			}
			return fmt.Errorf("SweSession.SetModel() [session_model_role.go]: provider not found: %s", ref.Provider)
		}
	}
	providerName := refs[0].Provider
	modelName := refs[0].Model
	provider := s.modelProviders[providerName]

	s.provider = provider
	s.providerName = providerName
	s.model = modelName
	s.modelSpec = models.ComposeProviderModelSpec(refs)
	s.applyModelTagToolSelection()
	if s.role != nil && s.role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, s.role.ToolsAccess)
	}
	s.persistSessionState()
	return nil
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

// SetRole changes the agent role for this session.
// It updates the VFS and Tools with access controls based on the new role,
// and adds or updates the system prompt at the beginning of the conversation.
func (s *SweSession) SetRole(roleName string) error {
	if s.logger != nil {
		s.logger.Info("set_role", "role", roleName)
	}

	role, ok := s.roles.Get(roleName)
	if !ok {
		if s.logger != nil {
			s.logger.Error("set_role_failed", "role", roleName, "error", "role not found")
		}
		return fmt.Errorf("SweSession.SetRole() [session_model_role.go]: role not found: %s", roleName)
	}

	// Store the new role
	s.role = &role
	s.rolesUsed = appendUniqueString(s.rolesUsed, role.Name)

	// Wrap VFS with access control based on role privileges
	if !s.allowAllPerms && role.VFSPrivileges != nil {
		s.VFS = vfs.NewAccessControlVFS(s.baseVFS, role.VFSPrivileges)
	} else {
		s.VFS = s.baseVFS
	}

	// Rebuild tools with the session's VFS and role and apply model-tag selection
	s.applyModelTagToolSelection()

	// Create a new tool registry with access-controlled tools if needed
	if role.ToolsAccess != nil {
		s.Tools = wrapToolsWithAccessControl(s.Tools, role.ToolsAccess)
	}

	// Generate and update system prompt using the prompt generator
	if s.promptGenerator != nil {
		state := s.GetState()

		// Get model tags from registry
		tags := s.GetModelTags()
		// If no specific tags are assigned, use empty list
		// The prompt system will include fragments with tag "all" by default
		if tags == nil {
			tags = []string{}
		}

		renderedPrompt, err := s.promptGenerator.GetPrompt(tags, &role, &state)
		if err != nil {
			return fmt.Errorf("SweSession.SetRole() [session_model_role.go]: failed to generate system prompt: %w", err)
		}

		// Check if there's already a system message
		if len(s.messages) > 0 && s.messages[0].Role == models.ChatRoleSystem {
			// Replace the existing system message
			systemMessage := models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
			s.messages[0] = systemMessage
			s.persistSessionState()
		} else {
			// Insert system message at the beginning
			systemMessage := models.NewTextMessage(models.ChatRoleSystem, renderedPrompt)
			s.messages = append([]*models.ChatMessage{systemMessage}, s.messages...)
			s.persistSessionState()
		}
	}

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
