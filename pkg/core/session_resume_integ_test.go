package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSystemLoadSessionResumption(t *testing.T) {
	t.Run("loads persisted session and resumes pending work", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
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

		statePath := filepath.Join(tmpDir, "sessions", session.ID(), "session.json")
		stateBytes, err := os.ReadFile(statePath)
		require.NoError(t, err)

		var state persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytes, &state))

		state.Messages = append(state.Messages,
			serializeChatMessage(models.NewTextMessage(models.ChatRoleUser, "Please continue.")),
		)

		state.PendingPermissionToolCalls = []tool.ToolCall{
			{
				ID:       "call-1",
				Function: "vfsRead",
				Arguments: tool.NewToolValue(map[string]any{
					"path": "README.md",
				}),
			},
		}

		rewrittenState, err := json.MarshalIndent(state, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(statePath, rewrittenState, 0644))

		system.Shutdown()

		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"Resumed and done"},"done":true}`)

		resumedHandler := testutil.NewMockSessionOutputHandler()
		loadedSession, err := system.LoadSession(session.ID(), resumedHandler)
		require.NoError(t, err)
		require.NotNil(t, loadedSession)
		assert.Equal(t, session.ID(), loadedSession.ID())
		assert.Equal(t, "ollama", loadedSession.ProviderName())
		assert.Equal(t, "test-model:latest", loadedSession.Model())
		assert.True(t, loadedSession.HasPendingWork())

		thread := NewSessionThreadWithSession(system, loadedSession, resumedHandler)
		require.NoError(t, thread.ResumePending())

		resumedHandler.WaitForRunFinished()
		require.NoError(t, resumedHandler.RunFinishedError)
		assert.NotEmpty(t, resumedHandler.AssistantMessages)
		assert.Equal(t, "Resumed and done", resumedHandler.AssistantMessages[len(resumedHandler.AssistantMessages)-1].Text)

		stateBytesAfterRun, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var stateAfterRun persistedSessionState
		require.NoError(t, json.Unmarshal(stateBytesAfterRun, &stateAfterRun))
		assert.Empty(t, stateAfterRun.PendingPermissionToolCalls)
	})
}

func TestSweSystemLoadLastSession(t *testing.T) {
	t.Run("loads latest session by update timestamp", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		fixture := newSweSystemFixture(t, "You are a helper.", withLogBaseDir(tmpDir))
		system := fixture.system
		handler := testutil.NewMockSessionOutputHandler()

		session1, err := system.NewSession("ollama/model-a:latest", handler)
		require.NoError(t, err)
		session2, err := system.NewSession("ollama/model-b:latest", handler)
		require.NoError(t, err)

		require.NoError(t, session1.UserPrompt("older"))
		require.NoError(t, session2.UserPrompt("newer"))

		system.Shutdown()

		loaded, err := system.LoadLastSession(handler)
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Equal(t, session2.ID(), loaded.ID())
		assert.NotEqual(t, session1.ID(), loaded.ID())
	})

	t.Run("returns error when no persisted sessions exist", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		fixture := newSweSystemFixture(t, "You are a helper.", withLogBaseDir(tmpDir))
		system := fixture.system

		_, err := system.LoadLastSession(testutil.NewMockSessionOutputHandler())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no persisted sessions found")
	})
}

func TestSweSessionHasPendingWork(t *testing.T) {
	t.Run("detects pending cases", func(t *testing.T) {
		session := &SweSession{}
		assert.False(t, session.HasPendingWork())

		session.messages = []*models.ChatMessage{models.NewTextMessage(models.ChatRoleUser, "next")}
		assert.True(t, session.HasPendingWork())

		session.messages = []*models.ChatMessage{
			models.NewTextMessage(models.ChatRoleAssistant, ""),
		}
		session.messages[0].AddToolCall(&tool.ToolCall{ID: "1", Function: "todoRead", Arguments: tool.NewToolValue(map[string]any{})})
		assert.True(t, session.HasPendingWork())

		session.messages = []*models.ChatMessage{models.NewTextMessage(models.ChatRoleAssistant, "done")}
		session.pendingPermissionToolCalls = []*tool.ToolCall{{ID: "2", Function: "todoRead", Arguments: tool.NewToolValue(map[string]any{})}}
		assert.True(t, session.HasPendingWork())

		session.pendingPermissionToolCalls = nil
		session.pendingToolResponses = []*tool.ToolResponse{{Done: true}}
		assert.True(t, session.HasPendingWork())

		session.pendingToolResponses = nil
		assert.False(t, session.HasPendingWork())
	})
}

func TestSweSystemLoadSessionReturnsExistingSession(t *testing.T) {
	fixture := newSweSystemFixture(t, "You are a helper.")
	system := fixture.system
	handler := testutil.NewMockSessionOutputHandler()

	session, err := system.NewSession("ollama/model-a:latest", handler)
	require.NoError(t, err)

	loaded, err := system.LoadSession(session.ID(), handler)
	require.NoError(t, err)
	assert.Equal(t, session, loaded)
}

func TestSweSystemLoadSessionInvalidState(t *testing.T) {
	t.Run("returns error for malformed session state", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{},
			LogBaseDir:     tmpDir,
		}

		sessionID := "018f6e30-3acb-7f24-bede-8d96cd157152"
		sessionDir := filepath.Join(tmpDir, "sessions", sessionID)
		require.NoError(t, os.MkdirAll(sessionDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "session.json"), []byte("not-json"), 0644))

		_, err := system.LoadSession(sessionID, testutil.NewMockSessionOutputHandler())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal session state")
	})

	t.Run("returns error when provider is missing", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{},
			LogBaseDir:     tmpDir,
		}

		sessionID := "018f6e30-3acb-7f24-bede-8d96cd157152"
		state := persistedSessionState{
			SessionID:    sessionID,
			ProviderName: "ollama",
			Model:        "test-model:latest",
			WorkDir:      ".",
		}
		stateBytes, err := json.Marshal(state)
		require.NoError(t, err)

		sessionDir := filepath.Join(tmpDir, "sessions", sessionID)
		require.NoError(t, os.MkdirAll(sessionDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "session.json"), stateBytes, 0644))

		_, err = system.LoadSession(sessionID, testutil.NewMockSessionOutputHandler())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provider not found")
	})
}

func TestSessionResumeRoundTripRun(t *testing.T) {
	t.Run("loaded session can run with new prompt", func(t *testing.T) {
		tmpDir := filepath.Join("../../tmp", "session_resume", t.Name())
		require.NoError(t, os.MkdirAll(tmpDir, 0755))
		defer os.RemoveAll(tmpDir)

		fixture := newSweSystemFixture(t, "You are a helper.", withLogBaseDir(tmpDir))
		system := fixture.system
		mockServer := fixture.server
		handler := testutil.NewMockSessionOutputHandler()

		session, err := system.NewSession("ollama/test-model:latest", handler)
		require.NoError(t, err)

		require.NoError(t, session.UserPrompt("first"))
		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"first done"},"done":true}`)
		require.NoError(t, session.Run(context.Background()))

		system.Shutdown()

		loaded, err := system.LoadSession(session.ID(), handler)
		require.NoError(t, err)
		require.NoError(t, loaded.UserPrompt("second"))

		mockServer.AddRestResponse("/api/chat", "POST", `{"model":"test-model:latest","message":{"role":"assistant","content":"second done"},"done":true}`)
		require.NoError(t, loaded.Run(context.Background()))

		messages := loaded.ChatMessages()
		require.NotEmpty(t, messages)
		assert.Equal(t, "second done", messages[len(messages)-1].GetText())
	})
}
