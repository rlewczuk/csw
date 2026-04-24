package testutil

import (
	"sync"
	"testing"
	"time"

	"github.com/rlewczuk/csw/pkg/tool"
)

// AssistantMessageRecord stores assistant output captured from session output handler.
type AssistantMessageRecord struct {
	Text     string
	Thinking string
}

// SessionMessageRecord stores message output captured from ShowMessage.
type SessionMessageRecord struct {
	Message     string
	MessageType string
}

// MockSessionOutputHandler is a mock implementation of SessionOutputHandler that keeps all output in memory.
// It is used for testing and capturing output from agent operations.
type MockSessionOutputHandler struct {
	// UserMessages stores user prompts received via AddUserMessage.
	UserMessages []string

	// AssistantMessages stores assistant outputs received via AddAssistantMessage.
	AssistantMessages []AssistantMessageRecord

	// Events stores ordered high-level events emitted by handler callbacks.
	Events []string

	// ToolCalls stores all tool calls received via AddToolCall.
	ToolCalls []*tool.ToolCall

	// ToolCallResults stores all tool responses received via AddToolCallResult.
	ToolCallResults []*tool.ToolResponse

	// StatusMessages stores all status messages received via ShowMessage.
	StatusMessages []SessionMessageRecord

	// RateLimitErrors stores all rate limit errors received via OnRateLimitError.
	RateLimitErrors      []int
	rateLimitErrorCalled chan struct{}

	// RunFinishedError stores the error from RunFinished call.
	RunFinishedError  error
	RunFinishedCalls  int
	runFinishedCalled chan struct{}
	runFinishedClosed bool

	mu sync.Mutex
}

// NewMockSessionOutputHandler creates a new MockSessionOutputHandler.
func NewMockSessionOutputHandler() *MockSessionOutputHandler {
	return &MockSessionOutputHandler{
		UserMessages:          make([]string, 0),
		AssistantMessages:     make([]AssistantMessageRecord, 0),
		Events:                make([]string, 0),
		ToolCalls:             make([]*tool.ToolCall, 0),
		ToolCallResults:       make([]*tool.ToolResponse, 0),
		StatusMessages:        make([]SessionMessageRecord, 0),
		rateLimitErrorCalled:  make(chan struct{}, 10),
		runFinishedCalled:     make(chan struct{}),
	}
}

// OnRateLimitError records a rate limit error.
func (h *MockSessionOutputHandler) OnRateLimitError(retryAfterSeconds int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.RateLimitErrors = append(h.RateLimitErrors, retryAfterSeconds)
	select {
	case h.rateLimitErrorCalled <- struct{}{}:
	default:
	}
}

// ShowMessage records status message.
func (h *MockSessionOutputHandler) ShowMessage(message string, messageType string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, "status")
	h.StatusMessages = append(h.StatusMessages, SessionMessageRecord{Message: message, MessageType: messageType})
}

// AddUserMessage records user message.
func (h *MockSessionOutputHandler) AddUserMessage(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.UserMessages = append(h.UserMessages, text)
	h.Events = append(h.Events, "user:"+text)
}

// WaitForRateLimitError blocks until OnRateLimitError is called.
func (h *MockSessionOutputHandler) WaitForRateLimitError() { <-h.rateLimitErrorCalled }

// AddAssistantMessage records a full assistant output.
func (h *MockSessionOutputHandler) AddAssistantMessage(text string, thinking string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, "assistant")
	h.AssistantMessages = append(h.AssistantMessages, AssistantMessageRecord{Text: text, Thinking: thinking})
}

// AddToolCall records a tool call.
func (h *MockSessionOutputHandler) AddToolCall(call *tool.ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, "tool_call")
	h.ToolCalls = append(h.ToolCalls, call)
}

// AddToolCallResult records a tool call result.
func (h *MockSessionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, "tool_result")
	h.ToolCallResults = append(h.ToolCallResults, result)
}

// RunFinished records that the session run has finished.
func (h *MockSessionOutputHandler) RunFinished(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Events = append(h.Events, "run_finished")
	h.RunFinishedCalls++
	h.RunFinishedError = err
	if !h.runFinishedClosed {
		close(h.runFinishedCalled)
		h.runFinishedClosed = true
	}
}

// WaitForRunFinished blocks until RunFinished is called.
func (h *MockSessionOutputHandler) WaitForRunFinished() { <-h.runFinishedCalled }

// WaitForFinished blocks until RunFinished is called, failing test on timeout.
func (h *MockSessionOutputHandler) WaitForFinished(t *testing.T) {
	t.Helper()
	select {
	case <-h.runFinishedCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for session finish")
	}
}

// EventsSnapshot returns a copy of recorded event names in observed order.
func (h *MockSessionOutputHandler) EventsSnapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]string, len(h.Events))
	copy(result, h.Events)
    return result
}

// FinishedError returns error passed to RunFinished.
func (h *MockSessionOutputHandler) FinishedError() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.RunFinishedError
}

// Reset clears all recorded output data.
func (h *MockSessionOutputHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.UserMessages = make([]string, 0)
	h.AssistantMessages = make([]AssistantMessageRecord, 0)
	h.Events = make([]string, 0)
	h.ToolCalls = make([]*tool.ToolCall, 0)
	h.ToolCallResults = make([]*tool.ToolResponse, 0)
	h.StatusMessages = make([]SessionMessageRecord, 0)
	h.RateLimitErrors = make([]int, 0)
	h.RunFinishedError = nil
	h.RunFinishedCalls = 0
	h.runFinishedCalled = make(chan struct{})
	h.runFinishedClosed = false
}
