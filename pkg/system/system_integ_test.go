package system_test

import (
	"reflect"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
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
