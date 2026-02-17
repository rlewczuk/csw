package main

import (
	"sync"
	"testing"

	coretestfixture "github.com/codesnort/codesnort-swe/pkg/core/testfixture"
	"github.com/codesnort/codesnort-swe/pkg/ui"
)

// cli_integ_test.go contains shared test fixtures and mock implementations
// used by the CLI integration test suite.

// mockChatView is a mock implementation of ui.IChatView for testing
type mockChatView struct {
	mu         sync.Mutex
	messages   []*ui.ChatMessageUI
	initCalled bool
}

func newMockChatView() *mockChatView {
	return &mockChatView{
		messages: make([]*ui.ChatMessageUI, 0),
	}
}

func (m *mockChatView) Init(session *ui.ChatSessionUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initCalled = true
	m.messages = append(m.messages, session.Messages...)
	return nil
}

func (m *mockChatView) AddMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockChatView) UpdateMessage(msg *ui.ChatMessageUI) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.messages {
		if existing.Id == msg.Id {
			m.messages[i] = msg
			return nil
		}
	}
	return nil
}

func (m *mockChatView) UpdateTool(tool *ui.ToolUI) error {
	return nil
}

func (m *mockChatView) MoveToBottom() error {
	return nil
}

func (m *mockChatView) QueryPermission(query *ui.PermissionQueryUI) error {
	return nil
}

func (m *mockChatView) GetMessages() []*ui.ChatMessageUI {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*ui.ChatMessageUI, len(m.messages))
	copy(result, m.messages)
	return result
}

// newCliSystemFixture creates a new SweSystemFixture for CLI integration tests
func newCliSystemFixture(t *testing.T, prompt string, opts ...coretestfixture.SweSystemFixtureOption) *coretestfixture.SweSystemFixture {
	base := []coretestfixture.SweSystemFixtureOption{
		coretestfixture.WithPromptGenerator(coretestfixture.NewStaticPromptGenerator(prompt)),
	}
	return coretestfixture.NewSweSystemFixture(t, append(base, opts...)...)
}
