package testutil

import (
	"sync"

	"github.com/rlewczuk/csw/pkg/tool"
)

// AssistantMessageRecord stores assistant output captured from session output handler.
type AssistantMessageRecord struct {
	Text     string
	Thinking string
}

// MockSessionOutputHandler is a mock implementation of SessionOutputHandler that keeps all output in memory.
// It is used for testing and capturing output from agent operations.
type MockSessionOutputHandler struct {
	// AssistantMessages stores assistant outputs received via AddAssistantMessage.
	AssistantMessages []AssistantMessageRecord

	// ToolCalls stores all tool calls received via AddToolCall.
	ToolCalls []*tool.ToolCall

	// ToolCallResults stores all tool responses received via AddToolCallResult.
	ToolCallResults []*tool.ToolResponse

	// PermissionQueries stores all permission queries received via OnPermissionQuery.
	PermissionQueries     []*tool.ToolPermissionsQuery
	permissionQueryCalled chan struct{}

	// RateLimitErrors stores all rate limit errors received via OnRateLimitError.
	RateLimitErrors      []int
	rateLimitErrorCalled chan struct{}

	// RunFinishedError stores the error from RunFinished call.
	RunFinishedError  error
	runFinishedCalled chan struct{}

	mu sync.Mutex
}

// NewMockSessionOutputHandler creates a new MockSessionOutputHandler.
func NewMockSessionOutputHandler() *MockSessionOutputHandler {
	return &MockSessionOutputHandler{
		AssistantMessages:     make([]AssistantMessageRecord, 0),
		ToolCalls:             make([]*tool.ToolCall, 0),
		ToolCallResults:       make([]*tool.ToolResponse, 0),
		PermissionQueries:     make([]*tool.ToolPermissionsQuery, 0),
		permissionQueryCalled: make(chan struct{}, 10),
		rateLimitErrorCalled:  make(chan struct{}, 10),
		runFinishedCalled:     make(chan struct{}),
	}
}

// OnPermissionQuery records a permission query.
func (h *MockSessionOutputHandler) OnPermissionQuery(query *tool.ToolPermissionsQuery) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.PermissionQueries = append(h.PermissionQueries, query)
	select {
	case h.permissionQueryCalled <- struct{}{}:
	default:
	}
}

// WaitForPermissionQuery blocks until OnPermissionQuery is called.
func (h *MockSessionOutputHandler) WaitForPermissionQuery() { <-h.permissionQueryCalled }

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

// ShowMessage records status message (ignored in this mock).
func (h *MockSessionOutputHandler) ShowMessage(message string, messageType string) {
	_ = message
	_ = messageType
}

// AddUserMessage records user message (ignored in this mock).
func (h *MockSessionOutputHandler) AddUserMessage(text string) {
	_ = text
}

// ShouldRetryAfterFailure returns false in test mock unless explicitly handled by tests.
func (h *MockSessionOutputHandler) ShouldRetryAfterFailure(message string) bool {
	_ = message
	return false
}

// WaitForRateLimitError blocks until OnRateLimitError is called.
func (h *MockSessionOutputHandler) WaitForRateLimitError() { <-h.rateLimitErrorCalled }

// AddAssistantMessage records a full assistant output.
func (h *MockSessionOutputHandler) AddAssistantMessage(text string, thinking string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.AssistantMessages = append(h.AssistantMessages, AssistantMessageRecord{Text: text, Thinking: thinking})
}

// AddToolCall records a tool call.
func (h *MockSessionOutputHandler) AddToolCall(call *tool.ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCalls = append(h.ToolCalls, call)
}

// AddToolCallResult records a tool call result.
func (h *MockSessionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCallResults = append(h.ToolCallResults, result)
}

// RunFinished records that the session run has finished.
func (h *MockSessionOutputHandler) RunFinished(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.RunFinishedError = err
	close(h.runFinishedCalled)
}

// WaitForRunFinished blocks until RunFinished is called.
func (h *MockSessionOutputHandler) WaitForRunFinished() { <-h.runFinishedCalled }

// Reset clears all recorded output data.
func (h *MockSessionOutputHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.AssistantMessages = make([]AssistantMessageRecord, 0)
	h.ToolCalls = make([]*tool.ToolCall, 0)
	h.ToolCallResults = make([]*tool.ToolResponse, 0)
	h.RateLimitErrors = make([]int, 0)
	h.RunFinishedError = nil
	h.runFinishedCalled = make(chan struct{})
}
