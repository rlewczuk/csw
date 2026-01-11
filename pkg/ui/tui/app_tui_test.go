package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codesnort/codesnort-swe/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
)

func TestTuiAppView(t *testing.T) {
	t.Run("NewTuiAppView creates view with presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)
		assert.NotNil(t, view)
		assert.NotNil(t, view.Model())
		assert.Equal(t, presenter, view.presenter)
	})

	t.Run("ShowChat creates and returns chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		chatPresenter := mock.NewMockChatPresenter()
		chatView := view.ShowChat(chatPresenter)
		assert.NotNil(t, chatView)
		assert.NotNil(t, view.chatView)
	})

	t.Run("ShowChat reuses existing chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		chatPresenter1 := mock.NewMockChatPresenter()
		chatView1 := view.ShowChat(chatPresenter1)
		assert.NotNil(t, chatView1)

		chatPresenter2 := mock.NewMockChatPresenter()
		chatView2 := view.ShowChat(chatPresenter2)
		assert.NotNil(t, chatView2)

		// Should reuse the same view instance
		assert.Equal(t, chatView1, chatView2)
	})

	t.Run("ShowSettings does not panic", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		// Should not panic
		assert.NotPanics(t, func() {
			view.ShowSettings()
		})
	})

	t.Run("Model returns bubbletea model", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		model := view.Model()
		assert.NotNil(t, model)

		cmd := model.Init()
		assert.Nil(t, cmd) // Should be nil when no chat view is set
	})

	t.Run("Model handles WindowSizeMsg", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		// Create chat view
		chatPresenter := mock.NewMockChatPresenter()
		view.ShowChat(chatPresenter)

		model := view.Model()
		model.Update(tea.WindowSizeMsg{
			Width:  100,
			Height: 50,
		})

		assert.Equal(t, 100, view.width)
		assert.Equal(t, 50, view.height)
	})

	t.Run("View renders status bar", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		viewStr := view.View()
		assert.Contains(t, viewStr, "Ctrl+P: Menu")
		assert.Contains(t, viewStr, appName)
		assert.Contains(t, viewStr, appVersion)
	})

	t.Run("View renders chat view when available", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		chatPresenter := mock.NewMockChatPresenter()
		view.ShowChat(chatPresenter)

		viewStr := view.View()
		assert.NotEmpty(t, viewStr)
		// Status bar should still be present
		assert.Contains(t, viewStr, "Ctrl+P: Menu")
	})
}

func TestTuiAppViewMenu(t *testing.T) {
	t.Run("Ctrl+P shows menu", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		term := NewTerminalMock()
		term.Run(view)

		// Send window size to initialize
		term.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

		// Wait for initial render
		time.Sleep(50 * time.Millisecond)

		// Press Ctrl+P to show menu
		term.SendKey("ctrl+p")

		// Wait for menu to appear
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second), "Menu should appear")
		assert.True(t, term.WaitForText("New Session", 1*time.Second), "Menu should show New Session option")
		assert.True(t, term.WaitForText("Exit", 1*time.Second), "Menu should show Exit option")

		// Cleanup
		term.SendKey("esc")
		term.Close()
	})

	t.Run("Menu New Session calls presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Press Ctrl+P to show menu
		term.SendKey("ctrl+p")

		// Wait for menu to appear
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second))

		// Select "New Session" (already selected by default)
		term.SendKey("enter")

		// Wait for action to be processed
		time.Sleep(50 * time.Millisecond)

		// Cleanup
		term.Close()

		// Verify presenter was called
		assert.Equal(t, 1, presenter.NewSessionCalls, "NewSession should be called once")
	})

	t.Run("Menu Exit calls presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Press Ctrl+P to show menu
		term.SendKey("ctrl+p")

		// Wait for menu to appear
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second))

		// Navigate to "Exit"
		term.SendKey("down")

		// Select "Exit"
		term.SendKey("enter")

		// Wait for action to be processed
		time.Sleep(50 * time.Millisecond)

		// Cleanup
		term.Close()

		// Verify presenter was called
		assert.Equal(t, 1, presenter.ExitCalls, "Exit should be called once")
	})

	t.Run("Menu can be dismissed with Esc", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Press Ctrl+P to show menu
		term.SendKey("ctrl+p")

		// Wait for menu to appear
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second))
		assert.True(t, view.showingMenu, "Menu should be showing")

		// Dismiss menu with Esc
		term.SendKey("esc")

		// Wait for menu to close
		time.Sleep(50 * time.Millisecond)

		// Cleanup
		term.Close()

		// Verify menu is closed
		assert.False(t, view.showingMenu, "Menu should not be showing after Esc")
	})

	t.Run("Menu blocks chat view input", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		chatPresenter := mock.NewMockChatPresenter()
		view.ShowChat(chatPresenter)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Press Ctrl+P to show menu
		term.SendKey("ctrl+p")

		// Wait for menu to appear
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second))

		// Try to send a message (should be blocked by menu)
		term.SendString("test message")

		// The message should not be sent to the chat presenter
		// because the menu is capturing all input

		// Dismiss menu
		term.SendKey("esc")

		// Cleanup
		term.Close()

		// Verify chat presenter was not called
		assert.Equal(t, 0, len(chatPresenter.SendUserMessageCalls), "Chat should not receive input while menu is open")
	})
}

func TestTuiAppViewIntegration(t *testing.T) {
	t.Run("Full workflow: show chat, open menu, close menu", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(appPresenter)
		assert.NoError(t, err)

		// Show chat view
		chatPresenter := mock.NewMockChatPresenter()
		chatView := view.ShowChat(chatPresenter)
		assert.NotNil(t, chatView)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Verify initial state shows status bar
		assert.True(t, term.WaitForText("Ctrl+P: Menu", 1*time.Second))

		// Open menu
		term.SendKey("ctrl+p")
		assert.True(t, term.WaitForText("Main Menu", 1*time.Second))
		assert.True(t, view.showingMenu)

		// Close menu
		term.SendKey("esc")
		time.Sleep(50 * time.Millisecond)
		assert.False(t, view.showingMenu)

		// Cleanup
		term.Close()
	})

	t.Run("Multiple menu open/close cycles", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		term := NewTerminalMock()
		term.Run(view)

		// Wait for initial render
		time.Sleep(10 * time.Millisecond)

		// Open and close menu multiple times
		for i := 0; i < 3; i++ {
			// Open menu
			term.SendKey("ctrl+p")
			assert.True(t, term.WaitForText("Main Menu", 1*time.Second))

			// Close menu
			term.SendKey("esc")
			time.Sleep(50 * time.Millisecond)
		}

		// Cleanup
		term.Close()

		// No assertions needed, just verify no panics
	})
}

func TestTuiAppViewStatusBar(t *testing.T) {
	t.Run("Status bar renders with correct width", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		// Set a specific width
		view.width = 100

		statusBar := view.renderStatusBar()
		assert.NotEmpty(t, statusBar)
		assert.Contains(t, statusBar, "Ctrl+P: Menu")
		assert.Contains(t, statusBar, appName)
	})

	t.Run("Status bar handles narrow width", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		// Set a very narrow width
		view.width = 40

		statusBar := view.renderStatusBar()
		assert.NotEmpty(t, statusBar)
		// Should still render without panic
	})

	t.Run("Status bar contains app name and version", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		view, err := NewTuiAppView(presenter)
		assert.NoError(t, err)

		statusBar := view.renderStatusBar()
		assert.Contains(t, statusBar, appName)
		assert.Contains(t, statusBar, appVersion)
	})
}
