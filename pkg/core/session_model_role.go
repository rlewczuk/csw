package core

import (
	"context"
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
	if s.taskManager != nil {
		registry.Register("taskNew", tool.NewTaskNewTool(func(_ context.Context, params tool.TaskRecord, prompt string, parentTaskID string) (tool.TaskRecord, error) {
			created, err := s.taskManager.CreateTask(TaskCreateParams{
				ParentTaskID:  strings.TrimSpace(parentTaskID),
				Name:          strings.TrimSpace(params.Name),
				Description:   strings.TrimSpace(params.Description),
				FeatureBranch: strings.TrimSpace(params.FeatureBranch),
				ParentBranch:  strings.TrimSpace(params.ParentBranch),
				Role:          strings.TrimSpace(params.Role),
				Deps:          append([]string(nil), params.Deps...),
				Prompt:        strings.TrimSpace(prompt),
			})
			if err != nil {
				return tool.TaskRecord{}, err
			}

			return toToolTaskRecord(created), nil
		}, s))

		registry.Register("taskUpdate", tool.NewTaskUpdateTool(func(_ context.Context, identifier string, params tool.TaskRecord, prompt *string) (tool.TaskRecord, error) {
			update := TaskUpdateParams{Identifier: strings.TrimSpace(identifier)}
			if strings.TrimSpace(params.Name) != "" {
				value := strings.TrimSpace(params.Name)
				update.Name = &value
			}
			if params.Description != "" {
				value := strings.TrimSpace(params.Description)
				update.Description = &value
			}
			if params.Status != "" {
				value := strings.TrimSpace(params.Status)
				update.Status = &value
			}
			if strings.TrimSpace(params.FeatureBranch) != "" {
				value := strings.TrimSpace(params.FeatureBranch)
				update.FeatureBranch = &value
			}
			if strings.TrimSpace(params.ParentBranch) != "" {
				value := strings.TrimSpace(params.ParentBranch)
				update.ParentBranch = &value
			}
			if params.Role != "" {
				value := strings.TrimSpace(params.Role)
				update.Role = &value
			}
			if params.Deps != nil {
				value := append([]string(nil), params.Deps...)
				update.Deps = &value
			}
			if prompt != nil {
				trimmedPrompt := strings.TrimSpace(*prompt)
				update.Prompt = &trimmedPrompt
			}

			updated, err := s.taskManager.UpdateTask(update)
			if err != nil {
				return tool.TaskRecord{}, err
			}

			return toToolTaskRecord(updated), nil
		}, s))

		registry.Register("taskGet", tool.NewTaskGetTool(func(_ context.Context, identifier string, fallbackTaskID string, includeSummary bool) (tool.TaskRecord, *tool.TaskSessionSummary, string, error) {
			taskData, summaryMeta, summaryText, err := s.taskManager.GetTask(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, includeSummary)
			if err != nil {
				return tool.TaskRecord{}, nil, "", err
			}

			var summary *tool.TaskSessionSummary
			if summaryMeta != nil {
				summary = &tool.TaskSessionSummary{
					SessionID:   summaryMeta.SessionID,
					Status:      summaryMeta.Status,
					StartedAt:   summaryMeta.StartedAt,
					CompletedAt: summaryMeta.CompletedAt,
					TaskID:      summaryMeta.TaskID,
				}
			}

			return toToolTaskRecord(taskData), summary, summaryText, nil
		}, s))

		registry.Register("taskList", tool.NewTaskListTool(func(_ context.Context, identifier string, fallbackTaskID string, recursive bool) ([]tool.TaskRecord, error) {
			tasks, err := s.taskManager.ListTasks(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, recursive)
			if err != nil {
				return nil, err
			}

			result := make([]tool.TaskRecord, 0, len(tasks))
			for _, item := range tasks {
				result = append(result, toToolTaskRecord(item))
			}

			return result, nil
		}, s))

		registry.Register("taskMerge", tool.NewTaskMergeTool(func(_ context.Context, identifier string, fallbackTaskID string) (tool.TaskRecord, error) {
			if s.taskVCS == nil {
				return tool.TaskRecord{}, fmt.Errorf("SweSession.registerSessionTools() [session_model_role.go]: task VCS is nil")
			}

			merged, err := s.taskManager.MergeTask(TaskLookup{Identifier: strings.TrimSpace(identifier), FallbackTaskID: strings.TrimSpace(fallbackTaskID)}, s.taskVCS)
			if err != nil {
				return tool.TaskRecord{}, err
			}

			return toToolTaskRecord(merged), nil
		}, s))
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
