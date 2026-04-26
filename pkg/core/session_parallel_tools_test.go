package core

import (
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/testutil"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingTool waits on release channel so tests can enqueue additional prompts before completing.
type blockingTool struct {
	release <-chan struct{}
}

func (t *blockingTool) Execute(args *tool.ToolCall) *tool.ToolResponse {
	if t.release != nil {
		<-t.release
	}

	return &tool.ToolResponse{
		Call:   args,
		Done:   true,
		Result: tool.NewToolValue(map[string]any{"status": "ok"}),
	}
}

func (t *blockingTool) Render(call *tool.ToolCall) (string, string, string, map[string]string) {
	_ = call
	return "blocking", "blocking", "{}", map[string]string{}
}

func (t *blockingTool) GetDescription() (string, bool) {
	return "", false
}

func TestSessionRun_FlushesQueuedUserPromptsAfterToolResponsesBeforeNextLLMRequest(t *testing.T) {
	releaseTool := make(chan struct{})

	registry := tool.NewToolRegistry()
	registry.Register("block", &blockingTool{release: releaseTool})

	mockProvider := models.NewMockProvider([]models.ModelInfo{{Name: "test-model", Model: "test-model"}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
		Role: models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{{
			ToolCall: &tool.ToolCall{
				ID:        "call-1",
				Function:  "block",
				Arguments: tool.NewToolValue(map[string]any{}),
			},
		}},
	}})
	mockProvider.SetChatResponse("test-model", &models.MockChatResponse{Response: &models.ChatMessage{
		Role:  models.ChatRoleAssistant,
		Parts: []models.ChatMessagePart{{Text: "done"}},
	}})
	fixture := newSweSystemFixture(t, "You are a test assistant.",
		withModelProviders(map[string]models.ModelProvider{"mock": mockProvider}),
		withTools(registry),
		withoutVFSTools(),
	)

	handler := testutil.NewMockSessionOutputHandler()
	thread := NewSessionThread(fixture.system, handler)
	require.NoError(t, thread.StartSession("mock/test-model"))
	require.NoError(t, thread.UserPrompt("first prompt"))

	require.Eventually(t, func() bool {
		return len(mockProvider.RecordedMessages) >= 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, thread.UserPrompt("queued prompt"))

	close(releaseTool)

	handler.WaitForFinished(t)
	require.NoError(t, handler.FinishedError())
	require.GreaterOrEqual(t, len(mockProvider.RecordedMessages), 2)

	secondCallMessages := mockProvider.RecordedMessages[1]
	toolResponseIndex := -1
	queuedPromptIndex := -1
	for i, msg := range secondCallMessages {
		if msg.Role == models.ChatRoleUser && len(msg.GetToolResponses()) > 0 {
			toolResponseIndex = i
		}
		if msg.Role == models.ChatRoleUser && msg.GetText() == "queued prompt" {
			queuedPromptIndex = i
		}
	}

	require.NotEqual(t, -1, toolResponseIndex, "tool response should be present before second LLM call")
	require.NotEqual(t, -1, queuedPromptIndex, "queued user prompt should be present before second LLM call")
	assert.Greater(t, queuedPromptIndex, toolResponseIndex, "queued prompt should be appended after tool responses")

	events := handler.EventsSnapshot()
	toolResultEventIndex := -1
	queuedEventIndex := -1
	for i, event := range events {
		if event == "tool_result" && toolResultEventIndex == -1 {
			toolResultEventIndex = i
		}
		if event == "user:queued prompt" {
			queuedEventIndex = i
		}
	}
	require.NotEqual(t, -1, toolResultEventIndex, "tool result event should be emitted")
	require.NotEqual(t, -1, queuedEventIndex, "queued user event should be emitted")
	assert.Greater(t, queuedEventIndex, toolResultEventIndex, "queued user event should be emitted after tool result")
}
