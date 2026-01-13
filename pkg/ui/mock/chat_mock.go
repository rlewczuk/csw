package mock

import (
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// MockChatView implements ui.IChatView interface for testing purposes.
type MockChatView struct {
	mu sync.RWMutex

	// Configurable errors for each method
	InitErr            error
	AddMessageErr      error
	UpdateMessageErr   error
	UpdateToolErr      error
	MoveToBottomErr    error
	QueryPermissionErr error

	// Recorded calls
	InitCalls            []*ui.ChatSessionUI
	AddMessageCalls      []*ui.ChatMessageUI
	UpdateMessageCalls   []*ui.ChatMessageUI
	UpdateToolCalls      []*ui.ToolUI
	MoveToBottomCalls    int
	QueryPermissionCalls []*ui.PermissionQueryUI
}

// NewMockChatView creates a new MockChatView instance.
func NewMockChatView() *MockChatView {
	return &MockChatView{}
}

// Init initializes the view with all messages from the session.
func (m *MockChatView) Init(session *ui.ChatSessionUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InitCalls = append(m.InitCalls, session)
	return m.InitErr
}

// AddMessage adds a new message to the view.
func (m *MockChatView) AddMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddMessageCalls = append(m.AddMessageCalls, msg)
	return m.AddMessageErr
}

// UpdateMessage updates an existing message in the view.
func (m *MockChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateMessageCalls = append(m.UpdateMessageCalls, msg)
	return m.UpdateMessageErr
}

// UpdateTool updates an existing tool in the view.
func (m *MockChatView) UpdateTool(tool *ui.ToolUI) error {
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

// QueryPermission queries user for permission to use a tool.
func (m *MockChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.QueryPermissionCalls = append(m.QueryPermissionCalls, query)
	return m.QueryPermissionErr
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
	m.QueryPermissionErr = nil

	m.InitCalls = nil
	m.AddMessageCalls = nil
	m.UpdateMessageCalls = nil
	m.UpdateToolCalls = nil
	m.MoveToBottomCalls = 0
	m.QueryPermissionCalls = nil
}

// MockChatPresenter implements ui.IChatPresenter interface for testing purposes.
type MockChatPresenter struct {
	mu sync.RWMutex

	// Configurable errors for each method
	SetViewErr            error
	SendUserMessageErr    error
	SaveUserMessageErr    error
	PauseErr              error
	ResumeErr             error
	PermissionResponseErr error

	// Recorded calls
	SetViewCalls            []ui.IChatView
	SendUserMessageCalls    []*ui.ChatMessageUI
	SaveUserMessageCalls    []*ui.ChatMessageUI
	PauseCalls              int
	ResumeCalls             int
	PermissionResponseCalls []string
}

// NewMockChatPresenter creates a new MockChatPresenter instance.
func NewMockChatPresenter() *MockChatPresenter {
	return &MockChatPresenter{}
}

// SetView sets the view to render the chat conversation.
func (m *MockChatPresenter) SetView(view ui.IChatView) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetViewCalls = append(m.SetViewCalls, view)
	return m.SetViewErr
}

// SendUserMessage sends a user message to the chat session and starts processing.
func (m *MockChatPresenter) SendUserMessage(message *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SendUserMessageCalls = append(m.SendUserMessageCalls, message)
	return m.SendUserMessageErr
}

// SaveUserMessage saves a user message to the chat session but doesn't start processing.
func (m *MockChatPresenter) SaveUserMessage(message *ui.ChatMessageUI) error {
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

// PermissionResponse sends user response to permission query.
func (m *MockChatPresenter) PermissionResponse(response string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PermissionResponseCalls = append(m.PermissionResponseCalls, response)
	return m.PermissionResponseErr
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
	m.PermissionResponseErr = nil

	m.SetViewCalls = nil
	m.SendUserMessageCalls = nil
	m.SaveUserMessageCalls = nil
	m.PauseCalls = 0
	m.ResumeCalls = 0
	m.PermissionResponseCalls = nil
}
