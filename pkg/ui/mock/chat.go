package mock

import (
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// MockChatView implements ui.ChatView interface for testing purposes.
type MockChatView struct {
	mu sync.RWMutex

	// Configurable errors for each method
	InitErr          error
	AddMessageErr    error
	UpdateMessageErr error
	UpdateToolErr    error
	MoveToBottomErr  error

	// Recorded calls
	InitCalls          []*ui.ChatSession
	AddMessageCalls    []*ui.ChatMessage
	UpdateMessageCalls []*ui.ChatMessage
	UpdateToolCalls    []*ui.ToolState
	MoveToBottomCalls  int
}

// NewMockChatView creates a new MockChatView instance.
func NewMockChatView() *MockChatView {
	return &MockChatView{}
}

// Init initializes the view with all messages from the session.
func (m *MockChatView) Init(session *ui.ChatSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InitCalls = append(m.InitCalls, session)
	return m.InitErr
}

// AddMessage adds a new message to the view.
func (m *MockChatView) AddMessage(msg *ui.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddMessageCalls = append(m.AddMessageCalls, msg)
	return m.AddMessageErr
}

// UpdateMessage updates an existing message in the view.
func (m *MockChatView) UpdateMessage(msg *ui.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateMessageCalls = append(m.UpdateMessageCalls, msg)
	return m.UpdateMessageErr
}

// UpdateTool updates an existing tool in the view.
func (m *MockChatView) UpdateTool(tool *ui.ToolState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateToolCalls = append(m.UpdateToolCalls, tool)
	return m.UpdateToolErr
}

// MoveToBottom scrolls the view to the bottom.
func (m *MockChatView) MoveToBottom() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MoveToBottomCalls++
	return m.MoveToBottomErr
}

// Reset clears all recorded calls and errors.
func (m *MockChatView) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InitErr = nil
	m.AddMessageErr = nil
	m.UpdateMessageErr = nil
	m.UpdateToolErr = nil
	m.MoveToBottomErr = nil

	m.InitCalls = nil
	m.AddMessageCalls = nil
	m.UpdateMessageCalls = nil
	m.UpdateToolCalls = nil
	m.MoveToBottomCalls = 0
}

// MockChatPresenter implements ui.ChatPresenter interface for testing purposes.
type MockChatPresenter struct {
	mu sync.RWMutex

	// Configurable errors for each method
	SetViewErr         error
	SendUserMessageErr error
	SaveUserMessageErr error
	PauseErr           error
	ResumeErr          error

	// Recorded calls
	SetViewCalls         []ui.ChatView
	SendUserMessageCalls []*ui.ChatMessage
	SaveUserMessageCalls []*ui.ChatMessage
	PauseCalls           int
	ResumeCalls          int
}

// NewMockChatPresenter creates a new MockChatPresenter instance.
func NewMockChatPresenter() *MockChatPresenter {
	return &MockChatPresenter{}
}

// SetView sets the view to render the chat conversation.
func (m *MockChatPresenter) SetView(view ui.ChatView) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetViewCalls = append(m.SetViewCalls, view)
	return m.SetViewErr
}

// SendUserMessage sends a user message to the chat session and starts processing.
func (m *MockChatPresenter) SendUserMessage(message *ui.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SendUserMessageCalls = append(m.SendUserMessageCalls, message)
	return m.SendUserMessageErr
}

// SaveUserMessage saves a user message to the chat session but doesn't start processing.
func (m *MockChatPresenter) SaveUserMessage(message *ui.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveUserMessageCalls = append(m.SaveUserMessageCalls, message)
	return m.SaveUserMessageErr
}

// Pause pauses the chat session (i.e. stops processing).
func (m *MockChatPresenter) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PauseCalls++
	return m.PauseErr
}

// Resume resumes the chat session (i.e. starts processing).
func (m *MockChatPresenter) Resume() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResumeCalls++
	return m.ResumeErr
}

// Reset clears all recorded calls and errors.
func (m *MockChatPresenter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetViewErr = nil
	m.SendUserMessageErr = nil
	m.SaveUserMessageErr = nil
	m.PauseErr = nil
	m.ResumeErr = nil

	m.SetViewCalls = nil
	m.SendUserMessageCalls = nil
	m.SaveUserMessageCalls = nil
	m.PauseCalls = 0
	m.ResumeCalls = 0
}
