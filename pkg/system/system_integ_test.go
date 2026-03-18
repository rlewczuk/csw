package system_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/core"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSystemSessionManagement(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System
	handler := testutil.NewMockSessionOutputHandler()

	session, err := system.NewSession("ollama/test-model:latest", handler)
	require.NoError(t, err)
	require.NotNil(t, session)

	stored, err := system.GetSession(session.ID())
	require.NoError(t, err)
	assert.Equal(t, session, stored)

	sessions := system.ListSessions()
	require.NotEmpty(t, sessions)

	thread1, err := system.GetSessionThread(session.ID())
	require.NoError(t, err)
	thread2, err := system.GetSessionThread(session.ID())
	require.NoError(t, err)
	assert.Equal(t, thread1, thread2)

	err = system.DeleteSession(session.ID())
	require.NoError(t, err)

	_, err = system.GetSession(session.ID())
	require.Error(t, err)
}

func TestSweSystemShutdownClearsSessions(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System
	handler := testutil.NewMockSessionOutputHandler()

	session1, err := system.NewSession("ollama/test-model:latest", handler)
	require.NoError(t, err)
	session2, err := system.NewSession("ollama/test-model:latest", handler)
	require.NoError(t, err)

	_, err = system.GetSessionThread(session1.ID())
	require.NoError(t, err)
	_, err = system.GetSessionThread(session2.ID())
	require.NoError(t, err)

	system.Shutdown()

	assert.Empty(t, system.ListSessions())
	_, err = system.GetSession(session1.ID())
	require.Error(t, err)
	_, err = system.GetSession(session2.ID())
	require.Error(t, err)
}

func TestSystemStreamingConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		streaming *bool
	}{
		{name: "streaming enabled", streaming: func() *bool { v := true; return &v }()},
		{name: "streaming disabled", streaming: func() *bool { v := false; return &v }()},
		{name: "streaming not configured", streaming: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := coretestfixture.NewSweSystemFixture(t)
			system := fixture.System
			mockServer := fixture.Server

			config := &conf.ModelProviderConfig{
				Type:      "ollama",
				Name:      "ollama",
				URL:       mockServer.URL(),
				Streaming: tc.streaming,
			}
			client, err := models.NewOllamaClient(config)
			require.NoError(t, err)
			system.ModelProviders = map[string]models.ModelProvider{"ollama": client}

			session, err := system.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
			require.NoError(t, err)
			assert.NotNil(t, session)
		})
	}
}

func TestLogLLMRequestsOption(t *testing.T) {
	t.Run("session has llm logger when LogLLMRequests is enabled", func(t *testing.T) {
		fixture := coretestfixture.NewSweSystemFixture(t, coretestfixture.WithLogLLMRequests(true))
		session, err := fixture.System.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
		require.NoError(t, err)

		llmLoggerField := reflect.ValueOf(session).Elem().FieldByName("llmLogger")
		require.True(t, llmLoggerField.IsValid())
		assert.False(t, llmLoggerField.IsNil())
	})

	t.Run("session has nil llm logger when LogLLMRequests is disabled", func(t *testing.T) {
		fixture := coretestfixture.NewSweSystemFixture(t, coretestfixture.WithLogLLMRequests(false))
		session, err := fixture.System.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
		require.NoError(t, err)

		llmLoggerField := reflect.ValueOf(session).Elem().FieldByName("llmLogger")
		require.True(t, llmLoggerField.IsValid())
		assert.True(t, llmLoggerField.IsNil())
	})
}

func TestSweSystem_SubAgentIntegration(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System
	mockServer := fixture.Server

	parentHandler := testutil.NewMockSessionOutputHandler()
	parent, err := system.NewSession("ollama/test-model:latest", parentHandler)
	require.NoError(t, err)

	tmpLogs := t.TempDir()
	system.LogBaseDir = tmpLogs

	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Child completed."},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	result, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "child-summary",
		Title:  "Child summary",
		Prompt: "Provide brief summary",
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)

	childSessions := system.ListSessions()
	require.GreaterOrEqual(t, len(childSessions), 2)

	var child *core.SweSession
	for _, session := range childSessions {
		if session.ID() == parent.ID() {
			continue
		}
		if session.ParentID() == parent.ID() {
			child = session
			break
		}
	}
	require.NotNil(t, child)
	assert.Equal(t, "child-summary", child.Slug())

	summaryPath := filepath.Join(tmpLogs, "sessions", child.ID(), "summary.json")
	summaryBytes, readErr := os.ReadFile(summaryPath)
	require.NoError(t, readErr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(summaryBytes, &summary))
	assert.Equal(t, parent.ID(), summary["parent_session_id"])
	assert.Equal(t, child.ID(), summary["session_id"])
}

type hookFeedbackExecutorCapture struct {
	requests []tool.HookFeedbackRequest
	response tool.HookFeedbackResponse
}

func (e *hookFeedbackExecutorCapture) ExecuteHookFeedback(request tool.HookFeedbackRequest) tool.HookFeedbackResponse {
	e.requests = append(e.requests, request)
	if e.response.Fn == "" {
		return tool.HookFeedbackResponse{Fn: request.Fn, ID: request.ID, OK: true, Result: map[string]any{"ok": true}}
	}
	return e.response
}

func TestSweSystem_SubAgentRegistersHookFeedbackToolOnlyWhenEnabled(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System

	parent, err := system.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)

	capture := &hookFeedbackExecutorCapture{}

	_, err = system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:                 "hook-child",
		Title:                "Hook child",
		Prompt:               "Run no-op",
		HookFeedbackExecutor: capture,
	})
	require.NoError(t, err)

	var childWithHook *core.SweSession
	for _, session := range system.ListSessions() {
		if session.ParentID() == parent.ID() && session.Slug() == "hook-child" {
			childWithHook = session
			break
		}
	}
	require.NotNil(t, childWithHook)
	_, toolErr := childWithHook.Tools.Get("hookFeedback")
	require.NoError(t, toolErr)

	normalParent, err := system.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)
	_, err = system.ExecuteSubAgentTask(normalParent, tool.SubAgentTaskRequest{
		Slug:   "normal-child",
		Title:  "Normal child",
		Prompt: "Run no-op",
	})
	require.NoError(t, err)

	var normalChild *core.SweSession
	for _, session := range system.ListSessions() {
		if session.ParentID() == normalParent.ID() && session.Slug() == "normal-child" {
			normalChild = session
			break
		}
	}
	require.NotNil(t, normalChild)
	_, toolErr = normalChild.Tools.Get("hookFeedback")
	require.Error(t, toolErr)
}

func TestSweSystem_SubAgentSummaryUsesHookFeedbackResponseStdout(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System
	mockServer := fixture.Server

	parent, err := system.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)

	// Child performs hookFeedback(response) tool call and then returns a fallback summary.
	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"hookfb-1","type":"function","function":{"name":"hookFeedback","arguments":{"fn":"response","args":{"status":"OK","stdout":"stdout from response"},"id":"resp-1"}}}]},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"fallback summary"},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	capture := &hookFeedbackExecutorCapture{response: tool.HookFeedbackResponse{Fn: "response", ID: "resp-1", OK: true, Result: map[string]any{"stdout": "stdout from response"}}}
	result, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:                 "hook-summary-child",
		Title:                "Hook summary child",
		Prompt:               "Send hook feedback response",
		HookFeedbackExecutor: capture,
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "stdout from response", result.Summary)
	require.Len(t, capture.requests, 1)
	assert.Equal(t, "response", capture.requests[0].Fn)

	// Ensure normal subagent still uses assistant summary when no response feedback exists.
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:04Z","message":{"role":"assistant","content":"normal summary"},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:05Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	normalResult, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "normal-summary-child",
		Title:  "Normal summary child",
		Prompt: "No hook feedback",
	})
	require.NoError(t, err)
	assert.Equal(t, "normal summary", normalResult.Summary)
}

func TestSweSystem_SubAgentSummaryResponseStatusError(t *testing.T) {
	fixture := coretestfixture.NewSweSystemFixture(t)
	system := fixture.System
	mockServer := fixture.Server

	parent, err := system.NewSession("ollama/test-model:latest", testutil.NewMockSessionOutputHandler())
	require.NoError(t, err)

	mockServer.AddStreamingResponse("/api/chat", "POST", false,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"hookfb-1","type":"function","function":{"name":"hookFeedback","arguments":{"fn":"response","args":{"status":"ERROR","stderr":"boom stderr"},"id":"resp-err"}}}]},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:01Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)
	mockServer.AddStreamingResponse("/api/chat", "POST", true,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:02Z","message":{"role":"assistant","content":"unused fallback"},"done":false}`,
		`{"model":"test-model:latest","created_at":"2024-01-01T00:00:03Z","message":{"role":"assistant"},"done":true,"done_reason":"stop"}`,
	)

	capture := &hookFeedbackExecutorCapture{response: tool.HookFeedbackResponse{Fn: "response", ID: "resp-err", OK: true, Result: map[string]any{"status": "ERROR", "stderr": "boom stderr"}}}
	result, err := system.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:                 "hook-error-child",
		Title:                "Hook error child",
		Prompt:               "Send error response",
		HookFeedbackExecutor: capture,
	})
	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)
	assert.Equal(t, "boom stderr", result.Summary)
	assert.Equal(t, "boom stderr", result.Error)
}
