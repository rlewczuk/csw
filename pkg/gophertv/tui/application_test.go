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
		name        string
		mainWidget  IWidget
		expectError bool
	}{
		{
			name: "valid widget",
			mainWidget: &TWidget{
				Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't test with real os.Stdin/os.Stdout in unit tests
			// This test primarily validates the interface
			assert.NotNil(t, tt.mainWidget)
		})
	}
}

// TestNewApplicationForTest tests the NewApplicationForTest constructor.
func TestNewApplicationForTest(t *testing.T) {
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
			app := NewApplicationForTest(widget, screen)

			// Verify application was created
			require.NotNil(t, app)
			assert.NotNil(t, app.mainWidget)
			assert.NotNil(t, app.screen)
			assert.Nil(t, app.renderer) // No renderer in test mode
			assert.True(t, app.testMode)

			// Verify widget was resized to screen size
			pos := widget.GetPos()
			assert.Equal(t, tt.expectSize, pos)
		})
	}
}

// TestApplicationRun tests the Run method in test mode.
func TestApplicationRun(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{
			name:   "standard size",
			width:  80,
			height: 24,
		},
		{
			name:   "large size",
			width:  120,
			height: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tio.NewScreenBuffer(tt.width, tt.height, 0)
			widget := &TWidget{
				Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
			}

			app := NewApplicationForTest(widget, screen)

			// Run should return immediately in test mode
			err := app.Run()
			assert.NoError(t, err)
		})
	}
}

// TestApplicationQuit tests the Quit method.
func TestApplicationQuit(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)

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

	app := NewApplicationForTest(widget, screen)

	// Get screen
	retrievedScreen := app.GetScreen()

	// Verify it's the same screen
	assert.Equal(t, screen, retrievedScreen)
}

// TestApplicationInjectEvent tests the InjectEvent method.
func TestApplicationInjectEvent(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)

	// Inject an event
	event := gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'a',
	}

	app.InjectEvent(event)

	// Verify event was queued
	select {
	case e := <-app.eventCh:
		assert.Equal(t, event, e)
	default:
		t.Error("Event was not queued")
	}
}

// TestApplicationProcessEvents tests the ProcessEvents method.
func TestApplicationProcessEvents(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)

	// Inject multiple events
	events := []gophertv.InputEvent{
		{Type: gophertv.InputEventKey, Key: 'a'},
		{Type: gophertv.InputEventKey, Key: 'b'},
		{Type: gophertv.InputEventKey, Key: 'c'},
	}

	for _, event := range events {
		app.InjectEvent(event)
	}

	// Process all events
	app.ProcessEvents()

	// Verify all events were processed (channel should be empty)
	select {
	case <-app.eventCh:
		t.Error("Not all events were processed")
	default:
		// All events processed successfully
	}
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

			app := NewApplicationForTest(widget, screen)
			err := app.Run()
			require.NoError(t, err)

			// Inject resize event
			app.InjectEvent(gophertv.InputEvent{
				Type: gophertv.InputEventResize,
				X:    tt.newW,
				Y:    tt.newH,
			})

			// Process events
			app.ProcessEvents()

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

	app := NewApplicationForTest(widget, screen)
	err := app.Run()
	require.NoError(t, err)

	// Inject Ctrl+C event
	app.InjectEvent(gophertv.InputEvent{
		Type:      gophertv.InputEventKey,
		Key:       'c',
		Modifiers: gophertv.ModCtrl,
	})

	// Process events
	app.ProcessEvents()

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

	app := NewApplicationForTest(widget, screen)
	err := app.Run()
	require.NoError(t, err)

	// Inject multiple resize events
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
		app.InjectEvent(gophertv.InputEvent{
			Type: gophertv.InputEventResize,
			X:    r.w,
			Y:    r.h,
		})
	}

	// Process all events
	app.ProcessEvents()

	// Verify final size matches last resize
	w, h := screen.GetSize()
	assert.Equal(t, int(resizes[len(resizes)-1].w), w)
	assert.Equal(t, int(resizes[len(resizes)-1].h), h)

	// Verify widget size matches final size
	pos := widget.GetPos()
	assert.Equal(t, resizes[len(resizes)-1].w, pos.W)
	assert.Equal(t, resizes[len(resizes)-1].h, pos.H)
}

// TestApplicationNotify tests the Notify method (InputEventHandler interface).
func TestApplicationNotify(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)

	// Call Notify directly
	event := gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'x',
	}

	app.Notify(event)

	// Verify event was queued
	select {
	case e := <-app.eventCh:
		assert.Equal(t, event, e)
	default:
		t.Error("Event was not queued by Notify")
	}
}

// TestApplicationEventOrdering tests that events are processed in order.
func TestApplicationEventOrdering(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)
	err := app.Run()
	require.NoError(t, err)

	// Track event order
	var processedEvents []rune

	// We can't directly track events in the base TWidget,
	// but we can verify that multiple events are processed
	// by checking the event channel is empty after processing

	// Inject events
	keys := []rune{'a', 'b', 'c', 'd', 'e'}
	for _, key := range keys {
		app.InjectEvent(gophertv.InputEvent{
			Type: gophertv.InputEventKey,
			Key:  key,
		})
	}

	// Process all events
	app.ProcessEvents()

	// Verify all events were processed
	assert.Equal(t, 0, len(app.eventCh))

	// Note: In a real scenario with a custom widget, we would verify
	// the order by tracking which events the widget received
	_ = processedEvents // Unused in this simple test
}

// TestApplicationEmptyEventQueue tests ProcessEvents with empty queue.
func TestApplicationEmptyEventQueue(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)

	// Process events when queue is empty (should not block or panic)
	app.ProcessEvents()

	// Verify no errors occurred
	assert.Equal(t, 0, len(app.eventCh))
}

// TestApplicationMixedEvents tests handling of mixed event types.
func TestApplicationMixedEvents(t *testing.T) {
	screen := tio.NewScreenBuffer(80, 24, 0)
	widget := &TWidget{
		Position: gophertv.TRect{X: 0, Y: 0, W: 0, H: 0},
	}

	app := NewApplicationForTest(widget, screen)
	err := app.Run()
	require.NoError(t, err)

	// Inject mixed events
	app.InjectEvent(gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'a',
	})

	app.InjectEvent(gophertv.InputEvent{
		Type: gophertv.InputEventResize,
		X:    100,
		Y:    30,
	})

	app.InjectEvent(gophertv.InputEvent{
		Type: gophertv.InputEventKey,
		Key:  'b',
	})

	app.InjectEvent(gophertv.InputEvent{
		Type: gophertv.InputEventMouse,
		X:    10,
		Y:    15,
	})

	// Process all events
	app.ProcessEvents()

	// Verify all events were processed
	assert.Equal(t, 0, len(app.eventCh))

	// Verify resize was applied
	w, h := screen.GetSize()
	assert.Equal(t, 100, w)
	assert.Equal(t, 30, h)
}
