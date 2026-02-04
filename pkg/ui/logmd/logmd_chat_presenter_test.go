package logmd

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/ui"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogmdChatPresenter(t *testing.T) {
	t.Run("creates new LogmdChatPresenter with correct dependencies", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}

		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		assert.NotNil(t, presenter)
		assert.Implements(t, (*ui.IChatPresenter)(nil), presenter)
	})
}

func TestLogmdChatPresenter_SetView(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		view := mock.NewMockChatView()

		err := presenter.SetView(view)

		require.NoError(t, err)
		assert.Len(t, wrapped.SetViewCalls, 1)
		assert.Equal(t, view, wrapped.SetViewCalls[0])
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.SetViewErr = errors.New("set view error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		view := mock.NewMockChatView()

		err := presenter.SetView(view)

		assert.Error(t, err)
		assert.Equal(t, "set view error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		view := mock.NewMockChatView()

		err := presenter.SetView(view)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `SetView`")
		assert.Contains(t, output, "**Parameters:**")
	})
}

func TestLogmdChatPresenter_SendUserMessage(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := presenter.SendUserMessage(msg)

		require.NoError(t, err)
		assert.Len(t, wrapped.SendUserMessageCalls, 1)
		assert.Equal(t, msg, wrapped.SendUserMessageCalls[0])
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.SendUserMessageErr = errors.New("send error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := presenter.SendUserMessage(msg)

		assert.Error(t, err)
		assert.Equal(t, "send error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello, world!",
		}

		err := presenter.SendUserMessage(msg)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `SendUserMessage`")
		assert.Contains(t, output, "id=msg-1")
		assert.Contains(t, output, "role=user")
		assert.Contains(t, output, "text=\"Hello, world!\"")
	})
}

func TestLogmdChatPresenter_SaveUserMessage(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := presenter.SaveUserMessage(msg)

		require.NoError(t, err)
		assert.Len(t, wrapped.SaveUserMessageCalls, 1)
		assert.Equal(t, msg, wrapped.SaveUserMessageCalls[0])
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.SaveUserMessageErr = errors.New("save error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello",
		}

		err := presenter.SaveUserMessage(msg)

		assert.Error(t, err)
		assert.Equal(t, "save error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		msg := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Save this message",
		}

		err := presenter.SaveUserMessage(msg)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `SaveUserMessage`")
		assert.Contains(t, output, "text=\"Save this message\"")
	})
}

func TestLogmdChatPresenter_Pause(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Pause()

		require.NoError(t, err)
		assert.Equal(t, 1, wrapped.PauseCalls)
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.PauseErr = errors.New("pause error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Pause()

		assert.Error(t, err)
		assert.Equal(t, "pause error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Pause()

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `Pause`")
	})
}

func TestLogmdChatPresenter_Resume(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Resume()

		require.NoError(t, err)
		assert.Equal(t, 1, wrapped.ResumeCalls)
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.ResumeErr = errors.New("resume error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Resume()

		assert.Error(t, err)
		assert.Equal(t, "resume error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.Resume()

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `Resume`")
	})
}

func TestLogmdChatPresenter_PermissionResponse(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.PermissionResponse("yes")

		require.NoError(t, err)
		assert.Len(t, wrapped.PermissionResponseCalls, 1)
		assert.Equal(t, "yes", wrapped.PermissionResponseCalls[0])
	})

	t.Run("returns error from wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		wrapped.PermissionResponseErr = errors.New("permission error")
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.PermissionResponse("no")

		assert.Error(t, err)
		assert.Equal(t, "permission error", err.Error())
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.PermissionResponse("ask")

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `PermissionResponse`")
		assert.Contains(t, output, "response=\"ask\"")
	})
}

func TestLogmdChatPresenter_SetModel(t *testing.T) {
	t.Run("delegates to wrapped presenter", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.SetModel("openai/gpt-4")

		require.NoError(t, err)
	})

	t.Run("logs method call to markdown", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		err := presenter.SetModel("anthropic/claude-3")

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "## System")
		assert.Contains(t, output, "**Method:** `SetModel`")
		assert.Contains(t, output, "model=\"anthropic/claude-3\"")
	})
}

func TestLogmdChatPresenter_Concurrency(t *testing.T) {
	t.Run("mutex protects concurrent writes", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				msg := &ui.ChatMessageUI{
					Id:   string(rune('a' + i%26)),
					Role: ui.ChatRoleUser,
					Text: "Message",
				}
				_ = presenter.SendUserMessage(msg)
			}(i)
		}
		wg.Wait()

		output := buf.String()
		count := strings.Count(output, "## System")
		assert.Equal(t, 100, count)
	})
}

func TestLogmdChatPresenter_FullSession(t *testing.T) {
	t.Run("complete session lifecycle", func(t *testing.T) {
		wrapped := mock.NewMockChatPresenter()
		var buf bytes.Buffer
		mu := &sync.Mutex{}
		presenter := NewLogmdChatPresenter(wrapped, &buf, mu)

		view := mock.NewMockChatView()

		// Set view
		err := presenter.SetView(view)
		require.NoError(t, err)

		// Send user message
		msg1 := &ui.ChatMessageUI{
			Id:   "msg-1",
			Role: ui.ChatRoleUser,
			Text: "Hello, can you help me?",
		}
		err = presenter.SendUserMessage(msg1)
		require.NoError(t, err)

		// Save user message
		msg2 := &ui.ChatMessageUI{
			Id:   "msg-2",
			Role: ui.ChatRoleUser,
			Text: "Also, save this for later",
		}
		err = presenter.SaveUserMessage(msg2)
		require.NoError(t, err)

		// Pause
		err = presenter.Pause()
		require.NoError(t, err)

		// Resume
		err = presenter.Resume()
		require.NoError(t, err)

		// Permission response
		err = presenter.PermissionResponse("yes")
		require.NoError(t, err)

		// Set model
		err = presenter.SetModel("openai/gpt-4o")
		require.NoError(t, err)

		// Verify output
		output := buf.String()

		// Should have all method calls logged
		assert.Contains(t, output, "**Method:** `SetView`")
		assert.Contains(t, output, "**Method:** `SendUserMessage`")
		assert.Contains(t, output, "**Method:** `SaveUserMessage`")
		assert.Contains(t, output, "**Method:** `Pause`")
		assert.Contains(t, output, "**Method:** `Resume`")
		assert.Contains(t, output, "**Method:** `PermissionResponse`")
		assert.Contains(t, output, "**Method:** `SetModel`")

		// Should have parameters logged
		assert.Contains(t, output, "Hello, can you help me?")
		assert.Contains(t, output, "Also, save this for later")
		assert.Contains(t, output, "response=\"yes\"")
		assert.Contains(t, output, "model=\"openai/gpt-4o\"")
	})
}
