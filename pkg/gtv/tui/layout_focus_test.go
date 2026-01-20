package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWidget is a simple test widget that captures events for testing.
type mockWidget struct {
	tui.TWidget
	Events []*tui.TEvent
}

func newMockWidget(parent tui.IWidget, rect gtv.TRect) *mockWidget {
	w := &mockWidget{
		TWidget: tui.TWidget{
			Position: rect,
			Parent:   parent,
			Flags:    tui.WidgetFlagNone,
		},
		Events: make([]*tui.TEvent, 0),
	}
	if parent != nil {
		parent.AddChild(w)
	}
	return w
}

func (w *mockWidget) HandleEvent(event *tui.TEvent) {
	w.Events = append(w.Events, event)
}

func (w *mockWidget) Draw(screen gtv.IScreenOutput) {
	// Do nothing
}

// TestLayout_FocusManagement_DefaultTabOrder tests that tab navigation works with default tab order
// (order in which widgets were added to the layout).
func TestLayout_FocusManagement_DefaultTabOrder(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget (fills entire screen)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create label widgets as children in specific order
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab to focus first widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Tab to focus second widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label2, layout.ActiveChild)

	// Press Tab to focus third widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label3, layout.ActiveChild)

	// Press Tab to wrap around to first widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Shift+Tab to go back to third widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, label3, layout.ActiveChild)

	// Press Shift+Tab to go to second widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, label2, layout.ActiveChild)

	// Press Shift+Tab to go to first widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Shift+Tab to wrap around to third widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, label3, layout.ActiveChild)
}

// TestLayout_FocusManagement_CustomTabOrder tests that custom tab order works correctly.
func TestLayout_FocusManagement_CustomTabOrder(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget (fills entire screen)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create label widgets as children in specific order
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Set custom tab order (reverse of default)
	layout.SetTabOrder([]tui.IWidget{label3, label2, label1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab to focus first widget in custom order (label3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label3, layout.ActiveChild)

	// Press Tab to focus second widget in custom order (label2)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label2, layout.ActiveChild)

	// Press Tab to focus third widget in custom order (label1)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Tab to wrap around to first widget (label3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label3, layout.ActiveChild)
}

// TestLayout_FocusManagement_TabOrderDisabled tests that tab navigation is disabled when TabOrderEnabled is false.
func TestLayout_FocusManagement_TabOrderDisabled(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget with tab order disabled
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil, false)

	// Create label widgets as children
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Set initial focus manually
	layout.ActiveChild = label1

	// Press Tab - should not change focus
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Shift+Tab - should not change focus
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, label1, layout.ActiveChild)
}

// TestLayout_FocusManagement_MouseClick tests that clicking on a widget gives it focus.
func TestLayout_FocusManagement_MouseClick(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create label widgets at different positions
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 10, H: 1}, gtv.Attrs(0))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 20, Y: 10, W: 10, H: 1}, gtv.Attrs(0))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 35, Y: 15, W: 10, H: 1}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Click on label2
	mockInput.MouseClick(25, 10, 0)
	assert.Equal(t, label2, layout.ActiveChild)

	// Click on label3
	mockInput.MouseClick(40, 15, 0)
	assert.Equal(t, label3, layout.ActiveChild)

	// Click on label1
	mockInput.MouseClick(10, 5, 0)
	assert.Equal(t, label1, layout.ActiveChild)

	// Click outside any widget - focus should remain unchanged
	mockInput.MouseClick(70, 20, 0)
	assert.Equal(t, label1, layout.ActiveChild)
}

// TestLayout_FocusManagement_FocusBlurEvents tests that focus and blur events are sent correctly.
func TestLayout_FocusManagement_FocusBlurEvents(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create mock widgets that capture focus/blur events
	widget1 := newMockWidget(layout, gtv.TRect{X: 5, Y: 5, W: 10, H: 1})
	widget2 := newMockWidget(layout, gtv.TRect{X: 20, Y: 10, W: 10, H: 1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no events initially
	assert.Len(t, widget1.Events, 0)
	assert.Len(t, widget2.Events, 0)

	// Press Tab to focus widget1
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, widget1, layout.ActiveChild)

	// Verify widget1 received focus event
	require.Len(t, widget1.Events, 1)
	assert.Equal(t, gtv.InputEventFocus, widget1.Events[0].InputEvent.Type)

	// Press Tab to focus widget2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, widget2, layout.ActiveChild)

	// Verify widget1 received blur event and widget2 received focus event
	require.Len(t, widget1.Events, 2)
	assert.Equal(t, gtv.InputEventBlur, widget1.Events[1].InputEvent.Type)
	require.Len(t, widget2.Events, 1)
	assert.Equal(t, gtv.InputEventFocus, widget2.Events[0].InputEvent.Type)
}

// TestLayout_FocusManagement_SetTabOrderResetsFocus tests that SetTabOrder resets focus if
// the current focused widget is not in the new tab order.
func TestLayout_FocusManagement_SetTabOrderResetsFocus(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create label widgets
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus label2 using Tab
	mockInput.TypeKeysByName("Tab")
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label2, layout.ActiveChild)

	// Set custom tab order that doesn't include label2
	layout.SetTabOrder([]tui.IWidget{label1, label3})

	// Press Tab - should focus label1 (first in new order)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Tab - should focus label3 (second in new order)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label3, layout.ActiveChild)

	// Press Tab - should wrap around to label1
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)
}

// TestLayout_FocusManagement_SetTabOrderEmpty tests that SetTabOrder with empty slice
// returns to default tab order.
func TestLayout_FocusManagement_SetTabOrderEmpty(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create label widgets
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Set custom tab order (reverse)
	layout.SetTabOrder([]tui.IWidget{label3, label2, label1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Press Tab to focus first widget in custom order (label3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label3, layout.ActiveChild)

	// Reset to default tab order
	layout.SetTabOrder(nil)

	// Press Tab - should use default order (label1)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label1, layout.ActiveChild)

	// Press Tab - should focus label2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, label2, layout.ActiveChild)
}

// TestLayout_FocusManagement_EmptyLayout tests that tab navigation does nothing
// when layout has no children.
func TestLayout_FocusManagement_EmptyLayout(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget with no children
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab - should not crash or change focus
	mockInput.TypeKeysByName("Tab")
	assert.Nil(t, layout.ActiveChild)

	// Press Shift+Tab - should not crash or change focus
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Nil(t, layout.ActiveChild)
}
