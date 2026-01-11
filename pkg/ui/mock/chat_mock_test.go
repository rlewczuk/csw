package mock

import (
	"errors"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/ui"
)

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
		t.Errorf("UpdateMessage() did not preserve message Text field")
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
}

func TestMockChatPresenter_SetView(t *testing.T) {
	presenter := NewMockChatPresenter()
	view := NewMockChatView()

	if err := presenter.SetView(view); err != nil {
		t.Errorf("SetView() returned unexpected error: %v", err)
	}

	if len(presenter.SetViewCalls) != 1 {
		t.Fatalf("expected 1 SetView call, got %d", len(presenter.SetViewCalls))
	}
	if presenter.SetViewCalls[0] != view {
		t.Errorf("SetView() did not record view correctly")
	}
}

func TestMockChatPresenter_SetView_WithError(t *testing.T) {
	presenter := NewMockChatPresenter()
	expectedErr := errors.New("set view error")
	presenter.SetViewErr = expectedErr

	err := presenter.SetView(NewMockChatView())
	if err != expectedErr {
		t.Errorf("SetView() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatPresenter_SendUserMessage(t *testing.T) {
	presenter := NewMockChatPresenter()

	msg := &ui.ChatMessageUI{
		Id:   "msg-1",
		Role: ui.ChatRoleUser,
		Text: "Hello, assistant!",
	}

	if err := presenter.SendUserMessage(msg); err != nil {
		t.Errorf("SendUserMessage() returned unexpected error: %v", err)
	}

	if len(presenter.SendUserMessageCalls) != 1 {
		t.Fatalf("expected 1 SendUserMessage call, got %d", len(presenter.SendUserMessageCalls))
	}
	if presenter.SendUserMessageCalls[0] != msg {
		t.Errorf("SendUserMessage() did not record message correctly")
	}
	if presenter.SendUserMessageCalls[0].Text != "Hello, assistant!" {
		t.Errorf("SendUserMessage() did not preserve message Text field")
	}
}

func TestMockChatPresenter_SendUserMessage_WithError(t *testing.T) {
	presenter := NewMockChatPresenter()
	expectedErr := errors.New("send user message error")
	presenter.SendUserMessageErr = expectedErr

	err := presenter.SendUserMessage(&ui.ChatMessageUI{})
	if err != expectedErr {
		t.Errorf("SendUserMessage() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatPresenter_SaveUserMessage(t *testing.T) {
	presenter := NewMockChatPresenter()

	msg := &ui.ChatMessageUI{
		Id:   "msg-1",
		Role: ui.ChatRoleUser,
		Text: "Draft message",
	}

	if err := presenter.SaveUserMessage(msg); err != nil {
		t.Errorf("SaveUserMessage() returned unexpected error: %v", err)
	}

	if len(presenter.SaveUserMessageCalls) != 1 {
		t.Fatalf("expected 1 SaveUserMessage call, got %d", len(presenter.SaveUserMessageCalls))
	}
	if presenter.SaveUserMessageCalls[0] != msg {
		t.Errorf("SaveUserMessage() did not record message correctly")
	}
	if presenter.SaveUserMessageCalls[0].Text != "Draft message" {
		t.Errorf("SaveUserMessage() did not preserve message Text field")
	}
}

func TestMockChatPresenter_SaveUserMessage_WithError(t *testing.T) {
	presenter := NewMockChatPresenter()
	expectedErr := errors.New("save user message error")
	presenter.SaveUserMessageErr = expectedErr

	err := presenter.SaveUserMessage(&ui.ChatMessageUI{})
	if err != expectedErr {
		t.Errorf("SaveUserMessage() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatPresenter_Pause(t *testing.T) {
	presenter := NewMockChatPresenter()

	for i := 0; i < 2; i++ {
		if err := presenter.Pause(); err != nil {
			t.Errorf("Pause() returned unexpected error: %v", err)
		}
	}

	if presenter.PauseCalls != 2 {
		t.Errorf("expected 2 Pause calls, got %d", presenter.PauseCalls)
	}
}

func TestMockChatPresenter_Pause_WithError(t *testing.T) {
	presenter := NewMockChatPresenter()
	expectedErr := errors.New("pause error")
	presenter.PauseErr = expectedErr

	err := presenter.Pause()
	if err != expectedErr {
		t.Errorf("Pause() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatPresenter_Resume(t *testing.T) {
	presenter := NewMockChatPresenter()

	for i := 0; i < 4; i++ {
		if err := presenter.Resume(); err != nil {
			t.Errorf("Resume() returned unexpected error: %v", err)
		}
	}

	if presenter.ResumeCalls != 4 {
		t.Errorf("expected 4 Resume calls, got %d", presenter.ResumeCalls)
	}
}

func TestMockChatPresenter_Resume_WithError(t *testing.T) {
	presenter := NewMockChatPresenter()
	expectedErr := errors.New("resume error")
	presenter.ResumeErr = expectedErr

	err := presenter.Resume()
	if err != expectedErr {
		t.Errorf("Resume() expected error %v, got %v", expectedErr, err)
	}
}

func TestMockChatPresenter_Reset(t *testing.T) {
	presenter := NewMockChatPresenter()

	// Set up some state
	presenter.SetViewErr = errors.New("some error")
	presenter.SetView(NewMockChatView())
	presenter.SendUserMessage(&ui.ChatMessageUI{Id: "msg-1"})
	presenter.SaveUserMessage(&ui.ChatMessageUI{Id: "msg-2"})
	presenter.Pause()
	presenter.Resume()

	// Reset
	presenter.Reset()

	// Verify everything is cleared
	if presenter.SetViewErr != nil {
		t.Errorf("Reset() did not clear SetViewErr")
	}
	if len(presenter.SetViewCalls) != 0 {
		t.Errorf("Reset() did not clear SetViewCalls")
	}
	if len(presenter.SendUserMessageCalls) != 0 {
		t.Errorf("Reset() did not clear SendUserMessageCalls")
	}
	if len(presenter.SaveUserMessageCalls) != 0 {
		t.Errorf("Reset() did not clear SaveUserMessageCalls")
	}
	if presenter.PauseCalls != 0 {
		t.Errorf("Reset() did not clear PauseCalls")
	}
	if presenter.ResumeCalls != 0 {
		t.Errorf("Reset() did not clear ResumeCalls")
	}
}

func TestMockChatView_ImplementsChatViewInterface(t *testing.T) {
	var _ ui.ChatView = (*MockChatView)(nil)
}

func TestMockChatPresenter_ImplementsChatPresenterInterface(t *testing.T) {
	var _ ui.ChatPresenter = (*MockChatPresenter)(nil)
}
