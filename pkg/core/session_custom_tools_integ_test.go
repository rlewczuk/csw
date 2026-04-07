package core

import (
	"context"
	"testing"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/conf/impl"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/runner"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCustomToolIntegration(t *testing.T) {
	store := impl.NewMockConfigStore()
	store.SetAgentRoleConfigs(map[string]*conf.AgentRoleConfig{
		"all": {
			Name: "all",
			ToolFragments: map[string]string{
				"customPing/customPing.md":          "Custom ping tool",
				"customPing/customPing.schema.json": `{"type":"object","description":"Ping tool","properties":{"message":{"type":"string"}},"required":["message"]}`,
				"customPing/customPing.json":        `{"command":"echo {{.arg.message}}","result":{"reply":"{{.stdout}}"}}`,
				"customPing/.tooldir":               "/tmp/customPing",
			},
		},
		"developer": {
			Name: "developer",
			ToolsAccess: map[string]conf.AccessFlag{
				"**": conf.AccessAllow,
			},
			VFSPrivileges: map[string]conf.FileAccess{
				"**": {
					Read:   conf.AccessAllow,
					Write:  conf.AccessAllow,
					Delete: conf.AccessAllow,
					List:   conf.AccessAllow,
					Find:   conf.AccessAllow,
					Move:   conf.AccessAllow,
				},
			},
		},
	})
	store.SetGlobalConfig(&conf.GlobalConfig{Defaults: conf.CLIDefaultsConfig{DefaultRole: "developer"}})

	promptGenerator, err := NewConfPromptGenerator(store, nil)
	require.NoError(t, err)

	roleRegistry := NewAgentRoleRegistry(store)
	mockRunner := runner.NewMockRunner()
	mockRunner.SetResponse("echo pong", "pong", 0, nil)

	tools := tool.NewToolRegistry()
	require.NoError(t, tool.RegisterCustomTools(tools, store, ".", mockRunner))

	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model", Model: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
		Role: models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{{ToolCall: &tool.ToolCall{
			ID:       "custom-1",
			Function: "customPing",
			Arguments: tool.NewToolValue(map[string]any{
				"message": "pong",
			}),
		}}},
	}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
		Role:  models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{{Text: "done"}},
	}})

	fixture := newSweSystemFixture(t, "", withConfigStore(store), withRoles(roleRegistry), withPromptGenerator(promptGenerator), withTools(tools), withoutVFSTools(), withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}))
	mockHandler := testutil.NewMockSessionOutputHandler()

	session, err := fixture.system.NewSession("mock/test-model", mockHandler)
	require.NoError(t, err)
	require.NoError(t, session.UserPrompt("please ping"))
	require.NoError(t, session.Run(context.Background()))

	exec := mockRunner.GetLastExecution()
	require.NotNil(t, exec)
	assert.Equal(t, "echo pong", exec.Command)

	require.NotEmpty(t, mockProvider.RecordedToolResponses)
	assert.Equal(t, "pong", mockProvider.RecordedToolResponses[0].Result.Get("reply").AsString())
}
