package mock

import (
	"errors"
	"testing"

	"github.com/rlewczuk/csw/pkg/presenter"
	"github.com/rlewczuk/csw/pkg/shared"
	"github.com/rlewczuk/csw/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPermissionResponder struct {
	err   error
	calls []string
}

func (m *mockPermissionResponder) PermissionResponse(response string) error {
	m.calls = append(m.calls, response)
	return m.err
}

// TestMockChatViewAutoPermissionResponse tests the MockChatView's automatic permission response functionality.
func TestMockChatViewAutoPermissionResponse(t *testing.T) {
	tests := []struct {
		name             string
		autoResponse     string
		options          []string
		expectedResponse string
	}{
		{
			name:             "Auto deny selects Deny option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Deny",
		},
		{
			name:             "Auto deny falls back to last option",
			autoResponse:     "Deny",
			options:          []string{"Allow", "Ask", "Reject"},
			expectedResponse: "Reject",
		},
		{
			name:             "Auto accept selects first option",
			autoResponse:     "Accept",
			options:          []string{"Allow", "Ask", "Deny"},
			expectedResponse: "Allow",
		},
		{
			name:             "Custom response",
			autoResponse:     "CustomAnswer",
			options:          []string{"Option1", "Option2"},
			expectedResponse: "CustomAnswer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockPresenter := &mockPermissionResponder{}

			mockView := NewMockChatView()
			mockView.AutoPermissionResponse = tc.autoResponse
			mockView.Presenter = mockPresenter

			query := &ui.PermissionQueryUI{
				Id:      "test-query-1",
				Title:   "Test Permission",
				Details: "Test details",
				Options: tc.options,
			}

			err := mockView.QueryPermission(query)
			require.NoError(t, err)

			assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")
			assert.Equal(t, 1, len(mockPresenter.calls), "Permission response should have been sent")
			assert.Equal(t, tc.expectedResponse, mockPresenter.calls[0], "Response should match expected")
		})
	}
}

// TestMockChatViewNoAutoResponse tests that MockChatView without auto-response just records the query.
func TestMockChatViewNoAutoResponse(t *testing.T) {
	mockPresenter := &mockPermissionResponder{}

	mockView := NewMockChatView()
	mockView.Presenter = mockPresenter

	query := &ui.PermissionQueryUI{
		Id:      "test-query-1",
		Title:   "Test Permission",
		Details: "Test details",
		Options: []string{"Allow", "Deny"},
	}

	err := mockView.QueryPermission(query)
	require.NoError(t, err)

	assert.Equal(t, 1, len(mockView.QueryPermissionCalls), "Query should have been recorded")
	assert.Equal(t, 0, len(mockPresenter.calls), "No permission response should have been sent without auto-response")
}

func TestMockChatView_Init(t *testing.T) {
	view := NewMockChatView()

	session := &ui.ChatSessionUI{
		Id:      "session-123",
		Model:   "gpt-4",
		Role:    "assistant",
		WorkDir: "/tmp",
		Messages: []*ui.ChatMessageUI{
			{Id: "msg-1", Role: ui.ChatRoleUser, Text: "Hello"},
		},
	}

	err := view.Init(session)
	if err != nil {
		t.Errorf("Init() returned unexpected error: %v", err)
	}

	if len(view.InitCalls) != 1 {
		t.Fatalf("expected 1 Init call, got %d", len(view.InitCalls))
	}
	if view.InitCalls[0] != session {
		t.Errorf("Init() did not record correct session")
	}
}

func TestMockChatView_Init_WithError(t *testing.T) {
	view := NewMockChatView()
	expectedErr := errors.New("init error")
	view.InitErr = expectedErr

	err := view.Init(&ui.ChatSessionUI{})
	if err != expectedErr {
		t.Errorf("Init() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatView_AddMessage(t *testing.T) {
	view := NewMockChatView()

	msg1 := &ui.ChatMessageUI{Id: "msg-1", Role: ui.ChatRoleUser, Text: "Hello"}
	msg2 := &ui.ChatMessageUI{Id: "msg-2", Role: ui.ChatRoleAssistant, Text: "Hi there"}

	if err := view.AddMessage(msg1); err != nil {
		t.Errorf("AddMessage() returned unexpected error: %v", err)
	}
	if err := view.AddMessage(msg2); err != nil {
		t.Errorf("AddMessage() returned unexpected error: %v", err)
	}

	if len(view.AddMessageCalls) != 2 {
		t.Fatalf("expected 2 AddMessage calls, got %d", len(view.AddMessageCalls))
	}
	if view.AddMessageCalls[0] != msg1 {
		t.Errorf("AddMessage() did not record first message correctly")
	}
	if view.AddMessageCalls[1] != msg2 {
		t.Errorf("AddMessage() did not record second message correctly")
	}
}

func TestMockChatView_AddMessage_WithError(t *testing.T) {
	view := NewMockChatView()
	expectedErr := errors.New("add message error")
	view.AddMessageErr = expectedErr

	err := view.AddMessage(&ui.ChatMessageUI{})
	if err != expectedErr {
		t.Errorf("AddMessage() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatView_UpdateMessage(t *testing.T) {
	view := NewMockChatView()

	msg := &ui.ChatMessageUI{Id: "msg-1", Role: ui.ChatRoleAssistant, Text: "Updated text"}

	if err := view.UpdateMessage(msg); err != nil {
		t.Errorf("UpdateMessage() returned unexpected error: %v", err)
	}

	if len(view.UpdateMessageCalls) != 1 {
		t.Fatalf("expected 1 UpdateMessage call, got %d", len(view.UpdateMessageCalls))
	}
	if view.UpdateMessageCalls[0] != msg {
		t.Errorf("UpdateMessage() did not record message correctly")
	}
	if view.UpdateMessageCalls[0].Text != "Updated text" {
		t.Errorf("UpdateMessage() did not preserve message Title field")
	}
}

func TestMockChatView_UpdateMessage_WithError(t *testing.T) {
	view := NewMockChatView()
	expectedErr := errors.New("update message error")
	view.UpdateMessageErr = expectedErr

	err := view.UpdateMessage(&ui.ChatMessageUI{})
	if err != expectedErr {
		t.Errorf("UpdateMessage() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatView_UpdateTool(t *testing.T) {
	view := NewMockChatView()

	tool := &ui.ToolUI{
		Id:      "tool-1",
		Status:  ui.ToolStatusExecuting,
		Name:    "bash",
		Message: "Running command",
		Props:   [][]string{{"cmd", "ls -la"}},
	}

	if err := view.UpdateTool(tool); err != nil {
		t.Errorf("UpdateTool() returned unexpected error: %v", err)
	}

	if len(view.UpdateToolCalls) != 1 {
		t.Fatalf("expected 1 UpdateTool call, got %d", len(view.UpdateToolCalls))
	}
	if view.UpdateToolCalls[0] != tool {
		t.Errorf("UpdateTool() did not record tool correctly")
	}
	if view.UpdateToolCalls[0].Name != "bash" {
		t.Errorf("UpdateTool() did not preserve tool Name field")
	}
	if view.UpdateToolCalls[0].Status != ui.ToolStatusExecuting {
		t.Errorf("UpdateTool() did not preserve tool Status field")
	}
}

func TestMockChatView_UpdateTool_WithError(t *testing.T) {
	view := NewMockChatView()
	expectedErr := errors.New("update tool error")
	view.UpdateToolErr = expectedErr

	err := view.UpdateTool(&ui.ToolUI{})
	if err != expectedErr {
		t.Errorf("UpdateTool() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatView_MoveToBottom(t *testing.T) {
	view := NewMockChatView()

	for i := 0; i < 3; i++ {
		if err := view.MoveToBottom(); err != nil {
			t.Errorf("MoveToBottom() returned unexpected error: %v", err)
		}
	}

	if view.MoveToBottomCalls != 3 {
		t.Errorf("expected 3 MoveToBottom calls, got %d", view.MoveToBottomCalls)
	}
}

func TestMockChatView_MoveToBottom_WithError(t *testing.T) {
	view := NewMockChatView()
	expectedErr := errors.New("move to bottom error")
	view.MoveToBottomErr = expectedErr

	err := view.MoveToBottom()
	if err != expectedErr {
		t.Errorf("MoveToBottom() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatView_Reset(t *testing.T) {
	view := NewMockChatView()

	// Set up some state
	view.InitErr = errors.New("some error")
	view.Init(&ui.ChatSessionUI{Id: "session-1"})
	view.AddMessage(&ui.ChatMessageUI{Id: "msg-1"})
	view.UpdateMessage(&ui.ChatMessageUI{Id: "msg-2"})
	view.UpdateTool(&ui.ToolUI{Id: "tool-1"})
	view.MoveToBottom()
	view.ShowMessage("status", shared.MessageTypeWarning)

	// Reset
	view.Reset()

	// Verify everything is cleared
	if view.InitErr != nil {
		t.Errorf("Reset() did not clear InitErr")
	}
	if len(view.InitCalls) != 0 {
		t.Errorf("Reset() did not clear InitCalls")
	}
	if len(view.AddMessageCalls) != 0 {
		t.Errorf("Reset() did not clear AddMessageCalls")
	}
	if len(view.UpdateMessageCalls) != 0 {
		t.Errorf("Reset() did not clear UpdateMessageCalls")
	}
	if len(view.UpdateToolCalls) != 0 {
		t.Errorf("Reset() did not clear UpdateToolCalls")
	}
	if view.MoveToBottomCalls != 0 {
		t.Errorf("Reset() did not clear MoveToBottomCalls")
	}
	if len(view.ShowMessageCalls) != 0 {
		t.Errorf("Reset() did not clear ShowMessageCalls")
	}
}

func TestMockChatView_ShowMessage(t *testing.T) {
	view := NewMockChatView()

	view.ShowMessage("hello", shared.MessageTypeInfo)
	view.ShowMessage("careful", shared.MessageTypeWarning)

	require.Len(t, view.ShowMessageCalls, 2)
	assert.Equal(t, "hello", view.ShowMessageCalls[0].Message)
	assert.Equal(t, shared.MessageTypeInfo, view.ShowMessageCalls[0].Type)
	assert.Equal(t, "careful", view.ShowMessageCalls[1].Message)
	assert.Equal(t, shared.MessageTypeWarning, view.ShowMessageCalls[1].Type)
}

func TestMockChatView_ImplementsPresenterChatView(t *testing.T) {
	var _ presenter.ChatView = (*MockChatView)(nil)
}
