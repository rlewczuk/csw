package core

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/conf"
	"github.com/rlewczuk/csw/pkg/models"
	"github.com/rlewczuk/csw/pkg/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type orderedSessionOutputHandler struct {
	mu       sync.Mutex
	events   []string
	finished chan struct{}
	err      error
}

func newOrderedSessionOutputHandler() *orderedSessionOutputHandler {
	return &orderedSessionOutputHandler{
		events:   make([]string, 0),
		finished: make(chan struct{}),
	}
}

func (h *orderedSessionOutputHandler) ShowMessage(message string, messageType string) {
	_ = message
	_ = messageType
}

func (h *orderedSessionOutputHandler) AddUserMessage(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, "user:"+text)
}

func (h *orderedSessionOutputHandler) AddAssistantMessage(text string, thinking string) {
	_ = text
	_ = thinking
}

func (h *orderedSessionOutputHandler) AddToolCall(call *tool.ToolCall) {
	_ = call
}

func (h *orderedSessionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, "tool_result")
	_ = result
}

func (h *orderedSessionOutputHandler) RunFinished(err error) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
	close(h.finished)
}

func (h *orderedSessionOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	_ = retryAfterSeconds
}

func (h *orderedSessionOutputHandler) WaitForFinished(t *testing.T) {
	t.Helper()
	select {
	case <-h.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for session finish")
	}
}

func (h *orderedSessionOutputHandler) Events() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]string, len(h.events))
	copy(result, h.events)
	return result
}

func (h *orderedSessionOutputHandler) FinishedError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

// probeTool is a test double that can be synchronized to verify parallel execution.
type probeTool struct {
	started      chan string
	release      <-chan struct{}
	active       *int32
	maxActive    *int32
	overlap      *atomic.Bool
	startTimes   *[]time.Time
	startTimesMu *sync.Mutex
	response     func(call *tool.ToolCall) *tool.ToolResponse
}

func (t *probeTool) Execute(args *tool.ToolCall) *tool.ToolResponse {
	if t.active != nil {
		current := atomic.AddInt32(t.active, 1)
		if current > 1 && t.overlap != nil {
			t.overlap.Store(true)
		}
		if t.maxActive != nil {
			for {
				maxCurrent := atomic.LoadInt32(t.maxActive)
				if current <= maxCurrent {
					break
				}
				if atomic.CompareAndSwapInt32(t.maxActive, maxCurrent, current) {
					break
				}
			}
		}
		defer atomic.AddInt32(t.active, -1)
	}

	if t.startTimes != nil && t.startTimesMu != nil {
		t.startTimesMu.Lock()
		*t.startTimes = append(*t.startTimes, time.Now())
		t.startTimesMu.Unlock()
	}

	if t.started != nil {
		t.started <- args.ID
	}

	if t.release != nil {
		<-t.release
	}

	if t.response != nil {
		return t.response(args)
	}

	return &tool.ToolResponse{Call: args, Done: true}
}

func (t *probeTool) Render(call *tool.ToolCall) (string, string, string, map[string]string) {
	return "probe", "probe", "{}", map[string]string{}
}

func (t *probeTool) GetDescription() (string, bool) {
	return "", false
}

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

	handler := newOrderedSessionOutputHandler()
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

	events := handler.Events()
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

func TestExecuteToolCalls_ExecutesCallsInParallelAndAggregatesResponses(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var active int32
	overlap := &atomic.Bool{}

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{
		started: started,
		release: release,
		active:  &active,
		overlap: overlap,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{Call: call, Done: true, Result: tool.NewToolValue(map[string]any{"content": "A"})}
		},
	})
	registry.Register("toolB", &probeTool{
		started: started,
		release: release,
		active:  &active,
		overlap: overlap,
		response: func(call *tool.ToolCall) *tool.ToolResponse {
			return &tool.ToolResponse{Call: call, Done: true, Result: tool.NewToolValue(map[string]any{"content": "B"})}
		},
	})

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		toolStartDelay: time.Millisecond,
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.executeToolCalls(calls)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tool %d to start", i+1)
		}
	}

	close(release)
	require.NoError(t, <-errCh)

	assert.True(t, overlap.Load(), "tool calls should overlap in execution")
	require.Len(t, session.messages, 1)

	toolMessage := session.messages[0]
	assert.Equal(t, models.ChatRoleUser, toolMessage.Role)
	require.Len(t, toolMessage.GetToolResponses(), 2)
	assert.Equal(t, "call-1", toolMessage.GetToolResponses()[0].Call.ID)
	assert.Equal(t, "call-2", toolMessage.GetToolResponses()[1].Call.ID)
}

func TestExecuteToolCalls_RespectsConfiguredMaxThreadsAndQueuesRemainingCalls(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	var active int32
	var maxActive int32

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolB", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})
	registry.Register("toolC", &probeTool{started: started, release: release, active: &active, maxActive: &maxActive})

	configStore := &conf.CswConfig{}

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		config:         configStore,
		maxToolThreads: 2,
		toolStartDelay: time.Millisecond,
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-3", Function: "toolC", Arguments: tool.NewToolValue(map[string]any{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.executeToolCalls(calls)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for tool %d to start", i+1)
		}
	}

	select {
	case id := <-started:
		t.Fatalf("unexpected third tool started before queue release: %s", id)
	case <-time.After(5 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-errCh)

	require.Len(t, session.messages, 1)
	assert.LessOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}

func TestExecuteToolCalls_SpacesToolStartsByMinimumDelay(t *testing.T) {
	startTimes := make([]time.Time, 0, 3)
	startTimesMu := sync.Mutex{}

	registry := tool.NewToolRegistry()
	registry.Register("toolA", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})
	registry.Register("toolB", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})
	registry.Register("toolC", &probeTool{startTimes: &startTimes, startTimesMu: &startTimesMu})

	session := &SweSession{
		Tools:          registry,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		maxToolThreads: 8,
		toolStartDelay: time.Millisecond,
	}

	calls := []*tool.ToolCall{
		{ID: "call-1", Function: "toolA", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-2", Function: "toolB", Arguments: tool.NewToolValue(map[string]any{})},
		{ID: "call-3", Function: "toolC", Arguments: tool.NewToolValue(map[string]any{})},
	}

	require.NoError(t, session.executeToolCalls(calls))
	startTimesMu.Lock()
	defer startTimesMu.Unlock()
	require.Len(t, startTimes, 3)

	for i := 1; i < len(startTimes); i++ {
		delta := startTimes[i].Sub(startTimes[i-1])
		assert.GreaterOrEqual(t, delta, 500*time.Microsecond)
	}
}

func TestMaxToolThreadsLimit_UsesOverrideThenConfigThenDefault(t *testing.T) {
	configStore := &conf.CswConfig{GlobalConfig: &conf.GlobalConfig{Defaults: conf.RunDefaultsConfig{MaxThreads: 11}}}

	session := &SweSession{config: configStore}
	assert.Equal(t, 11, session.maxToolThreadsLimit())

	session.maxToolThreads = 3
	assert.Equal(t, 3, session.maxToolThreadsLimit())

	configStore.GlobalConfig = nil
	session.maxToolThreads = 0
	assert.Equal(t, defaultMaxToolThreads, session.maxToolThreadsLimit())
}
