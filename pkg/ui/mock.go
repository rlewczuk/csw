package ui

import (
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/tool"
)

// MockSessionOutputHandler is a mock implementation of SessionOutputHandler that keeps all output in memory.
// It is used for testing and capturing output from agent operations.
type MockSessionOutputHandler struct {
	// MarkdownChunks stores all markdown chunks received via AddMarkdownChunk.
	MarkdownChunks []string

	// ToolCallStarts stores all tool calls received via AddToolCallStart.
	ToolCallStarts []*tool.ToolCall

	// ToolCallDetails stores all tool calls received via AddToolCallDetails.
	// Multiple entries may exist for the same tool call as it's being parsed.
	ToolCallDetails []*tool.ToolCall

	// ToolCallResults stores all tool responses received via AddToolCallResult.
	ToolCallResults []*tool.ToolResponse

	// RunFinishedError stores the error from RunFinished call.
	RunFinishedError error

	// runFinishedCalled is a channel that is closed when RunFinished is called.
	runFinishedCalled chan struct{}

	// mu protects access to fields.
	mu sync.Mutex
}

// NewMockSessionOutputHandler creates a new MockSessionOutputHandler.
func NewMockSessionOutputHandler() *MockSessionOutputHandler {
	return &MockSessionOutputHandler{
		MarkdownChunks:    make([]string, 0),
		ToolCallStarts:    make([]*tool.ToolCall, 0),
		ToolCallDetails:   make([]*tool.ToolCall, 0),
		ToolCallResults:   make([]*tool.ToolResponse, 0),
		runFinishedCalled: make(chan struct{}),
	}
}

// AddMarkdownChunk records a markdown chunk.
func (h *MockSessionOutputHandler) AddMarkdownChunk(markdown string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.MarkdownChunks = append(h.MarkdownChunks, markdown)
}

// AddToolCallStart records a tool call start event.
func (h *MockSessionOutputHandler) AddToolCallStart(call *tool.ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCallStarts = append(h.ToolCallStarts, call)
}

// AddToolCallDetails records tool call details.
func (h *MockSessionOutputHandler) AddToolCallDetails(call *tool.ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCallDetails = append(h.ToolCallDetails, call)
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
func (h *MockSessionOutputHandler) WaitForRunFinished() {
	<-h.runFinishedCalled
}

// Reset clears all recorded output data.
func (h *MockSessionOutputHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.MarkdownChunks = make([]string, 0)
	h.ToolCallStarts = make([]*tool.ToolCall, 0)
	h.ToolCallDetails = make([]*tool.ToolCall, 0)
	h.ToolCallResults = make([]*tool.ToolResponse, 0)
	h.RunFinishedError = nil
	h.runFinishedCalled = make(chan struct{})
}
