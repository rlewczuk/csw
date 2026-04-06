package system_test

import (
	"testing"

	coretestfixture "github.com/rlewczuk/csw/pkg/core/testfixture"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSweSystem_SubAgentForwardsToolEventsWithSubAgentSlug(t *testing.T) {
	mockProvider := models.NewMockProvider(nil)
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: &models.ChatMessage{
			Role: models.ChatRoleAssistant,
			Parts: []models.ChatMessagePart{
				{
					ToolCall: &tool.ToolCall{
						ID:       "tool-1",
						Function: "vfsRead",
						Arguments: tool.NewToolValue(map[string]any{
							"path": "missing.txt",
						}),
					},
				},
			},
		},
	})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{
		Response: models.NewTextMessage(models.ChatRoleAssistant, "Done."),
	})

	fixture := coretestfixture.NewSweSystemFixture(t,
		coretestfixture.WithProviderName("mock"),
		coretestfixture.WithModelProvider(mockProvider),
	)

	parentHandler := testutil.NewMockSessionOutputHandler()
	parent, err := fixture.System.NewSession("mock/test-model", parentHandler)
	require.NoError(t, err)

	_, err = fixture.System.ExecuteSubAgentTask(parent, tool.SubAgentTaskRequest{
		Slug:   "child-agent",
		Title:  "Child agent",
		Prompt: "Try reading missing.txt",
	})
	require.NoError(t, err)

	require.NotEmpty(t, parentHandler.ToolCalls)
	assert.Equal(t, "child-agent", parentHandler.ToolCalls[0].Arguments.String("__subagent_slug"))

	require.NotEmpty(t, parentHandler.ToolCallResults)
	require.NotNil(t, parentHandler.ToolCallResults[0].Call)
	assert.Equal(t, "child-agent", parentHandler.ToolCallResults[0].Call.Arguments.String("__subagent_slug"))
}
