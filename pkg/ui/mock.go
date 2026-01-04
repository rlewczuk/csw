package ui

import (
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
}

// NewMockSessionOutputHandler creates a new MockSessionOutputHandler.
func NewMockSessionOutputHandler() *MockSessionOutputHandler {
	return &MockSessionOutputHandler{
		MarkdownChunks:  make([]string, 0),
		ToolCallStarts:  make([]*tool.ToolCall, 0),
		ToolCallDetails: make([]*tool.ToolCall, 0),
		ToolCallResults: make([]*tool.ToolResponse, 0),
	}
}

// AddMarkdownChunk records a markdown chunk.
func (h *MockSessionOutputHandler) AddMarkdownChunk(markdown string) {
	h.MarkdownChunks = append(h.MarkdownChunks, markdown)
}

// AddToolCallStart records a tool call start event.
func (h *MockSessionOutputHandler) AddToolCallStart(call *tool.ToolCall) {
	h.ToolCallStarts = append(h.ToolCallStarts, call)
}

// AddToolCallDetails records tool call details.
func (h *MockSessionOutputHandler) AddToolCallDetails(call *tool.ToolCall) {
	h.ToolCallDetails = append(h.ToolCallDetails, call)
}

// AddToolCallResult records a tool call result.
func (h *MockSessionOutputHandler) AddToolCallResult(result *tool.ToolResponse) {
	h.ToolCallResults = append(h.ToolCallResults, result)
}

// Reset clears all recorded output data.
func (h *MockSessionOutputHandler) Reset() {
	h.MarkdownChunks = make([]string, 0)
	h.ToolCallStarts = make([]*tool.ToolCall, 0)
	h.ToolCallDetails = make([]*tool.ToolCall, 0)
	h.ToolCallResults = make([]*tool.ToolResponse, 0)
}

// MockUiFactory is a mock implementation of SweUiFactory.
// It creates MockSessionOutputHandler instances for sessions.
type MockUiFactory struct {
	// Handlers stores all created handlers for inspection in tests.
	Handlers []*MockSessionOutputHandler
}

// NewMockUiFactory creates a new MockUiFactory.
func NewMockUiFactory() *MockUiFactory {
	return &MockUiFactory{
		Handlers: make([]*MockSessionOutputHandler, 0),
	}
}

// NewSessionOutputHandler creates a new MockSessionOutputHandler and stores it for later inspection.
func (f *MockUiFactory) NewSessionOutputHandler() SessionOutputHandler {
	handler := NewMockSessionOutputHandler()
	f.Handlers = append(f.Handlers, handler)
	return handler
}

// Reset clears the list of created handlers.
func (f *MockUiFactory) Reset() {
	f.Handlers = make([]*MockSessionOutputHandler, 0)
}
