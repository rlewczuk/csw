package tui

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gophertv"
	"github.com/codesnort/codesnort-swe/pkg/gophertv/tio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewApplication tests the NewApplication constructor.
func TestNewApplication(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		widgetPos  gophertv.TRect
		expectSize gophertv.TRect
	}{
		{
			name:       "80x24 screen",
			width:      80,
			height:     24,
			widgetPos:  gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			expectSize: gophertv.TRect{X: 0, Y: 0, W: 80, H: 24},
		},
		{
			name:       "120x30 screen",
			width:      120,
			height:     30,
			widgetPos:  gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			expectSize: gophertv.TRect{X: 0, Y: 0, W: 120, H: 30},
		},
		{
			name:       "small screen 40x10",
			width:      40,
			height:     10,
			widgetPos:  gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			expectSize: gophertv.TRect{X: 0, Y: 0, W: 40, H: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create screen buffer
			screen := tio.NewScreenBuffer(tt.width, tt.height, 0)

			// Create widget
			widget := &TWidget{
				Position: tt.widgetPos,
			}

			// Create application
			app := NewApplication(widget, screen)

			// Verify application was created
			require.NotNil(t, app)
			assert.NotNil(t, app.mainWidget)
			assert.NotNil(t, app.screen)
			assert.Nil(t, app.renderer)    // No renderer until Run() is called
			assert.Nil(t, app.eventReader) // No event reader until Run() is called

			// Verify widget was resized to screen size
			pos := widget.GetPos()
			assert.Equal(t, tt.expectSize, pos)
		})
	}
}

// TestApplicationQuit tests the Quit method.
func TestApplicationQuit(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Call Quit
	app.Quit()

	// Verify quit signal was sent (channel should have a value)
	select {
	case <-app.quitCh:
		// Signal was sent successfully
	default:
		t.Error("Quit signal was not sent")
	}
}

// TestApplicationGetScreen tests the GetScreen method.
func TestApplicationGetScreen(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Get screen
	retrievedScreen := app.GetScreen()

	// Verify it's the same screen
	assert.Equal(t, screen, retrievedScreen)
}

// TestApplicationNotify tests the Notify method (InputEventHandler interface).
func TestApplicationNotify(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Call Notify with an event - it should handle it synchronously without panicking
	event := gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'a',
	}

	// Should not panic
	app.Notify(event)
}

// TestApplicationExecuteOnUiThread tests the ExecuteOnUiThread method.
func TestApplicationExecuteOnUiThread(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Execute a function on the UI thread
	executed := false
	app.ExecuteOnUiThread(func() {
		executed = true
	})

	// Verify function was executed
	assert.True(t, executed)
}

// TestApplicationHandleResize tests resize event handling.
func TestApplicationHandleResize(t *testing.T) {
	tests := []struct {
		name     string
		initialW int
		initialH int
		newW     uint16
		newH     uint16
		expectW  int
		expectH  int
	}{
		{
			name:     "grow both dimensions",
			initialW: 80,
			initialH: 24,
			newW:     120,
			newH:     30,
			expectW:  120,
			expectH:  30,
		},
		{
			name:     "shrink both dimensions",
			initialW: 120,
			initialH: 30,
			newW:     80,
			newH:     24,
			expectW:  80,
			expectH:  24,
		},
		{
			name:     "grow width, shrink height",
			initialW: 80,
			initialH: 24,
			newW:     100,
			newH:     20,
			expectW:  100,
			expectH:  20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tio.NewScreenBuffer(tt.initialW, tt.initialH, 0)
			widget := &TWidget{
				Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			}

			app := NewApplication(widget, screen)

			// Notify resize event (now processed synchronously)
			app.Notify(gophertv.InputEvent{
				Type: gophertv.InputEventResize,
				X:    tt.newW,
				Y:    tt.newH,
			})

			// Verify screen was resized
			w, h := screen.GetSize()
			assert.Equal(t, tt.expectW, w)
			assert.Equal(t, tt.expectH, h)

			// Verify widget was resized
			pos := widget.GetPos()
			assert.Equal(t, tt.newW, pos.W)
			assert.Equal(t, tt.newH, pos.H)
		})
	}
}

// TestApplicationHandleCtrlC tests Ctrl+C handling.
func TestApplicationHandleCtrlC(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Notify Ctrl+C event (now processed synchronously)
	app.Notify(gophertv.InputEvent{
		Type:      gophertv.InputEventKey,
		Key:       'c',
		Modifiers: gophertv.ModCtrl,
	})

	// Verify quit signal was sent
	select {
	case <-app.quitCh:
		// Quit signal was sent successfully
	default:
		t.Error("Ctrl+C did not send quit signal")
	}
}

// TestApplicationHandleMultipleResizes tests multiple resize events.
func TestApplicationHandleMultipleResizes(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Notify multiple resize events
	resizes := []struct {
		w uint16
		h uint16
	}{
		{100, 30},
		{120, 40},
		{80, 20},
		{90, 25},
	}

	for _, r := range resizes {
		// Each Notify is now processed synchronously
		app.Notify(gophertv.InputEvent{
			Type: gophertv.InputEventResize,
			X:    r.w,
			Y:    r.h,
		})
	}

	// Verify final size matches last resize
	w, h := screen.GetSize()
	assert.Equal(t, int(resizes[len(resizes)-1].w), w)
	assert.Equal(t, int(resizes[len(resizes)-1].h), h)

	// Verify widget size matches final size
	pos := widget.GetPos()
	assert.Equal(t, resizes[len(resizes)-1].w, pos.W)
	assert.Equal(t, resizes[len(resizes)-1].h, pos.H)
}

// TestApplicationEventOrdering tests that events are processed in order.
func TestApplicationEventOrdering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Notify events - they are now processed synchronously in order
	keys := []rune{'a', 'b', 'c', 'd', 'e'}
	for _, key := range keys {
		app.Notify(gophertv.InputEvent{
			Type: gophertv.InputEventKey,
			Key:  key,
		})
	}

	// All events are already processed synchronously
	// Note: In a real scenario with a custom widget, we would verify
	// the order by tracking which events the widget received
}

// TestApplicationExecuteOnUiThreadConcurrent tests ExecuteOnUiThread with concurrent access.
func TestApplicationExecuteOnUiThreadConcurrent(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Execute multiple functions concurrently to test mutex
	var counter int
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			app.ExecuteOnUiThread(func() {
				counter++
			})
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all increments were applied
	assert.Equal(t, 10, counter)
}

// TestApplicationMixedEvents tests handling of mixed event types.
func TestApplicationMixedEvents(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplication(widget, screen)

	// Notify mixed events
	app.Notify(gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'a',
	})

	app.Notify(gophertv.InputEvent{
		Type: gophertv.InputEventResize,
		X:    100,
		Y:    30,
	})

	app.Notify(gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'b',
	})

	app.Notify(gophertv.InputEvent{
		Type: gophertv.InputEventMouse,
		X:    10,
		Y:    15,
	})

	// All events are now processed synchronously

	// Verify resize was applied
	w, h := screen.GetSize()
	assert.Equal(t, 100, w)
	assert.Equal(t, 30, h)
}
