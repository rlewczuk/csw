package tui_test

import (
	"testing"

	"github.com/codesnort/codesnort-swe/pkg/gtv"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tio"
	"github.com/codesnort/codesnort-swe/pkg/gtv/tui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFrame_BasicRendering tests that the frame correctly renders with different border styles
func TestFrame_BasicRendering(t *testing.T) {
	tests := []struct {
		name        string
		borderStyle tui.BorderStyle
		expectedTL  rune // top-left corner
		expectedTR  rune // top-right corner
		expectedBL  rune // bottom-left corner
		expectedBR  rune // bottom-right corner
		expectedH   rune // horizontal
		expectedV   rune // vertical
	}{
		{
			name:        "single border",
			borderStyle: tui.BorderStyleSingle,
			expectedTL:  '┌',
			expectedTR:  '┐',
			expectedBL:  '└',
			expectedBR:  '┘',
			expectedH:   '─',
			expectedV:   '│',
		},
		{
			name:        "double border",
			borderStyle: tui.BorderStyleDouble,
			expectedTL:  '╔',
			expectedTR:  '╗',
			expectedBL:  '╚',
			expectedBR:  '╝',
			expectedH:   '═',
			expectedV:   '║',
		},
		{
			name:        "rounded border",
			borderStyle: tui.BorderStyleRounded,
			expectedTL:  '╭',
			expectedTR:  '╮',
			expectedBL:  '╰',
			expectedBR:  '╯',
			expectedH:   '─',
			expectedV:   '│',
		},
		{
			name:        "bold border",
			borderStyle: tui.BorderStyleBold,
			expectedTL:  '┏',
			expectedTR:  '┓',
			expectedBL:  '┗',
			expectedBR:  '┛',
			expectedH:   '━',
			expectedV:   '┃',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a screen buffer for testing (20x10 characters)
			screen := tio.NewScreenBuffer(20, 10, 0)

			// Create frame with specified border style
			frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tt.borderStyle,
				gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

			// Draw the frame
			frame.Draw(screen)

			// Get screen content
			width, height, content := screen.GetContent()
			assert.Equal(t, 20, width)
			assert.Equal(t, 10, height)

			// Check corners
			assert.Equal(t, tt.expectedTL, content[0].Rune, "top-left corner")
			assert.Equal(t, tt.expectedTR, content[19].Rune, "top-right corner")
			assert.Equal(t, tt.expectedBL, content[9*width].Rune, "bottom-left corner")
			assert.Equal(t, tt.expectedBR, content[9*width+19].Rune, "bottom-right corner")

			// Check top border
			for x := 1; x < 19; x++ {
				assert.Equal(t, tt.expectedH, content[x].Rune, "top border at x=%d", x)
			}

			// Check bottom border
			for x := 1; x < 19; x++ {
				assert.Equal(t, tt.expectedH, content[9*width+x].Rune, "bottom border at x=%d", x)
			}

			// Check left border
			for y := 1; y < 9; y++ {
				assert.Equal(t, tt.expectedV, content[y*width].Rune, "left border at y=%d", y)
			}

			// Check right border
			for y := 1; y < 9; y++ {
				assert.Equal(t, tt.expectedV, content[y*width+19].Rune, "right border at y=%d", y)
			}
		})
	}
}

// TestFrame_BorderlessStyle tests that borderless frame doesn't draw a border
func TestFrame_BorderlessStyle(t *testing.T) {
	// Create a screen buffer for testing (20x10 characters)
	screen := tio.NewScreenBuffer(20, 10, 0)

	// Create borderless frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleNone,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add a child label to verify it fills the entire frame
	label := tui.NewLabel(frame, "Test", gtv.TRect{X: 0, Y: 0, W: 4, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Draw the frame
	frame.Draw(screen)

	// Get screen content
	_, _, content := screen.GetContent()

	// Check that there are no border characters at corners
	assert.NotEqual(t, '┌', content[0].Rune, "should not have top-left corner")
	assert.NotEqual(t, '┐', content[19].Rune, "should not have top-right corner")

	// Verify label is drawn
	assert.Equal(t, 'T', content[0].Rune)
	assert.Equal(t, 'e', content[1].Rune)
	assert.Equal(t, 's', content[2].Rune)
	assert.Equal(t, 't', content[3].Rune)

	// Verify label got full frame size
	childPos := label.GetPos()
	assert.Equal(t, uint16(0), childPos.X)
	assert.Equal(t, uint16(0), childPos.Y)
	assert.Equal(t, uint16(20), childPos.W)
	assert.Equal(t, uint16(10), childPos.H)
}

// TestFrame_WithChild tests that frame correctly manages its child widget
func TestFrame_WithChild(t *testing.T) {
	// Create a screen buffer for testing (20x10 characters)
	screen := tio.NewScreenBuffer(20, 10, 0)

	// Create frame with single border
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add a child label
	label := tui.NewLabel(frame, "Hello", gtv.TRect{X: 0, Y: 0, W: 5, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Verify child position is adjusted for border (1 cell padding)
	childPos := label.GetPos()
	assert.Equal(t, uint16(1), childPos.X, "child X should be 1 (inside border)")
	assert.Equal(t, uint16(1), childPos.Y, "child Y should be 1 (inside border)")
	assert.Equal(t, uint16(18), childPos.W, "child width should be frame width - 2")
	assert.Equal(t, uint16(8), childPos.H, "child height should be frame height - 2")

	// Draw the frame
	frame.Draw(screen)

	// Get screen content
	width, _, content := screen.GetContent()

	// Verify border is drawn
	assert.Equal(t, '┌', content[0].Rune, "top-left corner")

	// Verify child is drawn inside border (at position 1,1 in absolute coordinates)
	assert.Equal(t, 'H', content[1*width+1].Rune)
	assert.Equal(t, 'e', content[1*width+2].Rune)
	assert.Equal(t, 'l', content[1*width+3].Rune)
	assert.Equal(t, 'l', content[1*width+4].Rune)
	assert.Equal(t, 'o', content[1*width+5].Rune)
}

// TestFrame_Title tests that frame correctly displays title with different positions
func TestFrame_Title(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		position      tui.TitlePosition
		expectedStart int // Expected starting position of title (relative to frame X)
	}{
		{
			name:          "left aligned",
			title:         "Test",
			position:      tui.TitlePositionLeft,
			expectedStart: 3, // 1 (corner) + 2 (gap)
		},
		{
			name:          "center aligned",
			title:         "Test",
			position:      tui.TitlePositionCenter,
			expectedStart: 8, // centered in 20-char width
		},
		{
			name:          "right aligned",
			title:         "Test",
			position:      tui.TitlePositionRight,
			expectedStart: 14, // 20 - 1 (corner) - 2 (gap) - 4 (title length) + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a screen buffer for testing (20x10 characters)
			screen := tio.NewScreenBuffer(20, 10, 0)

			// Create frame with title
			frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
				gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))
			frame.SetTitle(tt.title)
			frame.SetTitlePosition(tt.position)

			// Draw the frame
			frame.Draw(screen)

			// Get screen content
			_, _, content := screen.GetContent()

			// Verify title is rendered at expected position
			titleRunes := []rune(tt.title)
			for i, ch := range titleRunes {
				pos := tt.expectedStart + i
				assert.Equal(t, ch, content[pos].Rune, "title char %d at position %d", i, pos)
			}

			// Verify corners are still present
			assert.Equal(t, '┌', content[0].Rune, "top-left corner")
			assert.Equal(t, '┐', content[19].Rune, "top-right corner")
		})
	}
}

// TestFrame_SetTitle tests dynamic title updates
func TestFrame_SetTitle(t *testing.T) {
	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Set title
	frame.SetTitle("Original")
	assert.Equal(t, "Original", frame.GetTitle())

	// Change title
	frame.SetTitle("Updated")
	assert.Equal(t, "Updated", frame.GetTitle())

	// Create screen and verify new title is rendered
	screen := tio.NewScreenBuffer(20, 10, 0)
	frame.Draw(screen)

	_, _, content := screen.GetContent()
	assert.Equal(t, 'U', content[3].Rune)
	assert.Equal(t, 'p', content[4].Rune)
	assert.Equal(t, 'd', content[5].Rune)
}

// TestFrame_Icons tests frame icon functionality
func TestFrame_Icons(t *testing.T) {
	// Create a screen buffer for testing (30x10 characters)
	screen := tio.NewScreenBuffer(30, 10, 0)

	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 30, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add left icon
	leftIcon := &tui.FrameIcon{
		Rune:     '★',
		Color:    gtv.TextColor(0xFF0000),
		Position: tui.IconPositionLeft,
	}
	leftIconID := frame.AddIcon(leftIcon)
	assert.Greater(t, leftIconID, 0)
	assert.Equal(t, leftIconID, leftIcon.ID)

	// Add right icon
	rightIcon := &tui.FrameIcon{
		Rune:     '✓',
		Color:    gtv.TextColor(0x00FF00),
		Position: tui.IconPositionRight,
	}
	rightIconID := frame.AddIcon(rightIcon)
	assert.Greater(t, rightIconID, 0)
	assert.NotEqual(t, leftIconID, rightIconID)

	// Draw the frame
	frame.Draw(screen)

	// Get screen content
	_, _, content := screen.GetContent()

	// Verify left icon is rendered at position 3 (1 corner + 2 gap)
	assert.Equal(t, '★', content[3].Rune, "left icon")
	assert.Equal(t, gtv.TextColor(0xFF0000), content[3].Attrs.TextColor)

	// Verify right icon is rendered at appropriate position
	// Right icons start at: width - 1 (corner) - 2 (gap) - 1 (icon)
	rightIconPos := 30 - 1 - 2 - 1
	assert.Equal(t, '✓', content[rightIconPos].Rune, "right icon")
	assert.Equal(t, gtv.TextColor(0x00FF00), content[rightIconPos].Attrs.TextColor)
}

// TestFrame_IconVisibility tests showing and hiding icons
func TestFrame_IconVisibility(t *testing.T) {
	// Create a screen buffer for testing (20x10 characters)
	screen := tio.NewScreenBuffer(20, 10, 0)

	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add icon
	icon := &tui.FrameIcon{
		Rune:     '★',
		Color:    gtv.TextColor(0xFF0000),
		Position: tui.IconPositionLeft,
	}
	iconID := frame.AddIcon(icon)

	// Initially icon should be visible
	assert.False(t, icon.Hidden)

	// Draw and verify icon is present
	frame.Draw(screen)
	_, _, content := screen.GetContent()
	assert.Equal(t, '★', content[3].Rune)

	// Hide icon
	frame.HideIcon(iconID)
	assert.True(t, icon.Hidden)

	// Clear screen and redraw
	screen = tio.NewScreenBuffer(20, 10, 0)
	frame.Draw(screen)
	_, _, content = screen.GetContent()

	// Verify icon is not rendered (should be border character instead)
	assert.Equal(t, '─', content[3].Rune, "should be border after hiding icon")

	// Show icon again
	frame.ShowIcon(iconID)
	assert.False(t, icon.Hidden)

	// Clear screen and redraw
	screen = tio.NewScreenBuffer(20, 10, 0)
	frame.Draw(screen)
	_, _, content = screen.GetContent()
	assert.Equal(t, '★', content[3].Rune, "icon should reappear")
}

// TestFrame_IconHandler tests that icon handlers are called on click
func TestFrame_IconHandler(t *testing.T) {
	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Track handler calls
	handlerCalled := false

	// Add icon with handler
	icon := &tui.FrameIcon{
		Rune:     '★',
		Color:    gtv.TextColor(0xFF0000),
		Position: tui.IconPositionLeft,
		Handler: func() {
			handlerCalled = true
		},
	}
	frame.AddIcon(icon)

	// Create mouse click event at icon position (3, 0)
	clickEvent := &tui.TEvent{
		Type: tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{
			Type:      gtv.InputEventMouse,
			X:         3,
			Y:         0,
			Modifiers: gtv.ModClick,
		},
	}

	// Handle the event
	frame.HandleEvent(clickEvent)

	// Verify handler was called
	assert.True(t, handlerCalled, "icon handler should be called on click")
}

// TestFrame_FocusSupport tests that frame supports focus with different attributes
func TestFrame_FocusSupport(t *testing.T) {
	// Create a screen buffer for testing (20x10 characters)
	screen := tio.NewScreenBuffer(20, 10, 0)

	// Create frame with normal and focused attributes
	normalAttrs := gtv.AttrsWithColor(0, 0x888888, 0x000000)
	focusedAttrs := gtv.AttrsWithColor(0, 0x00AAFF, 0x000000)
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		normalAttrs, focusedAttrs)

	// Initially not focused
	assert.False(t, frame.IsFocused())

	// Draw and verify normal attributes
	frame.Draw(screen)
	_, _, content := screen.GetContent()
	assert.Equal(t, gtv.TextColor(0x888888), content[0].Attrs.TextColor, "normal color")

	// Send focus event
	focusEvent := &tui.TEvent{
		Type:       tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{Type: gtv.InputEventFocus},
	}
	frame.HandleEvent(focusEvent)

	// Verify frame is focused
	assert.True(t, frame.IsFocused())

	// Clear and redraw
	screen = tio.NewScreenBuffer(20, 10, 0)
	frame.Draw(screen)
	_, _, content = screen.GetContent()
	assert.Equal(t, gtv.TextColor(0x00AAFF), content[0].Attrs.TextColor, "focused color")

	// Send blur event
	blurEvent := &tui.TEvent{
		Type:       tui.TEventTypeInput,
		InputEvent: &gtv.InputEvent{Type: gtv.InputEventBlur},
	}
	frame.HandleEvent(blurEvent)

	// Verify frame is not focused
	assert.False(t, frame.IsFocused())
}

// TestFrame_Resize tests that frame correctly handles resize events
func TestFrame_Resize(t *testing.T) {
	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add child
	label := tui.NewLabel(frame, "Test", gtv.TRect{X: 0, Y: 0, W: 4, H: 1},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Verify initial child size
	childPos := label.GetPos()
	assert.Equal(t, uint16(18), childPos.W, "initial child width")
	assert.Equal(t, uint16(8), childPos.H, "initial child height")

	// Send resize event
	resizeEvent := &tui.TEvent{
		Type: tui.TEventTypeResize,
		Rect: gtv.TRect{X: 0, Y: 0, W: 30, H: 15},
	}
	frame.HandleEvent(resizeEvent)

	// Verify frame size changed
	framePos := frame.GetPos()
	assert.Equal(t, uint16(30), framePos.W)
	assert.Equal(t, uint16(15), framePos.H)

	// Verify child size changed accordingly
	childPos = label.GetPos()
	assert.Equal(t, uint16(28), childPos.W, "resized child width (30 - 2)")
	assert.Equal(t, uint16(13), childPos.H, "resized child height (15 - 2)")
}

// TestFrame_GetIcon tests retrieving icons by ID
func TestFrame_GetIcon(t *testing.T) {
	// Create frame
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 20, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Add icon
	icon := &tui.FrameIcon{
		Rune:     '★',
		Color:    gtv.TextColor(0xFF0000),
		Position: tui.IconPositionLeft,
	}
	iconID := frame.AddIcon(icon)

	// Retrieve icon by ID
	retrieved := frame.GetIcon(iconID)
	require.NotNil(t, retrieved)
	assert.Equal(t, iconID, retrieved.ID)
	assert.Equal(t, '★', retrieved.Rune)
	assert.Equal(t, gtv.TextColor(0xFF0000), retrieved.Color)

	// Try to retrieve non-existent icon
	nonExistent := frame.GetIcon(999)
	assert.Nil(t, nonExistent)
}

// TestFrame_TitleWithIcons tests rendering title alongside icons
func TestFrame_TitleWithIcons(t *testing.T) {
	// Create a screen buffer for testing (40x10 characters)
	screen := tio.NewScreenBuffer(40, 10, 0)

	// Create frame with title
	frame := tui.NewFrame(nil, gtv.TRect{X: 0, Y: 0, W: 40, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))
	frame.SetTitle("Test Title")
	frame.SetTitlePosition(tui.TitlePositionCenter)

	// Add left icon
	leftIcon := &tui.FrameIcon{
		Rune:     '★',
		Color:    gtv.TextColor(0xFF0000),
		Position: tui.IconPositionLeft,
	}
	frame.AddIcon(leftIcon)

	// Add right icon
	rightIcon := &tui.FrameIcon{
		Rune:     '✓',
		Color:    gtv.TextColor(0x00FF00),
		Position: tui.IconPositionRight,
	}
	frame.AddIcon(rightIcon)

	// Draw the frame
	frame.Draw(screen)

	// Get screen content
	_, _, content := screen.GetContent()

	// Verify left icon
	assert.Equal(t, '★', content[3].Rune, "left icon")

	// Verify right icon (at position 40 - 1 - 2 - 1 = 36)
	assert.Equal(t, '✓', content[36].Rune, "right icon")

	// Verify title is rendered somewhere in the middle
	titleFound := false
	for i := 10; i < 30; i++ {
		if content[i].Rune == 'T' && content[i+1].Rune == 'e' &&
			content[i+2].Rune == 's' && content[i+3].Rune == 't' {
			titleFound = true
			break
		}
	}
	assert.True(t, titleFound, "title should be rendered in the center")

	// Verify corners are present
	assert.Equal(t, '┌', content[0].Rune)
	assert.Equal(t, '┐', content[39].Rune)
}

// TestFrame_Application tests frame integration with TApplication
func TestFrame_Application(t *testing.T) {
	// Create a screen buffer for testing (40x20 characters)
	screen := tio.NewScreenBuffer(40, 20, 0)

	// Create main layout
	layout := tui.NewAbsoluteLayout(nil, gtv.TRect{X: 0, Y: 0, W: 40, H: 20}, nil)

	// Create frame with title and icons
	frame := tui.NewFrame(layout, gtv.TRect{X: 5, Y: 5, W: 30, H: 10}, tui.BorderStyleSingle,
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))
	frame.SetTitle("Test Frame")

	// Add child to frame
	_ = tui.NewLabel(frame, "Hello from frame!", gtv.TRect{X: 0, Y: 0, W: 0, H: 0},
		gtv.AttrsWithColor(0, 0xFFFFFF, 0x000000))

	// Create application
	app := tui.NewApplication(layout, screen)
	require.NotNil(t, app)

	// Draw
	layout.Draw(screen)

	// Get screen content
	width, _, content := screen.GetContent()

	// Verify frame is drawn at correct position (5, 5)
	topLeftIdx := 5*width + 5
	assert.Equal(t, '┌', content[topLeftIdx].Rune, "frame top-left corner")

	// Verify title is drawn
	titleIdx := 5*width + 5 + 3 // corner + gap
	assert.Equal(t, 'T', content[titleIdx].Rune)
	assert.Equal(t, 'e', content[titleIdx+1].Rune)
	assert.Equal(t, 's', content[titleIdx+2].Rune)
	assert.Equal(t, 't', content[titleIdx+3].Rune)

	// Verify child label is drawn inside frame (at 6, 6 in absolute coords)
	labelIdx := 6*width + 6
	assert.Equal(t, 'H', content[labelIdx].Rune)
	assert.Equal(t, 'e', content[labelIdx+1].Rune)
	assert.Equal(t, 'l', content[labelIdx+2].Rune)
	assert.Equal(t, 'l', content[labelIdx+3].Rune)
	assert.Equal(t, 'o', content[labelIdx+4].Rune)
}
