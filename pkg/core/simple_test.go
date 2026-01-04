package core

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/models"
	"github.com/codesnort/codesnort-swe/pkg/models/ollama"
	"github.com/codesnort/codesnort-swe/pkg/testutil"
	"github.com/codesnort/codesnort-swe/pkg/tool"
	"github.com/codesnort/codesnort-swe/pkg/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentCoreInitializationAndSimpleProgramGen(t *testing.T) {
	mockServer := testutil.NewMockHTTPServer()
	defer mockServer.Close()
	client, err := ollama.NewOllamaClientWithHTTPClient(mockServer.URL(), mockServer.Client())
	require.NoError(t, err)
	vfs := vfs.NewMockVFS()

	tools := tool.NewToolRegistry()
	tool.RegisterVFSTools(tools, vfs)

	t.Run("basic initialization", func(t *testing.T) {
		system := &SweSystem{
			ModelProviders: map[string]models.ModelProvider{"ollama": client},
			SystemPrompt:   "You are skilled software developer.",
			Tools:          tools,
			VFS:            vfs,
		}

		session, err := system.NewSession("ollama/devstral-small-2:latest")
		require.NoError(t, err)
		assert.NotNil(t, session)

		// TODO populate mock server with LLM response

		err = session.UserPrompt("Implement Hello World program in Python")
		assert.NoError(t, err)

		err = session.Run()
		assert.NoError(t, err)

		// TODO check in the mock that LLM was called with expected prompt
		// TODO check in the mock that LLM was called with expected tool responses
		// TODO check if system prompt was sent

		bytes, err := vfs.ReadFile("hello_world.py")
		assert.NoError(t, err)
		assert.Contains(t, string(bytes), "print(\"Hello World\")")
	})
}
