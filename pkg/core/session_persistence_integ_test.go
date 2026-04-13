package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionPersistenceStateJSON(t *testing.T) {
	t.Run("overwrites session state and includes resumption-critical fields", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_persistence", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "dev",
			},
		})
		roles := NewAgentRoleRegistry(configStore)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.", withLogBaseDir(tmpDir), withWorkDir("/workspace/project"), withRoles(roles), withConfigStore(configStore))
		system := fixture.system
		handler := testutil.NewMockSessionOutputHandler()

		session, err := system.NewSession("ollama/test-model:latest", handler)
		require.NoError(t, err)

		todos := []tool.TodoItem{{
			ID:       "todo-1",
			Content:  "Implement persistence",
			Status:   "in_progress",
			Priority: "high",
		}}
		session.SetTodoList(todos)
		session.loadedAgentFiles = map[string]struct{}{"AGENTS.md": {}}
	session.pendingToolResponses = []*tool.ToolResponse{{
		Call: &tool.ToolCall{
			ID:       "call-1",
			Function: "runBash",
			Arguments: tool.NewToolValue(map[string]any{
				"command": "pwd",
			}),
		},
		Error: fmt.Errorf("permission required"),
		Result: tool.NewToolValue(map[string]any{
			"output": "ok",
		}),
		Done: true,
	}}
		session.messages = append(session.messages,
			models.NewTextMessage(models.ChatRoleUser, "resume me"),
			models.NewTextMessage(models.ChatRoleAssistant, "ready"),
		)
		session.tokenUsage = models.TokenUsage{InputTokens: 3, OutputTokens: 5, TotalTokens: 8}
		session.contextLength = 8
		session.compactionCount = 2

		session.persistSessionState()

		statePath := filepath.Join(tmpDir, "sessions", session.ID(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		require.NoError(t, err)

		var state persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytes, &state))

		assert.Equal(t, session.ID(), state.SessionID)
		assert.Equal(t, "ollama", state.ProviderName)
		assert.Equal(t, "test-model:latest", state.Model)
		assert.Equal(t, "developer", state.RoleName)
		assert.Equal(t, "/workspace/project", state.WorkDir)
		require.Len(t, state.TodoList, 1)
		assert.Equal(t, "todo-1", state.TodoList[0].ID)
		assert.GreaterOrEqual(t, len(state.Messages), 3)
	require.Len(t, state.PendingToolResponses, 1)
		assert.Equal(t, "call-1", state.PendingToolResponses[0].Call.ID)
		assert.Equal(t, "ok", state.PendingToolResponses[0].Result.Get("output").AsString())
		assert.True(t, strings.Contains(state.PendingToolResponses[0].Error, "permission required"))
		assert.Equal(t, []string{"AGENTS.md"}, state.LoadedAgentFiles)
		assert.Equal(t, 8, state.TokenUsage.TotalTokens)
		assert.Equal(t, 8, state.ContextLengthTokens)
		assert.Equal(t, 2, state.ContextCompactionCount)
		assert.NotEmpty(t, state.UpdatedAt)

		session.SetTodoList([]tool.TodoItem{{
			ID:       "todo-2",
			Content:  "Second state",
			Status:   "pending",
			Priority: "medium",
		}})

		stateBytesAfterOverwrite, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var stateAfterOverwrite persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytesAfterOverwrite, &stateAfterOverwrite))

		require.Len(t, stateAfterOverwrite.TodoList, 1)
		assert.Equal(t, "todo-2", stateAfterOverwrite.TodoList[0].ID)
	})
}

func TestSessionPersistenceLogDirectoryFallback(t *testing.T) {
	t.Run("uses system log base directory when logging package is not configured", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_persistence", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		session := &SweSession{
			id:         "session-1",
			logBaseDir: tmpDir,
		}

		dir := session.getSessionLogDirectory()
		expected := filepath.Join(tmpDir, "sessions", "session-1")
		assert.Equal(t, expected, dir)
	})
}

func TestSessionPersistenceSerializeChatMessage(t *testing.T) {
	t.Run("serializes tool responses with stringified errors", func(t *testing.T) {
		msg := models.NewTextMessage(models.ChatRoleAssistant, "")
		msg.Parts = append(msg.Parts, models.ChatMessagePart{
			ToolResponse: &tool.ToolResponse{
				Call:  &tool.ToolCall{ID: "call-err", Function: "vfsRead"},
				Error: assert.AnError,
				Done:  true,
			},
		})

		serialized := serializeChatMessage(msg)
		require.Len(t, serialized.Parts, 2)
		require.NotNil(t, serialized.Parts[1].ToolResponse)
		assert.Equal(t, assert.AnError.Error(), serialized.Parts[1].ToolResponse.Error)
	})
}

func TestSessionPersistenceStateRoleOptional(t *testing.T) {
	t.Run("stores empty role name when no role is set", func(t *testing.T) {
		session := &SweSession{
			id:           "s-1",
			providerName: "ollama",
			model:        "m",
			workDir:      ".",
			todoList:     []tool.TodoItem{},
			messages:     []*models.ChatMessage{},
		}

		state := session.buildPersistedSessionState()
		assert.Equal(t, "", state.RoleName)
	})
}

func TestSessionPersistenceSessionJSONAlwaysLatest(t *testing.T) {
	t.Run("session.json stores latest message snapshot", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_persistence", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "dev",
			},
		})
		roles := NewAgentRoleRegistry(configStore)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.", withLogBaseDir(tmpDir), withRoles(roles), withConfigStore(configStore))
		system := fixture.system
		mockServer := fixture.server
		handler := testutil.NewMockSessionOutputHandler()

		session, err := system.NewSession("ollama/test-model:latest", handler)
		require.NoError(t, err)

		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"First response"},"done":true}`)
		require.NoError(t, session.UserPrompt("first"))
		require.NoError(t, session.Run(context.Background()))

		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Second response"},"done":true}`)
		require.NoError(t, session.UserPrompt("second"))
		require.NoError(t, session.Run(context.Background()))

		statePath := filepath.Join(tmpDir, "sessions", session.ID(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		require.NoError(t, err)

		var state persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytes, &state))
		require.GreaterOrEqual(t, len(state.Messages), 5)
		lastMessage := state.Messages[len(state.Messages)-1]
		assert.Equal(t, "assistant", lastMessage.Role)
		assert.Equal(t, "Second response", lastMessage.Parts[0].Text)
	})
}

func TestSessionPersistenceSetRolePersisted(t *testing.T) {
	t.Run("persists role changes into session state file", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_persistence", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		configStore := impl.NewMockConfigStore()
		configStore.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
			"developer": {
				Name:        "developer",
				Description: "dev",
			},
			"tester": {
				Name:        "tester",
				Description: "qa",
			},
		})
		roles := NewAgentRoleRegistry(configStore)

		fixture := newSweSystemFixture(t, "You are a helpful assistant.", withLogBaseDir(tmpDir), withRoles(roles), withConfigStore(configStore))
		system := fixture.system
		handler := testutil.NewMockSessionOutputHandler()
		session, err := system.NewSession("ollama/test-model:latest", handler)
		require.NoError(t, err)

		require.NoError(t, session.SetRole("tester"))

		statePath := filepath.Join(tmpDir, "sessions", session.ID(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		require.NoError(t, err)

		var state persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytes, &state))
		assert.Equal(t, "tester", state.RoleName)
	})
}
