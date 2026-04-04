package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rlewczuk/csw/pkg/logging"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/rlewczuk/csw/pkg/vfs"
)

type persistedToolResponse struct {
	Call   *tool.ToolCall `json:"call,omitempty"`
	Error  string         `json:"error,omitempty"`
	Result tool.ToolValue `json:"result"`
	Done   bool           `json:"done"`
}

type persistedChatMessagePart struct {
	Text             string                 `json:"text,omitempty"`
	ReasoningContent string                 `json:"reasoning_content,omitempty"`
	ToolCall         *tool.ToolCall         `json:"tool_call,omitempty"`
	ToolResponse     *persistedToolResponse `json:"tool_response,omitempty"`
}

type persistedChatMessage struct {
	Role  string                     `json:"role"`
	Parts []persistedChatMessagePart `json:"parts"`
}

type persistedSessionState struct {
	SessionID                  string                  `json:"session_id"`
	ParentSessionID            string                  `json:"parent_session_id,omitempty"`
	TaskID                     string                  `json:"task_id,omitempty"`
	Slug                       string                  `json:"slug,omitempty"`
	ModelSpec                  string                  `json:"model_spec,omitempty"`
	ProviderName               string                  `json:"provider_name"`
	Model                      string                  `json:"model"`
	Thinking                   string                  `json:"thinking,omitempty"`
	RolesUsed                  []string                `json:"roles_used,omitempty"`
	ToolsUsed                  []string                `json:"tools_used,omitempty"`
	RoleName                   string                  `json:"role_name,omitempty"`
	WorkDir                    string                  `json:"workdir"`
	TodoList                   []tool.TodoItem         `json:"todo_list"`
	Messages                   []persistedChatMessage  `json:"messages"`
	PendingPermissionToolCalls []tool.ToolCall         `json:"pending_permission_tool_calls"`
	PendingToolResponses       []persistedToolResponse `json:"pending_tool_responses"`
	LoadedAgentFiles           []string                `json:"loaded_agent_files"`
	TokenUsage                 models.TokenUsage       `json:"token_usage"`
	ContextLengthTokens        int                     `json:"context_length_tokens"`
	ContextCompactionCount     int                     `json:"context_compaction_count"`
	UsedSubAgentSlugs          []string                `json:"used_subagent_slugs,omitempty"`
	UpdatedAt                  string                  `json:"updated_at"`
}

// PersistedSessionState is a persisted session state model used for loading sessions.
type PersistedSessionState = persistedSessionState

func (s *SweSession) persistSessionState() {
	if err := s.persistSessionStateFile(); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed_to_persist_session_state", "error", err)
		}
	}
}

func (s *SweSession) persistSessionStateFile() error {
	sessionLogDir := s.getSessionLogDirectory()
	if sessionLogDir == "" {
		return nil
	}

	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session_persistence.go]: failed to create session log directory: %w", err)
	}

	state := s.buildPersistedSessionState()
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session_persistence.go]: failed to marshal session state: %w", err)
	}

	tempPath := filepath.Join(sessionLogDir, "session.json.tmp")
	finalPath := filepath.Join(sessionLogDir, "session.json")
	if err := os.WriteFile(tempPath, stateJSON, 0644); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session_persistence.go]: failed to write temporary session state file: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("SweSession.persistSessionStateFile() [session_persistence.go]: failed to replace session state file: %w", err)
	}

	return nil
}

func (s *SweSession) buildPersistedSessionState() persistedSessionState {
	state := persistedSessionState{
		SessionID:                  s.id,
		ParentSessionID:            s.parentID,
		TaskID:                     strings.TrimSpace(s.taskID),
		Slug:                       s.slug,
		ModelSpec:                  s.ModelWithProvider(),
		ProviderName:               s.providerName,
		Model:                      s.model,
		Thinking:                   strings.TrimSpace(s.thinking),
		RolesUsed:                  append([]string(nil), s.rolesUsed...),
		ToolsUsed:                  append([]string(nil), s.toolsUsed...),
		WorkDir:                    s.workDir,
		TodoList:                   s.GetTodoList(),
		Messages:                   make([]persistedChatMessage, 0, len(s.messages)),
		PendingPermissionToolCalls: make([]tool.ToolCall, 0, len(s.pendingPermissionToolCalls)),
		PendingToolResponses:       make([]persistedToolResponse, 0, len(s.pendingToolResponses)),
		LoadedAgentFiles:           make([]string, 0, len(s.loadedAgentFiles)),
		UsedSubAgentSlugs:          make([]string, 0, len(s.subAgentSlugs)),
		TokenUsage:                 s.tokenUsage,
		ContextLengthTokens:        s.contextLength,
		ContextCompactionCount:     s.compactionCount,
		UpdatedAt:                  time.Now().Format(time.RFC3339Nano),
	}

	if s.role != nil {
		state.RoleName = s.role.Name
	}

	for _, message := range s.messages {
		state.Messages = append(state.Messages, serializeChatMessage(message))
	}

	for _, toolCall := range s.pendingPermissionToolCalls {
		if toolCall == nil {
			continue
		}
		state.PendingPermissionToolCalls = append(state.PendingPermissionToolCalls, *toolCall)
	}

	for _, toolResponse := range s.pendingToolResponses {
		if toolResponse == nil {
			continue
		}
		state.PendingToolResponses = append(state.PendingToolResponses, serializeToolResponse(toolResponse))
	}

	for path := range s.loadedAgentFiles {
		state.LoadedAgentFiles = append(state.LoadedAgentFiles, path)
	}
	sort.Strings(state.LoadedAgentFiles)

	for slug := range s.subAgentSlugs {
		state.UsedSubAgentSlugs = append(state.UsedSubAgentSlugs, slug)
	}
	sort.Strings(state.UsedSubAgentSlugs)

	return state
}

func (s *SweSession) getSessionLogDirectory() string {
	if s == nil || s.id == "" {
		return ""
	}

	if s.logBaseDir != "" {
		return filepath.Join(s.logBaseDir, "sessions", s.id)
	}

	dir := logging.GetSessionLogDirectory(s.id)
	if dir != "" {
		return dir
	}

	return ""
}

func serializeChatMessage(message *models.ChatMessage) persistedChatMessage {
	serialized := persistedChatMessage{
		Role:  string(message.Role),
		Parts: make([]persistedChatMessagePart, 0, len(message.Parts)),
	}

	for _, part := range message.Parts {
		serializedPart := persistedChatMessagePart{
			Text:             part.Text,
			ReasoningContent: part.ReasoningContent,
			ToolCall:         part.ToolCall,
		}
		if part.ToolResponse != nil {
			serializedToolResponse := serializeToolResponse(part.ToolResponse)
			serializedPart.ToolResponse = &serializedToolResponse
		}
		serialized.Parts = append(serialized.Parts, serializedPart)
	}

	return serialized
}

func writeMessagesJSONL(path string, messages []*models.ChatMessage) error {
	var builder strings.Builder
	for _, message := range messages {
		if message == nil {
			continue
		}
		serializedMessage := serializeChatMessage(message)
		line, err := json.Marshal(serializedMessage)
		if err != nil {
			return fmt.Errorf("writeMessagesJSONL() [session_persistence.go]: failed to marshal chat message: %w", err)
		}
		builder.Write(line)
		builder.WriteByte('\n')
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("writeMessagesJSONL() [session_persistence.go]: failed to write jsonl file: %w", err)
	}

	return nil
}

func serializeToolResponse(response *tool.ToolResponse) persistedToolResponse {
	serialized := persistedToolResponse{
		Result: response.Result,
		Done:   response.Done,
	}
	if response.Call != nil {
		serializedCall := *response.Call
		serialized.Call = &serializedCall
	}
	if response.Error != nil {
		serialized.Error = response.Error.Error()
	}

	return serialized
}

// RestoreSessionFromPersistedState restores session state from persisted data.
func RestoreSessionFromPersistedState(params *SweSessionParams, state persistedSessionState, outputHandler SessionThreadOutput) (*SweSession, error) {
	if strings.TrimSpace(state.SessionID) == "" {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: missing session_id in persisted state")
	}

	if params == nil {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: params cannot be nil")
	}

	provider, ok := params.ModelProviders[state.ProviderName]
	if !ok {
		return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: provider not found: %s", state.ProviderName)
	}

	session := NewSweSession(
		&SweSessionParams{
			ID:       state.SessionID,
			ParentID: state.ParentSessionID,
			TaskID:   state.TaskID,
			Slug:     state.Slug,
			ModelSpec: func() string {
				if strings.TrimSpace(state.ModelSpec) != "" {
					return strings.TrimSpace(state.ModelSpec)
				}

				return composeProviderModel(state.ProviderName, state.Model)
			}(),
			Provider:        provider,
			ProviderName:    state.ProviderName,
			Model:           state.Model,
			RolesUsed:       state.RolesUsed,
			ToolsUsed:       state.ToolsUsed,
			Messages:        []*models.ChatMessage{},
			Role:            nil,
			VFS:             params.VFS,
			BaseVFS:         params.VFS,
			LSP:             params.LSP,
			SystemTools:     params.SystemTools,
			ModelProviders:  params.ModelProviders,
			ModelTags:       params.ModelTags,
			ToolSelection:   params.ToolSelection,
			PromptGenerator: params.PromptGenerator,
			Roles:           params.Roles,
			ConfigStore:     params.ConfigStore,
			OutputHandler:   outputHandler,
			WorkDir:         state.WorkDir,
			ShadowDir:       params.ShadowDir,
			LogBaseDir:      params.LogBaseDir,
			Thinking: func() string {
				if strings.TrimSpace(state.Thinking) != "" {
					return strings.TrimSpace(state.Thinking)
				}

				return params.Thinking
			}(),
			Logger:          params.Logger,
			LLMLogger:       params.LLMLogger,
			TodoList:        state.TodoList,
			TokenUsage:      state.TokenUsage,
			ContextLength:   state.ContextLengthTokens,
			CompactionCount: state.ContextCompactionCount,
			UsedSubAgentSlugs: func() map[string]struct{} {
				result := make(map[string]struct{}, len(state.UsedSubAgentSlugs))
				for _, slug := range state.UsedSubAgentSlugs {
					trimmedSlug := strings.TrimSpace(slug)
					if trimmedSlug == "" {
						continue
					}
					result[trimmedSlug] = struct{}{}
				}
				return result
			}(),
			TaskBackend: params.TaskBackend,
		},
	)

	if strings.TrimSpace(state.RoleName) != "" {
		role, roleOK := params.Roles.Get(state.RoleName)
		if !roleOK {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: role not found: %s", state.RoleName)
		}

		session.role = &role

		if role.VFSPrivileges != nil {
			session.VFS = vfs.NewAccessControlVFS(params.VFS, role.VFSPrivileges)
		} else {
			session.VFS = params.VFS
		}

		session.applyModelTagToolSelection()
		if role.ToolsAccess != nil {
			session.Tools = wrapToolsWithAccessControl(session.Tools, role.ToolsAccess)
		}
		if len(session.rolesUsed) == 0 {
			session.rolesUsed = append(session.rolesUsed, role.Name)
		}
	}

	for _, persistedMsg := range state.Messages {
		message, err := deserializeChatMessage(persistedMsg)
		if err != nil {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: failed to deserialize message: %w", err)
		}
		session.messages = append(session.messages, message)
	}

	session.pendingPermissionToolCalls = make([]*tool.ToolCall, 0, len(state.PendingPermissionToolCalls))
	for _, pendingCall := range state.PendingPermissionToolCalls {
		copiedCall := pendingCall
		session.pendingPermissionToolCalls = append(session.pendingPermissionToolCalls, &copiedCall)
	}

	session.pendingToolResponses = make([]*tool.ToolResponse, 0, len(state.PendingToolResponses))
	for _, pendingResponse := range state.PendingToolResponses {
		response, err := deserializeToolResponse(pendingResponse)
		if err != nil {
			return nil, fmt.Errorf("RestoreSessionFromPersistedState() [session_persistence.go]: failed to deserialize pending tool response: %w", err)
		}
		session.pendingToolResponses = append(session.pendingToolResponses, response)
	}

	session.loadedAgentFiles = make(map[string]struct{}, len(state.LoadedAgentFiles))
	for _, path := range state.LoadedAgentFiles {
		session.loadedAgentFiles[path] = struct{}{}
	}

	return session, nil
}

func deserializeChatMessage(persisted persistedChatMessage) (*models.ChatMessage, error) {
	message := &models.ChatMessage{
		Role:  models.ChatRole(persisted.Role),
		Parts: make([]models.ChatMessagePart, 0, len(persisted.Parts)),
	}

	for _, persistedPart := range persisted.Parts {
		part := models.ChatMessagePart{
			Text:             persistedPart.Text,
			ReasoningContent: persistedPart.ReasoningContent,
		}

		if persistedPart.ToolCall != nil {
			callCopy := *persistedPart.ToolCall
			part.ToolCall = &callCopy
		}

		if persistedPart.ToolResponse != nil {
			toolResponse, err := deserializeToolResponse(*persistedPart.ToolResponse)
			if err != nil {
				return nil, fmt.Errorf("deserializeChatMessage() [session_persistence.go]: failed to deserialize tool response: %w", err)
			}
			part.ToolResponse = toolResponse
		}

		message.Parts = append(message.Parts, part)
	}

	return message, nil
}

func deserializeToolResponse(persisted persistedToolResponse) (*tool.ToolResponse, error) {
	response := &tool.ToolResponse{
		Result: persisted.Result,
		Done:   persisted.Done,
	}

	if persisted.Call != nil {
		callCopy := *persisted.Call
		response.Call = &callCopy
	}

	if strings.TrimSpace(persisted.Error) != "" {
		response.Error = errors.New(persisted.Error)
	}

	return response, nil
}
