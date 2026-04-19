package core

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
)

func appendUniqueString(values []string, value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return values
	}

	for _, existing := range values {
		if existing == trimmed {
			return values
		}
	}

	return append(values, trimmed)
}

func (s *SweSession) ChatMessages() []*models.ChatMessage {
	return s.messages
}

// ID returns the unique identifier for this session.
func (s *SweSession) ID() string {
	return s.id
}

// TaskID returns task identifier associated with this session.
func (s *SweSession) TaskID() string {
	if s == nil {
		return ""
	}

	return strings.TrimSpace(s.taskID)
}

// SetTaskID sets task identifier associated with this session.
func (s *SweSession) SetTaskID(taskID string) {
	if s == nil {
		return
	}

	s.taskID = strings.TrimSpace(taskID)
	s.persistSessionState()
}

// SetTask sets task context associated with this session.
func (s *SweSession) SetTask(task *Task) {
	if s == nil {
		return
	}

	s.task = cloneTask(task)
	s.persistSessionState()
}

// ParentID returns the parent session identifier for delegated child sessions.
func (s *SweSession) ParentID() string {
	if s == nil {
		return ""
	}

	return s.parentID
}

// Slug returns session slug used by UI views and logs.
func (s *SweSession) Slug() string {
	if s == nil {
		return ""
	}

	return s.slug
}

// ReserveSubAgentSlug marks slug as used in parent session.
func (s *SweSession) ReserveSubAgentSlug(slug string) error {
	if s == nil {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session_state.go]: session is nil")
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session_state.go]: slug cannot be empty")
	}

	s.subAgentSlugsMu.Lock()
	defer s.subAgentSlugsMu.Unlock()
	if s.subAgentSlugs == nil {
		s.subAgentSlugs = make(map[string]struct{})
	}
	if _, exists := s.subAgentSlugs[trimmedSlug]; exists {
		return fmt.Errorf("SweSession.ReserveSubAgentSlug() [session_state.go]: slug already used in session: %s", trimmedSlug)
	}
	s.subAgentSlugs[trimmedSlug] = struct{}{}
	s.persistSessionState()

	return nil
}

// ReserveUniqueSubAgentSlug reserves slug in parent session and adds numeric suffix when needed.
func (s *SweSession) ReserveUniqueSubAgentSlug(slug string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("SweSession.ReserveUniqueSubAgentSlug() [session_state.go]: session is nil")
	}

	baseSlug := strings.TrimSpace(slug)
	if baseSlug == "" {
		return "", fmt.Errorf("SweSession.ReserveUniqueSubAgentSlug() [session_state.go]: slug cannot be empty")
	}

	s.subAgentSlugsMu.Lock()
	defer s.subAgentSlugsMu.Unlock()
	if s.subAgentSlugs == nil {
		s.subAgentSlugs = make(map[string]struct{})
	}

	resolvedSlug := baseSlug
	if _, exists := s.subAgentSlugs[resolvedSlug]; exists {
		for suffix := 2; ; suffix++ {
			candidate := fmt.Sprintf("%s-%d", baseSlug, suffix)
			if _, exists := s.subAgentSlugs[candidate]; exists {
				continue
			}
			resolvedSlug = candidate
			break
		}
	}

	s.subAgentSlugs[resolvedSlug] = struct{}{}
	s.persistSessionState()

	return resolvedSlug, nil
}

// SetLogger sets a custom logger for this session.
// This is useful for testing or when you want to use a different logger implementation.
func (s *SweSession) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// SetOutputHandler sets output handler used by session callbacks.
func (s *SweSession) SetOutputHandler(handler SessionThreadOutput) {
	s.outputHandler = handler
}

// OutputHandler returns currently configured output handler.
func (s *SweSession) OutputHandler() SessionThreadOutput {
	return s.outputHandler
}

// Model returns the model name (without provider prefix) used for this session.
func (s *SweSession) Model() string {
	return s.model
}

// ModelWithProvider returns the provider-qualified model name used for this session.
func (s *SweSession) ModelWithProvider() string {
	if strings.TrimSpace(s.modelSpec) != "" {
		return s.modelSpec
	}

	if strings.TrimSpace(s.providerName) == "" {
		return s.model
	}

	if strings.TrimSpace(s.model) == "" {
		return s.providerName
	}

	return s.providerName + "/" + s.model
}

// GetState returns the current agent state for this session.
func (s *SweSession) GetState() AgentState {
	shadowDir := ""
	if s != nil {
		shadowDir = strings.TrimSpace(s.shadowDir)
		if shadowDir == "" {
			shadowDir = strings.TrimSpace(s.workDir)
		}
	}
	if shadowDir == "" {
		shadowDir = strings.TrimSpace(s.workDir)
	}
	if shadowDir != "" && !filepath.IsAbs(shadowDir) {
		if absShadowDir, err := filepath.Abs(shadowDir); err == nil {
			shadowDir = absShadowDir
		}
	}

	return AgentState{
		Info: AgentStateCommonInfo{
			WorkDir:             s.workDir,
			ShadowDir:           shadowDir,
			CurrentTime:         time.Now().Format(time.RFC3339),
			AgentName:           "CSW Coding Agent",
			TokenUsage:          s.tokenUsage,
			ContextLengthTokens: s.contextLength,
		},
		Role: s.role.Clone(),
		Task: cloneTask(s.task),
	}
}

// TokenUsage returns aggregated token usage for this session.
func (s *SweSession) TokenUsage() models.TokenUsage {
	return s.tokenUsage
}

// ContextLengthTokens returns latest known context length in tokens.
func (s *SweSession) ContextLengthTokens() int {
	return s.contextLength
}

// CompactionCount returns number of performed context compactions.
func (s *SweSession) CompactionCount() int {
	if s == nil {
		return 0
	}

	return s.compactionCount
}

// SetWorkDir sets the working directory for this session.
func (s *SweSession) SetWorkDir(dir string) {
	s.workDir = dir
	s.persistSessionState()
}

// PersistSessionState persists current session state to disk.
func (s *SweSession) PersistSessionState() {
	s.persistSessionState()
}

// Role returns the current agent role for this session.
func (s *SweSession) Role() *conf.AgentRoleConfig {
	return s.role
}

// ProviderName returns the name of the provider used for this session.
func (s *SweSession) ProviderName() string {
	return s.providerName
}

// ThinkingLevel returns configured thinking level for this session.
func (s *SweSession) ThinkingLevel() string {
	if s == nil {
		return ""
	}

	return strings.TrimSpace(s.thinking)
}

// SetThinkingLevel updates configured thinking mode for this session.
func (s *SweSession) SetThinkingLevel(thinking string) {
	if s == nil {
		return
	}

	s.thinking = strings.TrimSpace(thinking)
	s.persistSessionState()
}

// UsedRoles returns roles used during this session in first-seen order.
func (s *SweSession) UsedRoles() []string {
	if s == nil || len(s.rolesUsed) == 0 {
		return nil
	}

	result := make([]string, len(s.rolesUsed))
	copy(result, s.rolesUsed)
	return result
}

// UsedTools returns tools used during this session in first-seen order.
func (s *SweSession) UsedTools() []string {
	if s == nil || len(s.toolsUsed) == 0 {
		return nil
	}

	result := make([]string, len(s.toolsUsed))
	copy(result, s.toolsUsed)
	return result
}

// GetModelTags returns all tags assigned to the current model.
// Tags are determined by matching the model name against regexp patterns
// from both global config and provider-specific config.
func (s *SweSession) GetModelTags() []string {
	if s.modelTags == nil {
		return nil
	}
	return s.modelTags.GetTagsForModel(s.providerName, s.model)
}

// GetTodoList returns a copy of the current todo list.
func (s *SweSession) GetTodoList() []tool.TodoItem {
	s.todoMu.Lock()
	defer s.todoMu.Unlock()

	// Return a copy to prevent external modification
	list := make([]tool.TodoItem, len(s.todoList))
	copy(list, s.todoList)
	return list
}

// SetTodoList replaces the entire todo list with a new list.
func (s *SweSession) SetTodoList(todos []tool.TodoItem) {
	s.todoMu.Lock()
	s.todoList = make([]tool.TodoItem, len(todos))
	copy(s.todoList, todos)
	s.todoMu.Unlock()

	s.persistSessionState()
}

// CountPendingTodos returns the number of pending or in_progress todos.
func (s *SweSession) CountPendingTodos() int {
	s.todoMu.Lock()
	defer s.todoMu.Unlock()

	count := 0
	for _, item := range s.todoList {
		if item.Status == "pending" || item.Status == "in_progress" {
			count++
		}
	}
	return count
}

// ExecuteSubAgentTask executes delegated child-session task using configured runner.
func (s *SweSession) ExecuteSubAgentTask(request tool.SubAgentTaskRequest) (tool.SubAgentTaskResult, error) {
	if s == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSession.ExecuteSubAgentTask() [session_state.go]: session is nil")
	}
	if s.subAgentRunner == nil {
		return tool.SubAgentTaskResult{}, fmt.Errorf("SweSession.ExecuteSubAgentTask() [session_state.go]: subagent runner is not configured")
	}

	return s.subAgentRunner.ExecuteSubAgentTask(s, request)
}

func (s *SweSession) appendConversationMessage(message *models.ChatMessage, direction string, source string) {
	if message == nil {
		return
	}

	s.messages = append(s.messages, message)
	s.applyMessageTokenStats(message)
	s.persistSessionState()
}

func (s *SweSession) applyMessageTokenStats(message *models.ChatMessage) {
	if message == nil {
		return
	}

	if message.TokenUsage != nil {
		s.tokenUsage.InputTokens += message.TokenUsage.InputTokens
		s.tokenUsage.InputCachedTokens += message.TokenUsage.InputCachedTokens
		s.tokenUsage.InputNonCachedTokens += message.TokenUsage.InputNonCachedTokens
		s.tokenUsage.OutputTokens += message.TokenUsage.OutputTokens
		s.tokenUsage.TotalTokens += message.TokenUsage.TotalTokens
	}

	if message.ContextLengthTokens > 0 {
		s.contextLength = message.ContextLengthTokens
	} else if message.TokenUsage != nil && message.TokenUsage.TotalTokens > 0 {
		s.contextLength = message.TokenUsage.TotalTokens
	}
}

// HasPendingWork returns true when the session has pending work that can be resumed
// without adding a new user message.
func (s *SweSession) HasPendingWork() bool {
	if s == nil {
		return false
	}

	if len(s.pendingToolResponses) > 0 {
		return true
	}

	if len(s.messages) == 0 {
		return false
	}

	last := s.messages[len(s.messages)-1]
	if last == nil {
		return false
	}

	if last.Role == models.ChatRoleUser {
		return true
	}

	if last.Role == models.ChatRoleAssistant && len(last.GetToolCalls()) > 0 {
		return true
	}

	return false
}
