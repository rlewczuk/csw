package tui

import (
	"testing"
	"time"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationRedrawFromBackgroundThread tests that UI changes from background threads
// trigger a redraw without requiring user input or resize events.
func TestApplicationRedrawFromBackgroundThread(t *testing.T) {
	t.Run("BUG: UI change from background thread without redraw notification", func(t *testing.T) {
		// This test demonstrates the BUG where UI modifications from background threads
		// don't automatically trigger a redraw, requiring a resize or input event

		// Create screen buffer
		screen := tio.NewScreenBuffer(80, 24, 0)

		// Create a test widget that can track draw calls
		drawCount := 0
		testWidget := &testDrawCountWidget{
			TWidget: TWidget{
				Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
			},
			drawFunc: func() {
				drawCount++
			},
		}

		// Create application
		app := NewApplication(testWidget, screen)
		require.NotNil(t, app)

		// Initial draw count should be 0
		assert.Equal(t, 0, drawCount, "Initial draw count should be 0")

		// Simulate a background thread modifying the widget state
		// This is what happens when OnPermissionQuery is called
		done := make(chan bool)
		go func() {
			// Wait a bit to ensure main thread is "idle"
			time.Sleep(10 * time.Millisecond)

			// Modify widget state directly (THIS IS THE BUG)
			// The background thread modifies UI without triggering redraw
			testWidget.label = "Modified from background thread"

			// Signal that modification is done
			done <- true
		}()

		// Wait for background thread to modify the widget
		<-done

		// BUG: At this point, the widget has been modified but no redraw happened
		// The UI won't update until next input event or resize
		assert.Equal(t, 0, drawCount, "BUG: Draw is not called after background modification")

		// In real app, user would have to resize window to see the change
		// This simulates resize event
		app.Notify(gtv.InputEvent{
			Type: gtv.InputEventResize,
			X:    80,
			Y:    24,
		})

		// NOW the draw should have been called (due to resize event)
		assert.Greater(t, drawCount, 0, "Draw is called after resize event")
	})

	t.Run("FIX: UI change from background thread with redraw notification", func(t *testing.T) {
		// This test shows the FIX - using ExecuteOnUiThread or RequestRedraw

		// Create screen buffer
		screen := tio.NewScreenBuffer(80, 24, 0)

		// Create a test widget that can track draw calls
		drawCount := 0
		testWidget := &testDrawCountWidget{
			TWidget: TWidget{
				Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
			},
			drawFunc: func() {
				drawCount++
			},
		}

		// Create application
		app := NewApplication(testWidget, screen)
		require.NotNil(t, app)

		// Initial draw count should be 0
		assert.Equal(t, 0, drawCount, "Initial draw count should be 0")

		// Simulate a background thread modifying the widget
		done := make(chan bool)
		go func() {
			// Wait a bit to ensure main thread is "idle"
			time.Sleep(10 * time.Millisecond)

			// FIX: Use ExecuteOnUiThread to modify state and trigger redraw
			app.ExecuteOnUiThread(func() {
				testWidget.label = "Modified from background thread"
				testWidget.Draw(screen)
			})

			// Signal that modification is done
			done <- true
		}()

		// Wait for background thread to modify the widget
		<-done

		// After ExecuteOnUiThread, draw should have been called
		assert.Equal(t, 1, drawCount, "Draw should be called after ExecuteOnUiThread")
	})

	t.Run("permission menu from background thread should trigger automatic redraw", func(t *testing.T) {
		// This test simulates the REAL permission menu scenario
		// The bug: QueryPermission is called from background thread without ExecuteOnUiThread
		// and doesn't trigger a redraw

		// Create screen buffer
		screen := tio.NewScreenBuffer(80, 24, 0)

		// Create a simple widget to hold a menu
		widget := &testMenuHolderWidget{
			TWidget: TWidget{
				Position: gtv.TRect{X: 0, Y: 0, W: 80, H: 24},
			},
		}

		// Create application
		app := NewApplication(widget, screen)
		require.NotNil(t, app)

		// Verify initial state - no menu
		assert.Nil(t, widget.menu, "Menu should be nil initially")
		assert.False(t, widget.menuDrawn, "Menu should not be drawn initially")

		// Simulate background thread showing permission menu
		// This is what happens in chat_presenter.go:126 when OnPermissionQuery is called
		done := make(chan bool)
		go func() {
			time.Sleep(10 * time.Millisecond)

			// BUG: Show menu from background thread WITHOUT using ExecuteOnUiThread
			// This is exactly what the current code does - it calls view.QueryPermission()
			// directly from the background thread
			widget.ShowMenu()

			done <- true
		}()

		// Wait for background thread
		<-done

		// Menu should exist now
		assert.NotNil(t, widget.menu, "Menu should exist after ShowMenu")

		// BUG: Menu is NOT drawn yet because ShowMenu() didn't trigger a redraw
		assert.False(t, widget.menuDrawn, "BUG: Menu is not drawn after ShowMenu from background thread")

		// FIX: Use ExecuteOnUiThread which should trigger automatic redraw
		app.ExecuteOnUiThread(func() {
			// No manual Draw() call here - the fix should auto-redraw after ExecuteOnUiThread
		})

		// Check that redraw was requested
		select {
		case <-app.redrawCh:
			// Redraw was requested - now process it manually (simulating event loop)
			app.mu.Lock()
			widget.Draw(screen)
			app.mu.Unlock()
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Redraw was not requested after ExecuteOnUiThread")
		}

		// After processing the redraw request, menu should be drawn
		assert.True(t, widget.menuDrawn, "Menu should be auto-drawn after ExecuteOnUiThread triggers redraw")
	})
}

// testDrawCountWidget is a test widget that counts how many times Draw is called
type testDrawCountWidget struct {
	TWidget
	label    string
	drawFunc func()
}

func (w *testDrawCountWidget) Draw(screen gtv.IScreenOutput) {
	if w.drawFunc != nil {
		w.drawFunc()
	}
}

// testMenuHolderWidget simulates a widget that can show a menu (like TChatView with permission menu)
type testMenuHolderWidget struct {
	TWidget
	menu      *TMenuWidget
	menuDrawn bool
}

func (w *testMenuHolderWidget) ShowMenu() {
	// Create menu widget (simulating QueryPermission)
	w.menu = NewMenuWidget(
		&w.TWidget,
		WithRectangle(10, 10, 30, 10),
	)
	w.menu.SetTitle("Permission Menu")
	w.menu.AddItem("Allow", func(text string) {})
	w.menu.AddItem("Deny", func(text string) {})
}

func (w *testMenuHolderWidget) Draw(screen gtv.IScreenOutput) {
	// Draw menu if it exists
	if w.menu != nil {
		w.menu.Draw(screen)
		w.menuDrawn = true
	}
}
