package mock

import (
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/ui"
)

// MockChatView implements ui.IChatView interface for testing purposes.
type MockChatView struct {
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
	ShowMessageCalls     []MockChatMessageCall

	// Automatic permission response configuration
	// When set, QueryPermission will automatically respond with this value
	// instead of recording the call and returning. Use "Accept" to accept
	// the first option, "Deny" to deny (select last option or "Deny" option),
	// or any other string to respond with that value.
	AutoPermissionResponse string

	// Presenter for sending automatic permission responses
	Presenter ui.IChatPresenter
}

// MockChatMessageCall stores one ShowMessage invocation.
type MockChatMessageCall struct {
	Message string
	Type    shared.MessageType
}

// NewMockChatView creates a new MockChatView instance.
func NewMockChatView() *MockChatView {
	return &MockChatView{}
}

// Init initializes the view with all messages from the session.
func (m *MockChatView) Init(session *ui.ChatSessionUI) error {
	m.InitCalls = append(m.InitCalls, session)
	return m.InitErr
}

// AddMessage adds a new message to the view.
func (m *MockChatView) AddMessage(msg *ui.ChatMessageUI) error {
	m.AddMessageCalls = append(m.AddMessageCalls, msg)
	return m.AddMessageErr
}

// UpdateMessage updates an existing message in the view.
func (m *MockChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	m.UpdateMessageCalls = append(m.UpdateMessageCalls, msg)
	return m.UpdateMessageErr
}

// UpdateTool updates an existing tool in the view.
func (m *MockChatView) UpdateTool(tool *ui.ToolUI) error {
	m.UpdateToolCalls = append(m.UpdateToolCalls, tool)
	return m.UpdateToolErr
}

// MoveToBottom scrolls the view to the bottom.
func (m *MockChatView) MoveToBottom() error {
	m.MoveToBottomCalls++
	return m.MoveToBottomErr
}

// QueryPermission queries user for permission to use a tool.
func (m *MockChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	m.QueryPermissionCalls = append(m.QueryPermissionCalls, query)

	// Handle automatic permission response if configured
	if m.AutoPermissionResponse != "" && m.Presenter != nil && len(query.Options) > 0 {
		var response string
		switch m.AutoPermissionResponse {
		case "Accept":
			// Accept the first option
			response = query.Options[0]
		case "Deny":
			// Find the "Deny" option or use the last option
			response = query.Options[len(query.Options)-1]
			for _, opt := range query.Options {
				if opt == "Deny" {
					response = opt
					break
				}
			}
		default:
			// Use the configured response directly
			response = m.AutoPermissionResponse
		}
		return m.Presenter.PermissionResponse(response)
	}

	return m.QueryPermissionErr
}

// ShowMessage stores a user-facing status message call.
func (m *MockChatView) ShowMessage(message string, messageType shared.MessageType) {
	m.ShowMessageCalls = append(m.ShowMessageCalls, MockChatMessageCall{
		Message: message,
		Type:    messageType,
	})
}

// Reset clears all recorded calls and errors.
func (m *MockChatView) Reset() {
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
	m.ShowMessageCalls = nil

	m.AutoPermissionResponse = ""
	m.Presenter = nil
}

// MockChatPresenter implements ui.IChatPresenter interface for testing purposes.
type MockChatPresenter struct {
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
	m.SetViewCalls = append(m.SetViewCalls, view)
	return m.SetViewErr
}

// SendUserMessage sends a user message to the chat session and starts processing.
func (m *MockChatPresenter) SendUserMessage(message *ui.ChatMessageUI) error {
	m.SendUserMessageCalls = append(m.SendUserMessageCalls, message)
	return m.SendUserMessageErr
}

// SaveUserMessage saves a user message to the chat session but doesn't start processing.
func (m *MockChatPresenter) SaveUserMessage(message *ui.ChatMessageUI) error {
	m.SaveUserMessageCalls = append(m.SaveUserMessageCalls, message)
	return m.SaveUserMessageErr
}

// Pause pauses the chat session (i.e. stops processing).
func (m *MockChatPresenter) Pause() error {
	m.PauseCalls++
	return m.PauseErr
}

// Resume resumes the chat session (i.e. starts processing).
func (m *MockChatPresenter) Resume() error {
	m.ResumeCalls++
	return m.ResumeErr
}

// PermissionResponse sends user response to permission query.
func (m *MockChatPresenter) PermissionResponse(response string) error {
	m.PermissionResponseCalls = append(m.PermissionResponseCalls, response)
	return m.PermissionResponseErr
}

// SetModel sets the model used for the chat session.
func (m *MockChatPresenter) SetModel(model string) error {
	return nil
}

// Reset clears all recorded calls and errors.
func (m *MockChatPresenter) Reset() {
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
