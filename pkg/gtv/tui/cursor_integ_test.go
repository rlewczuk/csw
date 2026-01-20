package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"
	"github.com/stretchr/testify/assert"
)

func TestCursorHandling(t *testing.T) {
	// Setup screen buffer
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Setup layout with two input boxes
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	input1 := tui.NewInputBox(layout, "Input 1", gtv.TRect{X: 0, Y: 0, W: 20, H: 1}, gtv.CellAttributes{}, gtv.CellAttributes{})
	input2 := tui.NewInputBox(layout, "Input 2", gtv.TRect{X: 0, Y: 2, W: 20, H: 1}, gtv.CellAttributes{}, gtv.CellAttributes{})
	button := tui.NewButton(layout, "Button", gtv.TRect{X: 0, Y: 4, W: 10, H: 1}, gtv.CellAttributes{}, gtv.CellAttributes{}, gtv.CellAttributes{})

	app := tui.NewApplication(layout, screen)

	// Helper to trigger redraw
	redraw := func() {
		// Simulate a resize event to trigger handleEvent and Draw
		// We can't call app.Draw() directly as it's not exported or doesn't exist in interface
		// But we can manually call layout.Draw(screen) as app.Run/handleEvent would
		// However, the issue is that App.handleEvent calls Draw.
		// So we want to simulate the app loop behavior.

		// Ideally we would use app.Notify() but that's async or requires mocking reader.
		// For unit testing here, we can just check what happens when we call Draw on the widget
		// BUT the fix involves TApplication clearing the cursor.
		// So checking layout.Draw() is not enough if the fix is in TApplication.

		// If I assume TApplication handles the event, I should use app.Notify with a dummy event
		// or rely on the fact that I'm testing TApplication behavior.

		// Let's use app.Notify with a repaint event (Resize with same size)
		app.Notify(gtv.InputEvent{Type: gtv.InputEventResize, X: 80, Y: 24})
	}

	t.Run("Cursor should be hidden initially when no widget is focused", func(t *testing.T) {
		// Ensure nothing is focused
		app.ExecuteOnUiThread(func() {
			input1.Blur()
			input2.Blur()
		})

		redraw()

		// Check cursor style
		style := screen.GetCursorStyle()
		assert.Equal(t, gtv.CursorStyleHidden, style, "Cursor should be hidden when no widget is focused")
	})

	t.Run("Cursor should be visible and positioned when widget is focused", func(t *testing.T) {
		// Focus input 1
		app.ExecuteOnUiThread(func() {
			input1.Focus()
		})

		redraw()

		// Check cursor style and position
		style := screen.GetCursorStyle()
		assert.True(t, style != gtv.CursorStyleHidden, "Cursor should be visible")
		assert.True(t, style&gtv.CursorStyleBar != 0, "Cursor should be Bar style")

		x, y := screen.GetCursorPosition()
		// "Input 1" len is 7. Input box at 0,0. Cursor at end -> 7,0
		assert.Equal(t, 7, x)
		assert.Equal(t, 0, y)
	})

	t.Run("Cursor should move when focus changes", func(t *testing.T) {
		// Focus input 2, blur input 1
		app.ExecuteOnUiThread(func() {
			input1.Blur()
			input2.Focus()
		})

		redraw()

		// Check cursor style and position
		style := screen.GetCursorStyle()
		assert.True(t, style != gtv.CursorStyleHidden, "Cursor should be visible")

		x, y := screen.GetCursorPosition()
		// "Input 2" len is 7. Input box at 0,2. Cursor at end -> 7,2
		assert.Equal(t, 7, x)
		assert.Equal(t, 2, y)
	})

	t.Run("Cursor should be hidden when focus moves to non-input widget (Ghosting fix)", func(t *testing.T) {
		// Blur input 2, focus button
		app.ExecuteOnUiThread(func() {
			input2.Blur()
			button.Focus()
		})

		redraw()

		// Check cursor style
		style := screen.GetCursorStyle()
		assert.Equal(t, gtv.CursorStyleHidden, style, "Cursor should be hidden when button is focused")
	})

	t.Run("Cursor should be hidden when focus is lost", func(t *testing.T) {
		// Blur button (and ensure inputs are blurred)
		app.ExecuteOnUiThread(func() {
			input1.Blur()
			input2.Blur()
			button.Blur()
		})

		redraw()

		// Check cursor style
		style := screen.GetCursorStyle()
		assert.Equal(t, gtv.CursorStyleHidden, style, "Cursor should be hidden when focus is lost")
	})
}
