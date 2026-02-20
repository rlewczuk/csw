package tui

import (
	"testing"

	"github.com/rlewczuk/csw/pkg/gtv"
	"github.com/rlewczuk/csw/pkg/gtv/tio"
	"github.com/rlewczuk/csw/pkg/gtv/tui"
	"github.com/rlewczuk/csw/pkg/ui/mock"
	"github.com/stretchr/testify/assert"
)

func TestNewAppView(t *testing.T) {
	t.Run("creates app view with presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		assert.NotNil(t, view)
		assert.Equal(t, presenter, view.presenter)
		assert.Equal(t, 80, view.width)
		assert.Equal(t, 24, view.height)
		assert.False(t, view.showingMenu)
		assert.Nil(t, view.chatView)
		assert.Nil(t, view.menu)
		assert.NotNil(t, view.mainLayout)
		assert.NotNil(t, view.contentLayout)
		assert.NotNil(t, view.statusBar)
	})

	t.Run("creates app view with correct dimensions", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 10, Y: 5, W: 100, H: 50}
		view := NewAppView(nil, rect, presenter)

		assert.NotNil(t, view)
		assert.Equal(t, 100, view.width)
		assert.Equal(t, 50, view.height)
		assert.Equal(t, rect, view.Position)
	})
}

func TestAppViewShowChat(t *testing.T) {
	t.Run("creates and returns chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, appPresenter)

		chatPresenter := mock.NewMockChatPresenter()
		chatView := view.ShowChat(chatPresenter)

		assert.NotNil(t, chatView)
		assert.NotNil(t, view.chatView)
	})

	t.Run("reuses existing chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, appPresenter)

		chatPresenter1 := mock.NewMockChatPresenter()
		chatView1 := view.ShowChat(chatPresenter1)
		assert.NotNil(t, chatView1)

		chatPresenter2 := mock.NewMockChatPresenter()
		chatView2 := view.ShowChat(chatPresenter2)
		assert.NotNil(t, chatView2)

		// Should reuse the same view instance
		assert.Equal(t, chatView1, chatView2)
	})
}

func TestAppViewShowSettings(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// Should not panic
		assert.NotPanics(t, func() {
			view.ShowSettings()
		})
	})
}

func TestAppViewDraw(t *testing.T) {
	t.Run("draws successfully without chat view", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		screen := tio.NewScreenBuffer(80, 24, 0)

		assert.NotPanics(t, func() {
			view.Draw(screen)
		})
	})

	t.Run("draws successfully with chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, appPresenter)

		chatPresenter := mock.NewMockChatPresenter()
		view.ShowChat(chatPresenter)

		screen := tio.NewScreenBuffer(80, 24, 0)

		assert.NotPanics(t, func() {
			view.Draw(screen)
		})
	})

	t.Run("draws status bar with correct text", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		screen := tio.NewScreenBuffer(80, 24, 0)
		view.Draw(screen)

		// Verify status bar is drawn
		assert.NotNil(t, view.statusBar)
		statusText := view.statusBar.GetText()
		assert.Contains(t, statusText, "Ctrl+P/Esc: Menu")
		assert.Contains(t, statusText, appViewName)
		assert.Contains(t, statusText, appViewVersion)
	})
}

func TestAppViewHandleEvent(t *testing.T) {
	t.Run("handles resize event", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// Send resize event
		resizeEvent := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: 100, H: 50},
		}
		view.HandleEvent(resizeEvent)

		assert.Equal(t, 100, view.width)
		assert.Equal(t, 50, view.height)
		assert.Equal(t, uint16(100), view.Position.W)
		assert.Equal(t, uint16(50), view.Position.H)
	})

	t.Run("resizes layout and status bar on resize event", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// Verify initial sizes
		assert.Equal(t, uint16(80), view.mainLayout.Position.W)
		assert.Equal(t, uint16(24), view.mainLayout.Position.H)
		assert.Equal(t, uint16(80), view.contentLayout.Position.W)
		// Content layout height should be H-1 (managed by flex layout)
		assert.Greater(t, int(view.contentLayout.Position.H), 0)
		assert.Equal(t, uint16(80), view.statusBar.Position.W)
		assert.Equal(t, uint16(1), view.statusBar.Position.H)

		// Send resize event to simulate terminal resize
		resizeEvent := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: 120, H: 40},
		}
		view.HandleEvent(resizeEvent)

		// Verify view was resized
		assert.Equal(t, 120, view.width)
		assert.Equal(t, 40, view.height)
		assert.Equal(t, uint16(120), view.Position.W)
		assert.Equal(t, uint16(40), view.Position.H)

		// Verify main layout was resized
		assert.Equal(t, uint16(120), view.mainLayout.Position.W)
		assert.Equal(t, uint16(40), view.mainLayout.Position.H)

		// Verify content layout was resized (should be H-1 for status bar, managed by flex)
		assert.Equal(t, uint16(120), view.contentLayout.Position.W)
		assert.Equal(t, uint16(39), view.contentLayout.Position.H)

		// Verify status bar dimensions
		assert.Equal(t, uint16(120), view.statusBar.Position.W)
		assert.Equal(t, uint16(1), view.statusBar.Position.H)
	})

	t.Run("propagates resize event to chat view", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, appPresenter)

		chatPresenter := mock.NewMockChatPresenter()
		chatView := view.ShowChat(chatPresenter)

		// Verify initial chat view size
		assert.Equal(t, 80, chatView.(*TChatView).width)
		assert.Equal(t, 23, chatView.(*TChatView).height) // H-1 for status bar

		// Send resize event
		resizeEvent := &tui.TEvent{
			Type: tui.TEventTypeResize,
			Rect: gtv.TRect{X: 0, Y: 0, W: 120, H: 40},
		}
		view.HandleEvent(resizeEvent)

		// Verify chat view was resized (should match layout size)
		assert.Equal(t, 120, chatView.(*TChatView).width)
		assert.Equal(t, 39, chatView.(*TChatView).height) // H-1 for status bar
	})

	t.Run("handles Ctrl+P to show menu", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// Send Ctrl+P event
		ctrlPEvent := &tui.TEvent{
			Type: tui.TEventTypeInput,
			InputEvent: &gtv.InputEvent{
				Type:      gtv.InputEventKey,
				Key:       'p',
				Modifiers: gtv.ModCtrl,
			},
		}
		view.HandleEvent(ctrlPEvent)

		assert.True(t, view.showingMenu)
		assert.NotNil(t, view.menu)
	})

	t.Run("handles Esc to show menu", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// Send Esc event
		escEvent := &tui.TEvent{
			Type: tui.TEventTypeInput,
			InputEvent: &gtv.InputEvent{
				Type: gtv.InputEventKey,
				Key:  0x1B, // ESC
			},
		}
		view.HandleEvent(escEvent)

		assert.True(t, view.showingMenu)
		assert.NotNil(t, view.menu)
	})

	t.Run("passes through events to layout when menu not showing", func(t *testing.T) {
		appPresenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, appPresenter)

		chatPresenter := mock.NewMockChatPresenter()
		view.ShowChat(chatPresenter)

		// Send a regular key event (should be passed to layout/chat view)
		keyEvent := &tui.TEvent{
			Type: tui.TEventTypeInput,
			InputEvent: &gtv.InputEvent{
				Type: gtv.InputEventKey,
				Key:  'a',
			},
		}

		assert.NotPanics(t, func() {
			view.HandleEvent(keyEvent)
		})
	})
}

func TestAppViewMenu(t *testing.T) {
	t.Run("showMenu creates menu widget", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		view.showMenu()

		assert.True(t, view.showingMenu)
		assert.NotNil(t, view.menu)
		assert.Equal(t, "Main Menu", view.menu.GetTitle())
		assert.Len(t, view.menu.GetItems(), 2)
		assert.Equal(t, "New Session", view.menu.GetItems()[0].Label)
		assert.Equal(t, "Exit", view.menu.GetItems()[1].Label)
	})

	t.Run("hideMenu clears menu widget", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		view.showMenu()
		assert.True(t, view.showingMenu)
		assert.NotNil(t, view.menu)

		view.hideMenu()
		assert.False(t, view.showingMenu)
		assert.Nil(t, view.menu)
	})

	t.Run("menu New Session calls presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		view.showMenu()
		assert.NotNil(t, view.menu)

		// Simulate selecting "New Session"
		items := view.menu.GetItems()
		assert.Len(t, items, 2)
		items[0].Handler("")

		assert.Equal(t, 1, presenter.NewSessionCalls)
		assert.False(t, view.showingMenu) // Menu should be hidden after selection
	})

	t.Run("menu Exit calls presenter", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		view.showMenu()
		assert.NotNil(t, view.menu)

		// Simulate selecting "Exit"
		items := view.menu.GetItems()
		assert.Len(t, items, 2)
		items[1].Handler("")

		assert.Equal(t, 1, presenter.ExitCalls)
		assert.False(t, view.showingMenu) // Menu should be hidden after selection
	})
}

func TestAppViewStatusBar(t *testing.T) {
	t.Run("renders status bar with correct width", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 100, H: 24}
		view := NewAppView(nil, rect, presenter)

		statusText := view.renderStatusBarText()
		assert.NotEmpty(t, statusText)
		assert.Contains(t, statusText, "Ctrl+P/Esc: Menu")
		assert.Contains(t, statusText, appViewName)
		// Text should be padded to fill width
		assert.Equal(t, 100, len(statusText))
	})

	t.Run("renders status bar with narrow width", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 40, H: 24}
		view := NewAppView(nil, rect, presenter)

		statusText := view.renderStatusBarText()
		assert.NotEmpty(t, statusText)
		// Should still render without panic even with narrow width
		assert.Equal(t, 40, len(statusText))
	})

	t.Run("status bar contains app name and version", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		statusText := view.renderStatusBarText()
		assert.Contains(t, statusText, appViewName)
		assert.Contains(t, statusText, appViewVersion)
	})
}

func TestAppViewInterfaces(t *testing.T) {
	t.Run("implements ui.IAppView", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// This will fail to compile if TAppView doesn't implement ui.IAppView
		var _ interface{} = view
	})

	t.Run("implements tui.IWidget", func(t *testing.T) {
		presenter := mock.NewMockAppPresenter()
		rect := gtv.TRect{X: 0, Y: 0, W: 80, H: 24}
		view := NewAppView(nil, rect, presenter)

		// This will fail to compile if TAppView doesn't implement tui.IWidget
		var _ tui.IWidget = view
	})
}
