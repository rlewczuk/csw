package mock

import (
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// MockAppView implements ui.IAppView interface for testing purposes.
type MockAppView struct {

	// Recorded calls
	ShowChatCalls     []ui.IChatPresenter
	ShowSettingsCalls int

	// MockChatView to return from ShowChat
	chatView *MockChatView
}

// NewMockAppView creates a new MockAppView instance.
func NewMockAppView() *MockAppView {
	return &MockAppView{
		chatView: NewMockChatView(),
	}
}

// ShowChat shows the chat view.
func (m *MockAppView) ShowChat(presenter ui.IChatPresenter) ui.IChatView {
	m.ShowChatCalls = append(m.ShowChatCalls, presenter)
	return m.chatView
}

// ShowSettings shows the settings view.
func (m *MockAppView) ShowSettings() {
	m.ShowSettingsCalls++
}

// Reset clears all recorded calls.
func (m *MockAppView) Reset() {
	m.ShowChatCalls = nil
	m.ShowSettingsCalls = 0
	m.chatView.Reset()
}

// MockAppPresenter implements ui.IAppPresenter interface for testing purposes.
type MockAppPresenter struct {
	// Configurable errors for each method
	NewSessionErr  error
	OpenSessionErr error
	ExitErr        error

	// Recorded calls
	NewSessionCalls  int
	OpenSessionCalls []string
	ExitCalls        int
}

// NewMockAppPresenter creates a new MockAppPresenter instance.
func NewMockAppPresenter() *MockAppPresenter {
	return &MockAppPresenter{}
}

// NewSession creates a new chat session.
func (m *MockAppPresenter) NewSession() error {
	m.NewSessionCalls++
	return m.NewSessionErr
}

// OpenSession opens an existing chat session.
func (m *MockAppPresenter) OpenSession(id string) error {
	m.OpenSessionCalls = append(m.OpenSessionCalls, id)
	return m.OpenSessionErr
}

// Exit exits the app.
func (m *MockAppPresenter) Exit() error {
	m.ExitCalls++
	return m.ExitErr
}

// Reset clears all recorded calls and errors.
func (m *MockAppPresenter) Reset() {
	m.NewSessionErr = nil
	m.OpenSessionErr = nil
	m.ExitErr = nil

	m.NewSessionCalls = 0
	m.OpenSessionCalls = nil
	m.ExitCalls = 0
}
