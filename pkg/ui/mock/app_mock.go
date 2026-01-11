package mock

import (
	"sync"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// MockAppView implements ui.IAppView interface for testing purposes.
type MockAppView struct {
	mu sync.RWMutex

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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShowChatCalls = append(m.ShowChatCalls, presenter)
	return m.chatView
}

// ShowSettings shows the settings view.
func (m *MockAppView) ShowSettings() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShowSettingsCalls++
}

// Reset clears all recorded calls.
func (m *MockAppView) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShowChatCalls = nil
	m.ShowSettingsCalls = 0
	m.chatView.Reset()
}

// MockAppPresenter implements ui.IAppPresenter interface for testing purposes.
type MockAppPresenter struct {
	mu sync.RWMutex

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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NewSessionCalls++
	return m.NewSessionErr
}

// OpenSession opens an existing chat session.
func (m *MockAppPresenter) OpenSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.OpenSessionCalls = append(m.OpenSessionCalls, id)
	return m.OpenSessionErr
}

// Exit exits the app.
func (m *MockAppPresenter) Exit() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExitCalls++
	return m.ExitErr
}

// Reset clears all recorded calls and errors.
func (m *MockAppPresenter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NewSessionErr = nil
	m.OpenSessionErr = nil
	m.ExitErr = nil

	m.NewSessionCalls = 0
	m.OpenSessionCalls = nil
	m.ExitCalls = 0
}
