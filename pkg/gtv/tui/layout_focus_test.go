package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFocusableWidget is a simple test widget that captures events for testing.
// It implements IFocusable so it can receive focus.
type mockFocusableWidget struct {
	tui.TFocusable
	Events []*tui.TEvent
}

func newMockFocusableWidget(parent tui.IWidget, rect gtv.TRect) *mockFocusableWidget {
	w := &mockFocusableWidget{
		TFocusable: tui.TFocusable{
			TWidget: tui.TWidget{
				Position: rect,
				Parent:   parent,
				Flags:    tui.WidgetFlagNone,
			},
		},
		Events: make([]*tui.TEvent, 0),
	}
	if parent != nil {
		parent.AddChild(w)
	}
	return w
}

func (w *mockFocusableWidget) HandleEvent(event *tui.TEvent) {
	w.Events = append(w.Events, event)
}

func (w *mockFocusableWidget) Draw(screen gtv.IScreenOutput) {
	// Do nothing
}

// TestLayout_FocusManagement_DefaultTabOrder tests that tab navigation works with default tab order
// (order in which widgets were added to the layout).
func TestLayout_FocusManagement_DefaultTabOrder(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget (fills entire screen)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create input box widgets as children in specific order
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 5, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(10, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox3 := tui.NewInputBox(
		layout,
		tui.WithText("Input 3"),
		tui.WithRectangle(15, 15, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab to focus first widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab to focus second widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Tab to focus third widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Press Tab to wrap around to first widget
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Shift+Tab to go back to third widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Press Shift+Tab to go to second widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Shift+Tab to go to first widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Shift+Tab to wrap around to third widget
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)
}

// TestLayout_FocusManagement_CustomTabOrder tests that custom tab order works correctly.
func TestLayout_FocusManagement_CustomTabOrder(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget (fills entire screen)
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create input box widgets as children in specific order
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 5, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(10, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox3 := tui.NewInputBox(
		layout,
		tui.WithText("Input 3"),
		tui.WithRectangle(15, 15, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Set custom tab order (reverse of default)
	layout.SetTabOrder([]tui.IWidget{inputBox3, inputBox2, inputBox1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab to focus first widget in custom order (inputBox3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Press Tab to focus second widget in custom order (inputBox2)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Tab to focus third widget in custom order (inputBox1)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab to wrap around to first widget (inputBox3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)
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

	// Create input box widgets at different positions
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 5, 10, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(20, 10, 10, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox3 := tui.NewInputBox(
		layout,
		tui.WithText("Input 3"),
		tui.WithRectangle(35, 15, 10, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Click on inputBox2
	mockInput.MouseClick(25, 10, 0)
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Click on inputBox3
	mockInput.MouseClick(40, 15, 0)
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Click on inputBox1
	mockInput.MouseClick(10, 5, 0)
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Click outside any widget - focus should remain unchanged
	mockInput.MouseClick(70, 20, 0)
	assert.Equal(t, inputBox1, layout.ActiveChild)
}

// TestLayout_FocusManagement_FocusBlurEvents tests that focus and blur events are sent correctly.
func TestLayout_FocusManagement_FocusBlurEvents(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create mock focusable widgets that capture focus/blur events
	widget1 := newMockFocusableWidget(layout, gtv.TRect{X: 5, Y: 5, W: 10, H: 1})
	widget2 := newMockFocusableWidget(layout, gtv.TRect{X: 20, Y: 10, W: 10, H: 1})

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

	// Create input box widgets
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 5, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(10, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox3 := tui.NewInputBox(
		layout,
		tui.WithText("Input 3"),
		tui.WithRectangle(15, 15, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Focus inputBox2 using Tab
	mockInput.TypeKeysByName("Tab")
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Set custom tab order that doesn't include inputBox2
	layout.SetTabOrder([]tui.IWidget{inputBox1, inputBox3})

	// Press Tab - should focus inputBox1 (first in new order)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab - should focus inputBox3 (second in new order)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Press Tab - should wrap around to inputBox1
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)
}

// TestLayout_FocusManagement_SetTabOrderEmpty tests that SetTabOrder with empty slice
// returns to default tab order.
func TestLayout_FocusManagement_SetTabOrderEmpty(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create input box widgets
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 5, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(10, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	inputBox3 := tui.NewInputBox(
		layout,
		tui.WithText("Input 3"),
		tui.WithRectangle(15, 15, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Set custom tab order (reverse)
	layout.SetTabOrder([]tui.IWidget{inputBox3, inputBox2, inputBox1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Press Tab to focus first widget in custom order (inputBox3)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox3, layout.ActiveChild)

	// Reset to default tab order
	layout.SetTabOrder(nil)

	// Press Tab - should use default order (inputBox1)
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab - should focus inputBox2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)
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

// TestLayout_FocusManagement_OnlyNonFocusableWidgets tests that tab navigation skips
// non-focusable widgets (like labels) and only focuses focusable widgets.
func TestLayout_FocusManagement_OnlyNonFocusableWidgets(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create only non-focusable widgets (labels)
	tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	tui.NewLabel(layout, "Label 2", gtv.TRect{X: 10, Y: 10, W: 0, H: 0}, gtv.Attrs(0))
	tui.NewLabel(layout, "Label 3", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab - should not focus any widget since all are non-focusable
	mockInput.TypeKeysByName("Tab")
	assert.Nil(t, layout.ActiveChild)

	// Press Tab again - still no focus
	mockInput.TypeKeysByName("Tab")
	assert.Nil(t, layout.ActiveChild)

	// Press Shift+Tab - still no focus
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Nil(t, layout.ActiveChild)
}

// TestLayout_FocusManagement_MixedFocusableAndNonFocusable tests that tab navigation
// only cycles through focusable widgets and skips non-focusable ones.
func TestLayout_FocusManagement_MixedFocusableAndNonFocusable(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create mixed focusable and non-focusable widgets in alternating order
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 10, H: 1}, gtv.Attrs(0))
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 7, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 5, Y: 9, W: 10, H: 1}, gtv.Attrs(0))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(5, 11, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	label3 := tui.NewLabel(layout, "Label 3", gtv.TRect{X: 5, Y: 13, W: 10, H: 1}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab - should focus first focusable widget (inputBox1), skipping label1
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab - should focus second focusable widget (inputBox2), skipping label2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Tab - should wrap around to first focusable widget (inputBox1), skipping label3
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Shift+Tab - should go back to inputBox2, skipping label3
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Shift+Tab - should go back to inputBox1, skipping label2
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Shift+Tab - should wrap around to inputBox2, skipping label1
	mockInput.TypeKeysByName("Shift+Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Verify that labels were never focused
	assert.NotEqual(t, label1, layout.ActiveChild)
	assert.NotEqual(t, label2, layout.ActiveChild)
	assert.NotEqual(t, label3, layout.ActiveChild)
}

// TestLayout_FocusManagement_MouseClickOnNonFocusable tests that clicking on a
// non-focusable widget does not give it focus.
func TestLayout_FocusManagement_MouseClickOnNonFocusable(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create mixed focusable and non-focusable widgets
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 10, H: 1}, gtv.Attrs(0))
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(5, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 5, Y: 15, W: 10, H: 1}, gtv.Attrs(0))

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Click on label1 - should not give it focus
	mockInput.MouseClick(7, 5, 0)
	assert.Nil(t, layout.ActiveChild)
	assert.NotEqual(t, label1, layout.ActiveChild)

	// Click on inputBox1 - should give it focus
	mockInput.MouseClick(10, 10, 0)
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Click on label2 - should not change focus
	mockInput.MouseClick(7, 15, 0)
	assert.Equal(t, inputBox1, layout.ActiveChild)
	assert.NotEqual(t, label2, layout.ActiveChild)
}

// TestLayout_FocusManagement_CustomTabOrderWithNonFocusable tests that custom tab order
// filters out non-focusable widgets.
func TestLayout_FocusManagement_CustomTabOrderWithNonFocusable(t *testing.T) {
	// Create a screen buffer for testing (80x24 characters)
	screen := tio.NewScreenBuffer(80, 24, 0)

	// Create main layout widget
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 80, H: 24}, nil)

	// Create mixed focusable and non-focusable widgets
	label1 := tui.NewLabel(layout, "Label 1", gtv.TRect{X: 5, Y: 5, W: 0, H: 0}, gtv.Attrs(0))
	inputBox1 := tui.NewInputBox(
		layout,
		tui.WithText("Input 1"),
		tui.WithRectangle(10, 10, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))
	label2 := tui.NewLabel(layout, "Label 2", gtv.TRect{X: 15, Y: 15, W: 0, H: 0}, gtv.Attrs(0))
	inputBox2 := tui.NewInputBox(
		layout,
		tui.WithText("Input 2"),
		tui.WithRectangle(20, 20, 20, 1),
		tui.WithAttrs(gtv.Attrs(0)),
		tui.WithFocusedAttrs(gtv.Attrs(gtv.AttrReverse),
	))

	// Set custom tab order including non-focusable widgets
	// The layout should filter out non-focusable ones
	layout.SetTabOrder([]tui.IWidget{label1, inputBox2, label2, inputBox1})

	// Create application with the layout as main widget
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Create mock input event reader
	mockInput := tio.NewMockInputEventReader(app)

	// Verify no widget has focus initially
	assert.Nil(t, layout.ActiveChild)

	// Press Tab - should focus first focusable widget in custom order (inputBox2), skipping label1
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Press Tab - should focus second focusable widget in custom order (inputBox1), skipping label2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox1, layout.ActiveChild)

	// Press Tab - should wrap around to inputBox2
	mockInput.TypeKeysByName("Tab")
	assert.Equal(t, inputBox2, layout.ActiveChild)

	// Verify that labels were never focused
	assert.NotEqual(t, label1, layout.ActiveChild)
	assert.NotEqual(t, label2, layout.ActiveChild)
}
